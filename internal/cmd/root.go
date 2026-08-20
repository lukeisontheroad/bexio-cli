// Package cmd implements the bexio command tree.
package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/auth"
	"github.com/lukeisontheroad/bexio-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagInstance string
	flagOutput   string
	flagVerbose  bool
	flagReadOnly bool
)

func Execute(version string) error {
	return newRootCmd(version).Execute()
}

// newRootCmd builds the full command tree (also used by tests).
func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "bexio",
		Short: "Work with bexio contacts from the command line",
		Long: `Work with bexio contacts from the command line.

Authentication: run "bexio auth login" once (browser-based OAuth, never
expires), or use a Personal Access Token from
https://developer.bexio.com/pat via --token, or set the BEXIO_TOKEN
environment variable.

Every read command supports "-o json" for machine-readable output (raw
bexio API objects, exact field names from docs.bexio.com).

Run "bexio docs" for the complete reference in one page (recommended
for LLM/agent use).`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagInstance, "instance", "", "config instance (account) to use")
	root.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", `output format: "table" (human) or "json" (raw API objects, for scripting)`)
	root.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "log HTTP requests to stderr")
	root.PersistentFlags().BoolVar(&flagReadOnly, "read-only", false, "refuse all modifying requests for this invocation")

	root.AddCommand(newAuthCmd())
	root.AddCommand(newContactCmd())
	root.AddCommand(newGroupCmd())
	root.AddCommand(newRelationCmd())
	root.AddCommand(newSectorCmd())
	root.AddCommand(newAPICmd())
	root.AddCommand(newDocsCmd())
	for _, f := range moduleCmds {
		root.AddCommand(f())
	}

	return root
}

// moduleCmds collects resource command constructors registered via init() in
// their own file (registerModule), so adding a module never edits root.go.
var moduleCmds []func() *cobra.Command

func registerModule(f func() *cobra.Command) {
	moduleCmds = append(moduleCmds, f)
}

// newClient resolves the active instance and returns an API client for it.
func newClient() (*api.Client, error) {
	name, p, err := config.Resolve(flagInstance)
	if err != nil {
		return nil, err
	}
	var source api.TokenSource
	if p.OAuth() {
		source = &oauthSource{name: name, instance: p}
	} else {
		source = api.StaticToken(p.Token)
	}
	c, err := api.New(p.BaseURL(), source)
	if err != nil {
		return nil, err
	}
	c.Verbose = flagVerbose
	c.ReadOnly = p.ReadOnly || flagReadOnly
	return c, nil
}

// oauthSource serves the cached access token and refreshes it via the OAuth
// refresh token grant when expired, persisting the rotated tokens.
type oauthSource struct {
	name     string
	instance config.Instance
}

func (s *oauthSource) Token(ctx context.Context) (string, error) {
	if s.instance.AccessToken != "" && time.Now().Before(s.instance.Expiry) {
		return s.instance.AccessToken, nil
	}
	tok, err := auth.Refresh(ctx, s.instance.ClientID, s.instance.ClientSecret, s.instance.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh access token: %w (run `bexio auth login` again)", err)
	}
	s.instance.AccessToken = tok.AccessToken
	s.instance.Expiry = tok.Expiry()
	if tok.RefreshToken != "" {
		s.instance.RefreshToken = tok.RefreshToken
	}
	// Persist the rotated tokens; a failure here only costs an extra
	// refresh on the next run.
	if cfg, err := config.Load(); err == nil {
		if cur, ok := cfg.Instances[s.name]; ok {
			cur.AccessToken = s.instance.AccessToken
			cur.Expiry = s.instance.Expiry
			cur.RefreshToken = s.instance.RefreshToken
			cfg.Instances[s.name] = cur
			_ = config.Save(cfg)
		}
	}
	return s.instance.AccessToken, nil
}

func validateOutput() error {
	if flagOutput != "table" && flagOutput != "json" {
		return fmt.Errorf("invalid --output %q (want table or json)", flagOutput)
	}
	return nil
}

// parseID parses a positional numeric id argument.
func parseID(kind, arg string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s id %q", kind, arg)
	}
	return id, nil
}

// listFlags adds the standard 2.0 list parameters to cmd.
func listFlags(cmd *cobra.Command, opts *api.ListOptions, orderFields string) {
	cmd.Flags().StringVar(&opts.OrderBy, "order-by", "", "sort by field(s), comma-separated, suffix _desc to reverse ("+orderFields+")")
	cmd.Flags().IntVar(&opts.Limit, "limit", 100, "maximum number of results (API max 2000)")
	cmd.Flags().IntVar(&opts.Offset, "offset", 0, "skip this many results (pagination)")
}

// whereOps maps CLI operators to bexio search criteria, longest first so
// two-character operators win over their one-character prefixes.
var whereOps = []struct{ op, criteria string }{
	{"!=", "not_equal"},
	{">=", "greater_equal"},
	{"<=", "less_equal"},
	{"!~", "not_like"},
	{"=", "equal"},
	{">", "greater_than"},
	{"<", "less_than"},
	{"~", "like"},
}

// parseWhere turns --where clauses like "name_1~Meyer" or "nr>10" into
// search criteria. All clauses are combined with AND by the API.
func parseWhere(clauses []string) ([]api.SearchCriterion, error) {
	var out []api.SearchCriterion
	for _, c := range clauses {
		crit, err := parseWhereClause(c)
		if err != nil {
			return nil, err
		}
		out = append(out, crit)
	}
	return out, nil
}

func parseWhereClause(clause string) (api.SearchCriterion, error) {
	for _, w := range whereOps {
		if i := strings.Index(clause, w.op); i > 0 {
			return api.SearchCriterion{
				Field:    strings.TrimSpace(clause[:i]),
				Value:    strings.TrimSpace(clause[i+len(w.op):]),
				Criteria: w.criteria,
			}, nil
		}
	}
	return api.SearchCriterion{}, fmt.Errorf("invalid --where %q (want field=value, field!=value, field~value, field!~value, field>value, field>=value, field<value, or field<=value)", clause)
}

// setIfChanged copies a flag value into the API payload only when the user
// provided the flag, so updates stay partial.
func setIfChanged(cmd *cobra.Command, fields map[string]any, flag, apiField string, value any) {
	if cmd.Flags().Changed(flag) {
		fields[apiField] = value
	}
}

func shortDate(iso string) string {
	if len(iso) >= 16 {
		return strings.Replace(iso[:16], "T", " ", 1)
	}
	return iso
}
