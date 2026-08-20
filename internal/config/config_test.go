package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv(EnvConfig, p)
	t.Setenv(EnvToken, "")
	t.Setenv(EnvInstance, "")
	t.Setenv(EnvURL, "")
	return p
}

func TestLoadMissingFile(t *testing.T) {
	withTempConfig(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Instances) != 0 {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	p := withTempConfig(t)
	cfg := &Config{
		Instances: map[string]Instance{
			"acme":  {Token: "tok-1"},
			"other": {ClientID: "cid", ClientSecret: "cs", RefreshToken: "rt"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file mode = %v, want 0600", perm)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Instances["acme"].Token != "tok-1" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if !got.Instances["other"].OAuth() {
		t.Fatal("expected OAuth() for client_id instance")
	}
}

func TestResolveEnvToken(t *testing.T) {
	withTempConfig(t)
	t.Setenv(EnvToken, "env-token")
	name, inst, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "(env)" || inst.Token != "env-token" {
		t.Fatalf("got %q %+v", name, inst)
	}
}

func TestResolveOrder(t *testing.T) {
	withTempConfig(t)
	cfg := &Config{
		Instances: map[string]Instance{
			"a": {Token: "ta"},
			"b": {Token: "tb"},
			"c": {Token: "tc"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// several instances, nothing selected -> explicit choice required
	if _, _, err := Resolve(""); err == nil {
		t.Fatal("expected error with several instances and no selection")
	}
	// env instance selects
	t.Setenv(EnvInstance, "b")
	if name, _, _ := Resolve(""); name != "b" {
		t.Fatalf("env: got %q", name)
	}
	// flag beats env
	if name, _, _ := Resolve("c"); name != "c" {
		t.Fatalf("flag: got %q", name)
	}
	// unknown instance errors
	if _, _, err := Resolve("nope"); err == nil {
		t.Fatal("expected error for unknown instance")
	}
}

func TestResolveSingleInstanceAuto(t *testing.T) {
	withTempConfig(t)
	if err := Save(&Config{Instances: map[string]Instance{"acme-ag": {Token: "t"}}}); err != nil {
		t.Fatal(err)
	}
	name, inst, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "acme-ag" || inst.Token != "t" {
		t.Fatalf("got %q %+v", name, inst)
	}
}

func TestResolveNotLoggedIn(t *testing.T) {
	withTempConfig(t)
	if _, _, err := Resolve(""); err == nil {
		t.Fatal("expected not-logged-in error")
	}
}

func TestBaseURL(t *testing.T) {
	withTempConfig(t)
	if got := (Instance{}).BaseURL(); got != DefaultBaseURL {
		t.Fatalf("got %q", got)
	}
	if got := (Instance{URL: "https://example.test"}).BaseURL(); got != "https://example.test" {
		t.Fatalf("got %q", got)
	}
	t.Setenv(EnvURL, "http://127.0.0.1:1")
	if got := (Instance{URL: "https://example.test"}).BaseURL(); got != "http://127.0.0.1:1" {
		t.Fatalf("env override: got %q", got)
	}
}
