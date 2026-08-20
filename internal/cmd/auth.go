package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/auth"
	"github.com/lukeisontheroad/bexio-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in to bexio and inspect auth status",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthStatusCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var tokenFlag, nameFlag, clientID, clientSecret, scopes string
	var modules []string
	var port int
	var patFlag, readOnly bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the bexio API",
		Long: `Authenticate against the bexio API. Two methods:

OAuth (default): browser-based Authorization Code Flow using the CLI's
built-in app. Tokens refresh automatically, so this login never
expires. An interactive checklist (or --modules) picks which modules
to authorize; only the scopes for the selected modules are requested.
To use your own app from https://developer.bexio.com instead, pass
--client-id (and --client-secret for confidential apps); the app must
list http://localhost:23946/callback (or your --port) as an allowed
redirect URL.

Token (--token / --pat): use a Personal Access Token (PAT) or API
token as-is. Create a PAT at https://developer.bexio.com/pat — it has
all scopes and is valid for 60 days after creation.`,
		Example: `  bexio auth login                          # browser flow, never expires
  bexio auth login --modules contacts,invoices
  bexio auth login --name work              # second bexio company
  bexio auth login --token $TOKEN           # static PAT / API token
  bexio auth login --pat                    # prompts for a token (hidden input)
  bexio auth login --client-id ID --client-secret SECRET`,
		RunE: func(cmd *cobra.Command, args []string) error {
			useToken := patFlag || tokenFlag != ""
			if !useToken && clientID == "" {
				clientID, clientSecret = auth.DefaultClientID, auth.DefaultClientSecret
			}

			var instance config.Instance
			if !useToken {
				if scopes == "" {
					sel := modules
					if !cmd.Flags().Changed("modules") && term.IsTerminal(int(os.Stdin.Fd())) {
						var err error
						sel, err = promptModules(cmd)
						if err != nil {
							return err
						}
					}
					if len(sel) == 1 && strings.EqualFold(sel[0], "all") {
						sel = nil
					}
					var err error
					scopes, err = auth.ScopesFor(sel, readOnly)
					if err != nil {
						return err
					}
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
				defer cancel()
				tok, err := auth.Login(ctx, clientID, clientSecret, scopes, port, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if tok.RefreshToken == "" {
					return fmt.Errorf("no refresh token received: make sure the offline_access scope is allowed for the app")
				}
				instance = config.Instance{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					Scopes:       scopes,
					RefreshToken: tok.RefreshToken,
					AccessToken:  tok.AccessToken,
					Expiry:       tok.Expiry(),
					ReadOnly:     readOnly,
				}
			} else {
				token := tokenFlag
				if token == "" {
					out := cmd.OutOrStdout()
					fmt.Fprintln(out, "Create a Personal Access Token at https://developer.bexio.com/pat")
					fmt.Fprintln(out, "(valid 60 days, all scopes; for non-expiring auth use plain `bexio auth login`).")
					fmt.Fprint(out, "\nToken (input hidden): ")
					if term.IsTerminal(int(os.Stdin.Fd())) {
						b, err := term.ReadPassword(int(os.Stdin.Fd()))
						fmt.Fprintln(out)
						if err != nil {
							return err
						}
						token = strings.TrimSpace(string(b))
					} else {
						line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
						if err != nil {
							return err
						}
						token = strings.TrimSpace(line)
					}
				}
				if token == "" {
					return fmt.Errorf("token is required")
				}
				instance = config.Instance{Token: token, ReadOnly: readOnly}
			}

			// Verify the credentials before saving.
			var source api.TokenSource
			if instance.OAuth() {
				source = api.StaticToken(instance.AccessToken)
			} else {
				source = api.StaticToken(instance.Token)
			}
			client, err := api.New(instance.BaseURL(), source)
			if err != nil {
				return err
			}
			client.Verbose = flagVerbose
			me, err := client.Me(cmd.Context())
			if err != nil {
				return fmt.Errorf("token check failed: %w", err)
			}

			company := client.CompanyName(cmd.Context())

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			// Instances are named after the company automatically, so a
			// login per company never collides and re-authenticating the
			// same company just refreshes its entry. There is no default
			// instance: with one login it is picked automatically, with
			// several the caller selects via --instance / BEXIO_INSTANCE.
			name := nameFlag
			if name == "" {
				name = slugify(company)
			}
			if name == "" {
				return fmt.Errorf("could not determine the company name for this token: pass --name")
			}
			cfg.Instances[name] = instance
			if err := config.Save(cfg); err != nil {
				return err
			}

			p, err := config.Path()
			if err != nil {
				p = "(unknown path)"
			}
			if company != "" {
				company = " @ " + company
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s (%s)%s\nConfig written to %s (instance %q)\n",
				me.DisplayName(), me.Email, company, p, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenFlag, "token", "", "authenticate with a static Personal Access Token or API token")
	cmd.Flags().BoolVar(&patFlag, "pat", false, "authenticate with a static token, prompting for it (hidden input)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "instance name in the config (default: the company name, slugified)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client id of your own app (default: the CLI's built-in app)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth client secret (confidential apps)")
	cmd.Flags().StringSliceVar(&modules, "modules", []string{"all"}, "modules to authorize: "+strings.Join(auth.AllModuleNames(), ", ")+`, or "all"`)
	cmd.Flags().StringVar(&scopes, "scopes", "", "raw OAuth scopes to request (overrides --modules)")
	cmd.Flags().IntVar(&port, "port", auth.DefaultPort, "loopback port for the OAuth redirect URL")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "read-only login: request only read scopes (OAuth) and refuse all modifying requests")
	return cmd
}

// slugify turns a company name into a config-friendly instance name
// ("Acme AG" -> "acme-ag").
func slugify(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// promptModules shows an interactive, pre-checked checkbox list of modules
// for the OAuth consent (caller ensures stdin is a TTY). Returns nil when
// everything stays selected.
func promptModules(cmd *cobra.Command) ([]string, error) {
	out := cmd.OutOrStdout()
	items := make([]checkboxItem, len(auth.Modules))
	for i, m := range auth.Modules {
		items[i] = checkboxItem{Label: m.Name, Description: m.Description, Checked: true}
	}
	sel, err := checkboxSelect(out, "Modules to authorize (↑/↓ move, space toggle, a all, enter confirm):", items)
	if err != nil {
		return nil, err
	}
	if len(sel) == 0 {
		return nil, fmt.Errorf("no modules selected")
	}
	if len(sel) == len(items) {
		fmt.Fprintln(out, "Authorizing all modules.")
		return nil, nil
	}
	names := make([]string, len(sel))
	for i, idx := range sel {
		names[i] = auth.Modules[idx].Name
	}
	fmt.Fprintf(out, "Authorizing: %s\n", strings.Join(names, ", "))
	return names, nil
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active instance and authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, p, err := config.Resolve(flagInstance)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			me, err := client.Me(cmd.Context())
			if err != nil {
				return fmt.Errorf("instance %s: token check failed: %w", name, err)
			}
			kind := "token (PAT / API token)"
			if p.OAuth() {
				kind = "OAuth (client " + p.ClientID + ")"
			}
			if p.ReadOnly || flagReadOnly {
				kind += ", read-only"
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Instance: %s\nAuth:    %s\nUser:    %s (%s)\n", name, kind, me.DisplayName(), me.Email)
			if company := client.CompanyName(cmd.Context()); company != "" {
				fmt.Fprintf(out, "Company: %s\n", company)
			}
			return nil
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the active instance from the config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name := flagInstance
			if name == "" {
				name = os.Getenv(config.EnvInstance)
			}
			if name == "" && len(cfg.Instances) == 1 {
				for k := range cfg.Instances {
					name = k
				}
			}
			if name == "" {
				return fmt.Errorf("no instance selected: pass --instance")
			}
			if _, ok := cfg.Instances[name]; !ok {
				return fmt.Errorf("unknown instance %q", name)
			}
			delete(cfg.Instances, name)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed instance %q from config\n", name)
			return nil
		},
	}
}
