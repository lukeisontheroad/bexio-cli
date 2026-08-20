package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q", got)
		}
		if got := r.Form.Get("refresh_token"); got != "old-rt" {
			t.Errorf("refresh_token = %q", got)
		}
		if got := r.Form.Get("client_id"); got != "cid" {
			t.Errorf("client_id = %q", got)
		}
		if got := r.Form.Get("client_secret"); got != "cs" {
			t.Errorf("client_secret = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-at","refresh_token":"new-rt","expires_in":3600}`))
	}))
	defer srv.Close()
	orig := TokenURL
	TokenURL = srv.URL
	defer func() { TokenURL = orig }()

	tok, err := Refresh(context.Background(), "cid", "cs", "old-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new-at" || tok.RefreshToken != "new-rt" {
		t.Fatalf("got %+v", tok)
	}
}

func TestRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant","error_description":"Token is not active"}`))
	}))
	defer srv.Close()
	orig := TokenURL
	TokenURL = srv.URL
	defer func() { TokenURL = orig }()

	if _, err := Refresh(context.Background(), "cid", "", "expired"); err == nil {
		t.Fatal("expected error")
	}
}
