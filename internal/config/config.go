// Package config handles the bexio CLI configuration file and instance
// resolution.
//
// Resolution order for the active instance:
//  1. BEXIO_TOKEN environment variable (full override, no file needed)
//  2. --instance flag
//  3. BEXIO_INSTANCE environment variable
//  4. the sole configured instance (several -> explicit selection required)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EnvToken    = "BEXIO_TOKEN"
	EnvInstance = "BEXIO_INSTANCE"
	EnvConfig   = "BEXIO_CONFIG"    // override config file path
	EnvURL      = "BEXIO_URL"       // override API base URL (testing)
	EnvReadOnly = "BEXIO_READ_ONLY" // any non-empty value forces read-only

	// DefaultBaseURL is the bexio API live server.
	DefaultBaseURL = "https://api.bexio.com"
)

// Instance is one authenticated account. Either Token (a static PAT or API
// token, used as-is as a bearer token) or the OAuth fields are set.
type Instance struct {
	Token string `yaml:"token,omitempty"`

	// OAuth (Authorization Code Flow) credentials and cached tokens.
	ClientID     string    `yaml:"client_id,omitempty"`
	ClientSecret string    `yaml:"client_secret,omitempty"`
	Scopes       string    `yaml:"scopes,omitempty"`
	RefreshToken string    `yaml:"refresh_token,omitempty"`
	AccessToken  string    `yaml:"access_token,omitempty"`
	Expiry       time.Time `yaml:"access_token_expiry,omitempty"`

	// ReadOnly makes the CLI refuse every modifying request for this
	// instance (reads and searches only).
	ReadOnly bool `yaml:"read_only,omitempty"`

	// URL overrides the API base URL (default DefaultBaseURL).
	URL string `yaml:"url,omitempty"`
}

// OAuth reports whether the instance authenticates via the OAuth flow (as
// opposed to a static token).
func (p Instance) OAuth() bool { return p.ClientID != "" }

// BaseURL returns the API base URL for the instance, honoring BEXIO_URL.
func (p Instance) BaseURL() string {
	if u := os.Getenv(EnvURL); u != "" {
		return u
	}
	if p.URL != "" {
		return p.URL
	}
	return DefaultBaseURL
}

type Config struct {
	Instances map[string]Instance `yaml:"instances,omitempty"`
}

// Path returns the config file location: $BEXIO_CONFIG, or
// $XDG_CONFIG_HOME/bexio/config.yml, defaulting to ~/.config/bexio/config.yml
// on every platform (like gh/glab; deliberately not os.UserConfigDir, whose
// macOS result "Library/Application Support" is hostile to terminal use).
func Path() (string, error) {
	if p := os.Getenv(EnvConfig); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bexio", "config.yml"), nil
}

// Load reads the config file. A missing file yields an empty config, not an error.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{Instances: map[string]Instance{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if cfg.Instances == nil {
		cfg.Instances = map[string]Instance{}
	}
	return &cfg, nil
}

// Save writes the config file with 0600 permissions, creating the directory
// if needed.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Resolve returns the active instance. flagInstance is the value of --instance
// ("" if unset). The returned name is "(env)" when BEXIO_TOKEN is used; such
// an instance must not be saved back to the config file.
func Resolve(flagInstance string) (name string, p Instance, err error) {
	if t := os.Getenv(EnvToken); t != "" {
		return "(env)", Instance{Token: t, ReadOnly: os.Getenv(EnvReadOnly) != ""}, nil
	}
	cfg, err := Load()
	if err != nil {
		return "", Instance{}, err
	}
	name = flagInstance
	if name == "" {
		name = os.Getenv(EnvInstance)
	}
	if name == "" {
		// No default instance is stored: a single login is unambiguous,
		// with several the caller must pick one explicitly.
		switch len(cfg.Instances) {
		case 0:
			return "", Instance{}, fmt.Errorf("not logged in: run `bexio auth login` or set %s", EnvToken)
		case 1:
			for k := range cfg.Instances {
				name = k
			}
		default:
			return "", Instance{}, fmt.Errorf("several instances configured (%s): pass --instance or set %s", keys(cfg.Instances), EnvInstance)
		}
	}
	p, ok := cfg.Instances[name]
	if !ok {
		return "", Instance{}, fmt.Errorf("unknown instance %q in config (available: %s)", name, keys(cfg.Instances))
	}
	if p.Token == "" && !p.OAuth() {
		return "", Instance{}, fmt.Errorf("instance %q has no credentials: run `bexio auth login`", name)
	}
	if os.Getenv(EnvReadOnly) != "" {
		p.ReadOnly = true
	}
	return name, p, nil
}

func keys(m map[string]Instance) string {
	if len(m) == 0 {
		return "none"
	}
	s := ""
	for k := range m {
		if s != "" {
			s += ", "
		}
		s += k
	}
	return s
}
