package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// newDocsCmd prints the CLI reference for LLM/agent consumption. The bare
// command stays deliberately small (conventions + command index, ~2k
// tokens) so it can sit in an agent's context without crowding it;
// per-command details are fetched on demand with `docs <command>`, and
// `--full` dumps everything for offline use.
func newDocsCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "docs [command]",
		Short: "Print the CLI reference (optimized for LLMs/agents)",
		Long: `Print the CLI reference, written for LLM/agent consumption.

Without arguments: setup, conventions, search syntax, document workflows,
and a one-line index of every command — compact enough to keep in context.
With a command name ("bexio docs kb-invoice"): all subcommands, flags, and
examples of that command only. --full prints everything at once.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			root := cmd.Root()
			if len(args) == 1 {
				target := findCommand(root, args[0])
				if target == nil {
					return fmt.Errorf("unknown command %q: run `bexio docs` for the index", args[0])
				}
				printCmdDocs(out, target)
				return nil
			}
			fmt.Fprint(out, docsHeader)
			if full {
				printCmdDocs(out, root)
			} else {
				printCmdIndex(out, root)
			}
			fmt.Fprint(out, docsFooter)
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "print every subcommand with flags and examples (long)")
	return cmd
}

// findCommand resolves a top-level command by name or alias.
func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, c := range root.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

// printCmdIndex writes one line per top-level command plus the hint how to
// drill down.
func printCmdIndex(w io.Writer, root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" || c.Name() == "docs" {
			continue
		}
		name := c.Name()
		if len(c.Aliases) > 0 {
			name += " (" + strings.Join(c.Aliases, ", ") + ")"
		}
		subs := make([]string, 0, len(c.Commands()))
		for _, s := range c.Commands() {
			if !s.Hidden {
				subs = append(subs, s.Name())
			}
		}
		fmt.Fprintf(w, "- %s — %s", name, c.Short)
		if len(subs) > 0 {
			fmt.Fprintf(w, " [%s]", strings.Join(subs, ", "))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprint(w, `
Per-command details (subcommands, flags, examples):
"bexio docs <command>", e.g. "bexio docs kb-invoice" — fetch these on
demand instead of dumping everything ("bexio docs --full") into context.
`)
}

const docsHeader = `# bexio CLI reference

Command-line client for the bexio business software: contacts, quotes
(kb-offer), sales orders (kb-order), invoices (kb-invoice), items & stock,
deliveries, projects & timesheets, notes & tasks, and master data. All
commands are non-interactive (except a bare "auth login") and print to
stdout; errors go to stderr with exit code 1, success is exit code 0.

## Setup

Authentication (pick one):
- OAuth (default): "bexio auth login" runs a browser flow with the CLI's
  built-in app and refreshes tokens automatically (never expires). An
  interactive checklist (or --modules contacts,invoices,...) limits the
  requested scopes to the selected modules. A custom app works via
  --client-id/--client-secret; it must allow the redirect URL
  http://localhost:23946/callback.
- Token: run "bexio auth login --token <PAT>" once (or --pat to be
  prompted). Create a Personal Access Token at
  https://developer.bexio.com/pat (all scopes, valid 60 days).
- Environment: set BEXIO_TOKEN (overrides the config file).

Read-only mode: "bexio auth login --read-only" requests only read scopes
(server-enforced) and marks the instance read-only (client refuses all
modifying requests; searches still work). Per invocation: the global
"--read-only" flag or BEXIO_READ_ONLY=1 — recommended when an agent only
needs to look things up.

Multiple bexio companies: log in once per company; each login is stored
as an instance named after the company (slugified, e.g. "acme-ag";
override with --name). With one instance it is used automatically; with
several, select per call with "--instance <name>" or the BEXIO_INSTANCE
environment variable ("bexio auth status" shows the instance names).
There is no sticky default instance.

## Conventions

- The CLI mirrors the bexio API scheme (https://docs.bexio.com): resources
  keep their API names (contact, contact-group, contact-relation,
  contact-sector) and field flags map 1:1 to API fields
  (--name-1 -> name_1, --contact-group-ids -> contact_group_ids, ...).
- Every read command supports "-o json", printing the raw bexio API objects
  (exact API field names, machine-parseable). Prefer it when consuming
  output programmatically; the default table view truncates long values.
- Contacts: contact_type_id 1 = company, 2 = person. For persons name_1 is
  the last name and name_2 the first name.
- "contact delete" archives (restore with "contact restore"); listing
  archived contacts: "contact list --archived".
- "--verbose" logs each HTTP request to stderr for debugging.

## Search syntax (--where)

Search commands take repeatable --where clauses over raw API field names,
combined with AND:
  field=value    exact          field~value    partial (like)
  field!=value   not equal      field!~value   not like
  field>value    greater        field>=value   greater or equal
  field<value    less           field<=value   less or equal
Values for ~ are SQL-like patterns; add % wildcards yourself
(bare search terms are wrapped in % automatically).

## Document workflows (kb resources)

- kb-offer (quote) statuses: 1 draft, 2 pending, 3 confirmed, 4 declined.
  Lifecycle: issue -> accept|reject (revert-issue, reissue, mark-as-sent);
  convert with "kb-offer order" / "kb-offer invoice".
- kb-order statuses: 5 pending, 6 done, 15 partial, 21 canceled. Convert
  with "kb-order invoice" / "kb-order delivery".
- kb-invoice statuses: 7 draft, 8 pending, 9 paid, 16 partial, 19 canceled,
  31 unpaid — read-only, driven by issue/revert-issue/cancel and payments.
  Nested: "kb-invoice payment ..." and "kb-invoice reminder ...".
- kb-delivery statuses: 10 draft, 18 done, 20 canceled. Deliveries are
  created from orders; "kb-delivery issue" books the stock movements.
- "send" commands need --recipient-email/--subject/--message, and the
  message must contain the literal "[Network Link]" placeholder.
- Positions on quotes/orders/invoices: repeatable
  --position "type=article,article_id=5,amount=2" specs on create, and
  "position add|update|delete" on existing documents. Types: article,
  custom, text, subtotal, discount, pagebreak, subposition.
- Deleting kb documents and projects is permanent (--force where offered);
  contact delete archives (restorable), projects archive/reactivate.

## Commands

`

const docsFooter = `
## Raw API access

"bexio api METHOD /2.0/..." reaches any endpoint with authentication
handled. GET/DELETE: -f key=value pairs become query parameters
(repeatable). Other methods: pairs form a flat JSON body, or --body sends
raw JSON (needed for array bodies like the search endpoints). Responses
print as JSON. Endpoint reference: https://docs.bexio.com
`

// printCmdDocs walks the command tree and prints usage, description,
// examples, and flags for every runnable command.
func printCmdDocs(w io.Writer, c *cobra.Command) {
	if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
		return
	}
	if c.Runnable() && c.Name() != "docs" {
		fmt.Fprintf(w, "### %s\n", c.CommandPath())
		fmt.Fprintf(w, "Usage: %s\n", c.UseLine())
		desc := c.Long
		if desc == "" {
			desc = c.Short
		}
		if desc != "" {
			fmt.Fprintln(w, desc)
		}
		if c.Example != "" {
			fmt.Fprintf(w, "Examples:\n%s\n", c.Example)
		}
		flags := c.NonInheritedFlags()
		if flags.HasAvailableFlags() {
			fmt.Fprintf(w, "Flags:\n%s", flags.FlagUsages())
		}
		fmt.Fprintln(w)
	}
	for _, sub := range c.Commands() {
		printCmdDocs(w, sub)
	}
}
