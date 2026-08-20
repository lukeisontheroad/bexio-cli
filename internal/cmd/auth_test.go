package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/lukeisontheroad/bexio-cli/internal/config"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Acme AG":        "acme-ag",
		"  Müller & Co ": "m-ller-co",
		"---":            "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// Logins are stored under the slugified company name, so one login per
// company never collides and re-authenticating just refreshes the entry.
func TestLoginNamesInstanceAfterCompany(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"T","lastname":"U","email":"t@example.com"}`))
		case "/2.0/company_profile":
			w.Write([]byte(`[{"id":1,"name":"Acme AG"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvConfig, filepath.Join(t.TempDir(), "config.yml"))
	t.Setenv(config.EnvURL, srv.URL)
	t.Setenv(config.EnvToken, "")

	login := func(args ...string) error {
		root := newRootCmd("test")
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(append([]string{"auth", "login"}, args...))
		err := root.Execute()
		flagInstance, flagOutput, flagVerbose = "", "table", false
		return err
	}

	if err := login("--token", "tok-1"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Instances["acme-ag"].Token != "tok-1" {
		t.Fatalf("instance not named after company: %+v", cfg.Instances)
	}

	// Same company again: refreshes the same entry, no duplicate.
	if err := login("--token", "tok-2"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if len(cfg.Instances) != 1 || cfg.Instances["acme-ag"].Token != "tok-2" {
		t.Fatalf("re-login did not refresh in place: %+v", cfg.Instances)
	}

	// Explicit --name overrides.
	if err := login("--token", "tok-3", "--name", "work"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if cfg.Instances["work"].Token != "tok-3" || cfg.Instances["acme-ag"].Token != "tok-2" {
		t.Fatalf("named login wrong: %+v", cfg.Instances)
	}
}
