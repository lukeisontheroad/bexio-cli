// Package auth implements the bexio OpenID Connect flows: the browser-based
// Authorization Code Flow (with PKCE) for `bexio auth login`, and the Refresh
// Token grant used transparently before API calls.
//
// Endpoints per https://docs.bexio.com (Authentication section).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Endpoint URLs are variables so tests can point them at a fake server.
var (
	AuthURL  = "https://auth.bexio.com/realms/bexio/protocol/openid-connect/auth"
	TokenURL = "https://auth.bexio.com/realms/bexio/protocol/openid-connect/token"
)

// Built-in OAuth app (developer.bexio.com) so `bexio auth login --oauth`
// works without creating an app. Deliberately shipped in the binary: for
// native apps the client secret is not confidential (RFC 8252 §8.5) — the
// consent screen, the localhost redirect URL whitelist, and PKCE carry the
// security, not the secret. Overridable at build time via -ldflags -X.
var (
	DefaultClientID     = "81b9de8c-2cea-4306-8c2e-8e677c8e913a"
	DefaultClientSecret = "NlAjCdPJ7Chl3LbXfK7BmrAmVYY70VrR"
)

// DefaultPort is the loopback port for the redirect URL. The app on
// developer.bexio.com must list http://localhost:23946/callback as an
// allowed redirect URL.
const DefaultPort = 23946

// Token is the OAuth token endpoint response.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// Expiry converts ExpiresIn to an absolute time, with a safety margin so a
// token is refreshed before it actually runs out mid-request.
func (t Token) Expiry() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn)*time.Second - 30*time.Second)
}

func postToken(ctx context.Context, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(data))
	}
	if tok.Error != "" || resp.StatusCode >= 400 {
		msg := tok.ErrorDesc
		if msg == "" {
			msg = tok.Error
		}
		return nil, fmt.Errorf("token endpoint error (HTTP %d): %s", resp.StatusCode, msg)
	}
	return &tok, nil
}

// Refresh exchanges a refresh token for a new token pair. bexio rotates
// refresh tokens: always persist the returned RefreshToken.
func Refresh(ctx context.Context, clientID, clientSecret, refreshToken string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	return postToken(ctx, form)
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Login runs the Authorization Code Flow with PKCE: starts a loopback server
// on port, opens the browser (also printing the URL to status), and exchanges
// the returned code. Blocks until the redirect arrives or ctx is done.
func Login(ctx context.Context, clientID, clientSecret, scopes string, port int, status io.Writer) (*Token, error) {
	verifier, err := randomURLSafe(48)
	if err != nil {
		return nil, err
	}
	state, err := randomURLSafe(24)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on port %d for the OAuth redirect: %w", port, err)
	}
	defer ln.Close()

	authURL := AuthURL + "?" + url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		switch {
		case q.Get("state") != state:
			http.Error(w, "state mismatch", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("OAuth redirect state mismatch")}
		case q.Get("error") != "":
			fmt.Fprintf(w, "Authorization failed: %s. You can close this window.", q.Get("error"))
			resultCh <- result{err: fmt.Errorf("authorization failed: %s (%s)", q.Get("error"), q.Get("error_description"))}
		default:
			fmt.Fprint(w, "bexio CLI is authorized. You can close this window.")
			resultCh <- result{code: q.Get("code")}
		}
	})}
	go srv.Serve(ln) //nolint:errcheck // closed via ln on return
	defer srv.Close()

	fmt.Fprintf(status, "Opening browser for bexio authorization...\nIf it does not open, visit:\n\n  %s\n\n", authURL)
	openBrowser(authURL)

	var code string
	select {
	case r := <-resultCh:
		if r.err != nil {
			return nil, r.err
		}
		code = r.code
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for OAuth redirect: %w", ctx.Err())
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	return postToken(ctx, form)
}

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start() // best effort; the URL is printed either way
}
