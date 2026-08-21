package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// This file implements the accounting commands: manual-entry (incl. file
// attachments), account, account-group, business-year, calendar-year,
// vat-period, and the journal report.

func init() {
	registerModule(newManualEntryCmd)
	registerModule(newAccountCmd)
	registerModule(newAccountGroupCmd)
	registerModule(newBusinessYearCmd)
	registerModule(newCalendarYearCmd)
	registerModule(newVatPeriodCmd)
	registerModule(newJournalCmd)
}

// newManualEntryCmd manages manual accounting entries (API resource
// "accounting/manual_entries").
func newManualEntryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "manual-entry",
		Aliases: []string{"manual-entries"},
		Short:   "List, view, and modify manual accounting entries",
	}
	cmd.AddCommand(
		newManualEntryListCmd(),
		newManualEntryViewCmd(),
		newManualEntryCreateCmd(),
		newManualEntryUpdateCmd(),
		newManualEntryDeleteCmd(),
		newManualEntryNextRefNrCmd(),
		newManualEntryFileCmd(),
	)
	return cmd
}

var manualEntryDetailOrder = []string{
	"id", "type", "date", "reference_nr", "is_locked", "locked_info",
	"created_by_user_id", "edited_by_user_id", "entries",
}

func formatAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func renderManualEntries(cmd *cobra.Command, entries []api.ManualEntry) error {
	return renderLookupList(cmd, entries,
		[]string{"id", "date", "type", "reference_nr", "lines", "amount", "locked"},
		func(m api.ManualEntry) json.RawMessage { return m.Raw },
		func(m api.ManualEntry) []string {
			return []string{
				strconv.Itoa(m.ID),
				m.Date,
				strings.TrimPrefix(m.Type, "manual_"),
				output.Truncate(m.ReferenceNr, 30),
				strconv.Itoa(len(m.Entries)),
				formatAmount(m.Total()),
				strconv.FormatBool(m.IsLocked),
			}
		})
}

func newManualEntryListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List manual entries",
		Example: `  bexio manual-entry list
  bexio manual-entry list --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			entries, err := client.ListManualEntries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderManualEntries(cmd, entries)
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newManualEntryViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single manual entry (including its lines)",
		Long: `Show a single manual entry.

The 3.0 API has no "fetch one" endpoint for manual entries, so the CLI
pages through the list endpoint and matches on id.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			m, err := client.GetManualEntry(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, m.Raw, manualEntryDetailOrder)
		},
	}
}

// manualEntryLineIntKeys / manualEntryLineFloatKeys type the numeric keys of
// an --entry spec; every other key is sent as a string.
var manualEntryLineIntKeys = map[string]bool{
	"id": true, "debit_account_id": true, "credit_account_id": true,
	"tax_id": true, "tax_account_id": true, "currency_id": true,
}

var manualEntryLineFloatKeys = map[string]bool{
	"amount": true, "currency_factor": true,
}

// parseManualEntryLineSpec parses
// "debit_account_id=1,credit_account_id=2,amount=100.00,description=Rent"
// into one entry of the nested entries array. Keys are raw API field names.
func parseManualEntryLineSpec(spec string) (map[string]any, error) {
	fields := map[string]any{}
	for _, part := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return nil, fmt.Errorf("invalid entry spec %q: %q is not key=value", spec, part)
		}
		k = strings.TrimSpace(k)
		switch {
		case manualEntryLineIntKeys[k]:
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid entry spec %q: %s must be a number", spec, k)
			}
			fields[k] = n
		case manualEntryLineFloatKeys[k]:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid entry spec %q: %s must be a decimal number", spec, k)
			}
			fields[k] = f
		default:
			fields[k] = v
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty entry spec")
	}
	return fields, nil
}

// manualEntryFieldFlags mirrors the payload fields of POST/PUT
// /3.0/accounting/manual_entries.
type manualEntryFieldFlags struct {
	entryType   string
	date        string
	referenceNr string
	entries     []string
	entriesJSON string
}

// register adds one flag per API field. defaultType is the value used when
// --type is omitted (create defaults to a single entry; update leaves it
// empty so an existing compound/group entry is never silently converted).
func (f *manualEntryFieldFlags) register(cmd *cobra.Command, defaultType string) {
	fl := cmd.Flags()
	fl.StringVar(&f.entryType, "type", defaultType,
		"entry type: manual_single_entry, manual_compound_entry, or manual_group_entry")
	fl.StringVar(&f.date, "date", "", "booking date (YYYY-MM-DD)")
	fl.StringVar(&f.referenceNr, "reference-nr", "", "reference number (see `bexio manual-entry next-ref-nr`)")
	fl.StringArrayVar(&f.entries, "entry", nil,
		`booking line as "key=value,..." (repeatable); keys: debit_account_id, credit_account_id, amount, description, tax_id, tax_account_id, currency_id, currency_factor, id`)
	fl.StringVar(&f.entriesJSON, "entries-json", "", "raw JSON array for entries (escape hatch, overrides --entry)")
}

// payload collects the fields the user set, parsing --entry/--entries-json
// into the nested entries array.
func (f *manualEntryFieldFlags) payload(cmd *cobra.Command) (map[string]any, error) {
	fields := map[string]any{}
	if f.entryType != "" {
		valid := false
		for _, t := range api.ManualEntryTypes {
			if t == f.entryType {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("--type must be one of %s", strings.Join(api.ManualEntryTypes, ", "))
		}
		fields["type"] = f.entryType
	}
	setIfChanged(cmd, fields, "date", "date", f.date)
	setIfChanged(cmd, fields, "reference-nr", "reference_nr", f.referenceNr)

	var lines []any
	for _, spec := range f.entries {
		line, err := parseManualEntryLineSpec(spec)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if f.entriesJSON != "" {
		if len(lines) > 0 {
			return nil, fmt.Errorf("--entry and --entries-json are mutually exclusive")
		}
		var raw []any
		if err := json.Unmarshal([]byte(f.entriesJSON), &raw); err != nil {
			return nil, fmt.Errorf("--entries-json is not a valid JSON array: %w", err)
		}
		lines = raw
	}
	if len(lines) > 0 {
		fields["entries"] = lines
	}
	return fields, nil
}

func newManualEntryCreateCmd() *cobra.Command {
	var fields manualEntryFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a manual entry",
		Long: `Create a manual entry. --date and at least one --entry are required;
--type defaults to manual_single_entry.

Types: manual_single_entry (one line), manual_compound_entry (one total
distributed over several accounts), manual_group_entry (several one-line
bookings sharing one reference number).`,
		Example: `  bexio manual-entry create --date 2026-01-31 --reference-nr "BA-22" \
      --entry "debit_account_id=77,credit_account_id=139,amount=328.25,description=Rent"
  bexio manual-entry create --type manual_group_entry --date 2026-01-31 \
      --entry "debit_account_id=77,credit_account_id=139,amount=100" \
      --entry "debit_account_id=78,credit_account_id=140,amount=200"
  bexio manual-entry create --date 2026-01-31 \
      --entries-json '[{"debit_account_id":77,"credit_account_id":139,"amount":50}]'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if payload["date"] == nil {
				return fmt.Errorf("--date is required (YYYY-MM-DD)")
			}
			if payload["entries"] == nil {
				return fmt.Errorf("at least one --entry (or --entries-json) is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			m, err := client.CreateManualEntry(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), m.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created manual entry %d (%s, %d line(s), %s)\n",
				m.ID, m.Type, len(m.Entries), formatAmount(m.Total()))
			return nil
		},
	}
	fields.register(cmd, "manual_single_entry")
	return cmd
}

func newManualEntryUpdateCmd() *cobra.Command {
	var fields manualEntryFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Replace a manual entry",
		Long: `Replace a manual entry. Unlike the 2.0 resources this is a PUT: the
payload replaces the whole entry, so --type, --date and the complete set
of --entry lines are required. Fetch the current state with
"bexio manual-entry view <id> -o json" first.`,
		Example: `  bexio manual-entry update 42 --type manual_single_entry --date 2026-02-01 \
      --entry "id=32,debit_account_id=77,credit_account_id=139,amount=400"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if payload["type"] == nil || payload["date"] == nil || payload["entries"] == nil {
				return fmt.Errorf("update replaces the whole entry: --type, --date and at least one --entry (or --entries-json) are required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			m, err := client.UpdateManualEntry(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), m.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated manual entry %d (%d line(s), %s)\n",
				m.ID, len(m.Entries), formatAmount(m.Total()))
			return nil
		},
	}
	fields.register(cmd, "")
	return cmd
}

func newManualEntryDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a manual entry",
		Long:  "Permanently delete a manual entry. This CANNOT be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a manual entry is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteManualEntry(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted manual entry %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newManualEntryNextRefNrCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next-ref-nr",
		Short: "Show the suggested reference number for the next manual entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.NextManualEntryRefNr(cmd.Context())
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), r.Raw)
			}
			fmt.Fprintln(cmd.OutOrStdout(), r.NextRefNr)
			return nil
		},
	}
}

// newManualEntryFileCmd groups the file attachment endpoints. Without
// --entry-id the files of the manual (compound) entry itself are used, with
// it the files of that single booking line.
func newManualEntryFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage files attached to a manual entry",
		Long: `Manage files attached to a manual entry.

Without --entry-id the commands address the files of the manual (compound)
entry itself (/manual_entries/{id}/files); with --entry-id they address the
files of that single booking line
(/manual_entries/{id}/entries/{entry_id}/files).`,
	}
	cmd.AddCommand(
		newManualEntryFileListCmd(),
		newManualEntryFileAttachCmd(),
		newManualEntryFileViewCmd(),
		newManualEntryFileDeleteCmd(),
	)
	return cmd
}

var manualEntryFileDetailOrder = []string{
	"id", "uuid", "name", "extension", "mime_type", "size_in_bytes",
	"user_id", "uploader_email", "is_archived", "is_referenced",
	"source_type", "created_at",
}

func renderManualEntryFiles(cmd *cobra.Command, files []api.ManualEntryFile) error {
	return renderLookupList(cmd, files,
		[]string{"id", "name", "extension", "mime_type", "size", "created"},
		func(f api.ManualEntryFile) json.RawMessage { return f.Raw },
		func(f api.ManualEntryFile) []string {
			return []string{
				strconv.Itoa(f.ID),
				output.Truncate(f.Name, 40),
				f.Extension,
				f.MimeType,
				strconv.FormatInt(f.SizeInBytes, 10),
				shortDate(f.CreatedAt),
			}
		})
}

// manualEntryFileIDFlag adds the shared --entry-id switch.
func manualEntryFileIDFlag(cmd *cobra.Command, entryID *int) {
	cmd.Flags().IntVar(entryID, "entry-id", 0, "address the files of this single booking line instead of the whole entry")
}

func newManualEntryFileListCmd() *cobra.Command {
	var entryID int
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list <manual-entry-id>",
		Short: "List the files attached to a manual entry",
		Example: `  bexio manual-entry file list 42
  bexio manual-entry file list 42 --entry-id 32`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			files, err := client.ListManualEntryFiles(cmd.Context(), id, entryID, opts)
			if err != nil {
				return err
			}
			return renderManualEntryFiles(cmd, files)
		},
	}
	manualEntryFileIDFlag(cmd, &entryID)
	lookupFlags30(cmd, &opts)
	return cmd
}

func newManualEntryFileAttachCmd() *cobra.Command {
	var entryID int
	cmd := &cobra.Command{
		Use:   "attach <manual-entry-id> <file>...",
		Short: "Upload and attach files to a manual entry",
		Long: `Upload local files and attach them to a manual entry. Max 12 MB per file;
supported formats: PNG, JPG, JPEG, GIF, DOC, DOCX, XLS, XLSX, PPT, PPTX, PDF.`,
		Example: `  bexio manual-entry file attach 42 receipt.pdf
  bexio manual-entry file attach 42 receipt.pdf --entry-id 32`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			files, err := client.UploadManualEntryFiles(cmd.Context(), id, entryID, args[1:])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return renderManualEntryFiles(cmd, files)
			}
			for _, f := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "Attached file %d (%s) to manual entry %d\n", f.ID, f.Name, id)
			}
			return nil
		},
	}
	manualEntryFileIDFlag(cmd, &entryID)
	return cmd
}

func newManualEntryFileViewCmd() *cobra.Command {
	var entryID int
	var out string
	cmd := &cobra.Command{
		Use:   "view <manual-entry-id> <file-id>",
		Short: "Show an attached file (--out downloads its content)",
		Example: `  bexio manual-entry file view 42 7
  bexio manual-entry file view 42 7 --out receipt.pdf`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			fileID, err := parseID("file", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			f, err := client.GetManualEntryFile(cmd.Context(), id, entryID, fileID)
			if err != nil {
				return err
			}
			if out != "" {
				data, err := base64.StdEncoding.DecodeString(f.Data)
				if err != nil {
					return fmt.Errorf("decode file content: %w", err)
				}
				if err := os.WriteFile(out, data, 0o644); err != nil { //nolint:gosec // user-chosen download path
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d bytes)\n", out, len(data))
				return nil
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), f.Raw)
			}
			// Drop the embedded base64 content from the table view.
			var m map[string]any
			if err := json.Unmarshal(f.Raw, &m); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			delete(m, "data")
			raw, err := json.Marshal(m)
			if err != nil {
				return err
			}
			return renderDetail(cmd, raw, manualEntryFileDetailOrder)
		},
	}
	manualEntryFileIDFlag(cmd, &entryID)
	cmd.Flags().StringVar(&out, "out", "", "write the file content to this path")
	return cmd
}

func newManualEntryFileDeleteCmd() *cobra.Command {
	var entryID int
	cmd := &cobra.Command{
		Use:   "delete <manual-entry-id> <file-id>",
		Short: "Detach a file from a manual entry",
		Long:  "Delete the connection between a file and a manual entry (the file itself stays in the file manager).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("manual entry", args[0])
			if err != nil {
				return err
			}
			fileID, err := parseID("file", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteManualEntryFile(cmd.Context(), id, entryID, fileID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Detached file %d from manual entry %d\n", fileID, id)
			return nil
		},
	}
	manualEntryFileIDFlag(cmd, &entryID)
	return cmd
}

// accountTypeNames maps the account_type enum of /2.0/accounts.
var accountTypeNames = map[int]string{
	1: "earnings",
	2: "expenditures",
	3: "active",
	4: "passive",
	5: "complete",
}

var accountDetailOrder = []string{
	"id", "uuid", "account_no", "name", "account_type", "tax_id",
	"fibu_account_group_id", "is_active", "is_locked",
}

func renderAccounts(cmd *cobra.Command, accounts []api.Account) error {
	return renderLookupList(cmd, accounts,
		[]string{"id", "account_no", "name", "type", "tax_id", "group", "active", "locked"},
		func(a api.Account) json.RawMessage { return a.Raw },
		func(a api.Account) []string {
			t := accountTypeNames[a.AccountType]
			if t == "" {
				t = strconv.Itoa(a.AccountType)
			}
			tax := ""
			if a.TaxID > 0 {
				tax = strconv.Itoa(a.TaxID)
			}
			return []string{
				strconv.Itoa(a.ID),
				a.AccountNo,
				output.Truncate(a.Name, 40),
				t,
				tax,
				strconv.Itoa(a.FibuAccountGroupID),
				strconv.FormatBool(a.IsActive),
				strconv.FormatBool(a.IsLocked),
			}
		})
}

// newAccountCmd exposes the ledger accounts (API resource "accounts",
// read-only).
func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account",
		Aliases: []string{"accounts"},
		Short:   "List, search, and view ledger accounts",
	}
	cmd.AddCommand(newAccountListCmd(), newAccountSearchCmd(), newAccountViewCmd())
	return cmd
}

func newAccountListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ledger accounts",
		Example: `  bexio account list
  bexio account list --limit 500 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			accounts, err := client.ListAccounts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderAccounts(cmd, accounts)
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newAccountSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search ledger accounts",
		Long: `Search ledger accounts. A bare term matches the account name partially.
Searchable fields: id, uuid, account_no, name, account_type, fibu_account_group_id.`,
		Example: `  bexio account search Bank
  bexio account search --where account_no=1020
  bexio account search --where account_type=3 -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				criteria = append(criteria, api.SearchCriterion{
					Field: "name", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			accounts, err := client.SearchAccounts(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderAccounts(cmd, accounts)
		},
	}
	lookupFlags30(cmd, &opts)
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newAccountViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single ledger account",
		Long: `Show a single ledger account. The API has no "fetch one" endpoint for
accounts, so the CLI searches for the id.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("account", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			accounts, err := client.SearchAccounts(cmd.Context(),
				[]api.SearchCriterion{{Field: "id", Value: id, Criteria: "equal"}}, api.ListOptions{})
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fmt.Errorf("account %d not found", id)
			}
			return renderDetail(cmd, accounts[0].Raw, accountDetailOrder)
		},
	}
}

// newAccountGroupCmd exposes the account groups (read-only, list only).
func newAccountGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account-group",
		Aliases: []string{"account-groups"},
		Short:   "List account groups",
	}
	var opts api.ListOptions
	list := &cobra.Command{
		Use:     "list",
		Short:   "List account groups",
		Example: `  bexio account-group list -o json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			groups, err := client.ListAccountGroups(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, groups,
				[]string{"id", "account_no", "name", "parent", "active", "locked"},
				func(g api.AccountGroup) json.RawMessage { return g.Raw },
				func(g api.AccountGroup) []string {
					parent := ""
					if g.ParentFibuAccountGroupID > 0 {
						parent = strconv.Itoa(g.ParentFibuAccountGroupID)
					}
					return []string{
						strconv.Itoa(g.ID),
						g.AccountNo,
						output.Truncate(g.Name, 40),
						parent,
						strconv.FormatBool(g.IsActive),
						strconv.FormatBool(g.IsLocked),
					}
				})
		},
	}
	lookupFlags30(list, &opts)
	cmd.AddCommand(list)
	return cmd
}

var businessYearDetailOrder = []string{"id", "start", "end", "status", "closed_at"}

// newBusinessYearCmd exposes the business years (read-only).
func newBusinessYearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "business-year",
		Aliases: []string{"business-years"},
		Short:   "List and view business years",
	}
	var opts api.ListOptions
	list := &cobra.Command{
		Use:     "list",
		Short:   "List business years",
		Example: `  bexio business-year list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			years, err := client.ListBusinessYears(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, years,
				[]string{"id", "start", "end", "status", "closed_at"},
				func(y api.BusinessYear) json.RawMessage { return y.Raw },
				func(y api.BusinessYear) []string {
					return []string{strconv.Itoa(y.ID), y.Start, y.End, y.Status, y.ClosedAt}
				})
		},
	}
	lookupFlags30(list, &opts)
	view := &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single business year",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("business year", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			y, err := client.GetBusinessYear(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, y.Raw, businessYearDetailOrder)
		},
	}
	cmd.AddCommand(list, view)
	return cmd
}

var calendarYearDetailOrder = []string{
	"id", "start", "end", "is_vat_subject", "is_annual_reporting",
	"vat_accounting_method", "vat_accounting_type", "created_at", "updated_at",
}

func renderCalendarYears(cmd *cobra.Command, years []api.CalendarYear) error {
	return renderLookupList(cmd, years,
		[]string{"id", "start", "end", "vat_subject", "vat_method", "vat_type", "annual_reporting"},
		func(y api.CalendarYear) json.RawMessage { return y.Raw },
		func(y api.CalendarYear) []string {
			return []string{
				strconv.Itoa(y.ID), y.Start, y.End,
				strconv.FormatBool(y.IsVatSubject),
				y.VatAccountingMethod, y.VatAccountingType,
				strconv.FormatBool(y.IsAnnualReporting),
			}
		})
}

// newCalendarYearCmd exposes the calendar years (list/search/view/create).
func newCalendarYearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "calendar-year",
		Aliases: []string{"calendar-years"},
		Short:   "List, search, view, and create calendar years",
	}
	cmd.AddCommand(
		newCalendarYearListCmd(),
		newCalendarYearSearchCmd(),
		newCalendarYearViewCmd(),
		newCalendarYearCreateCmd(),
	)
	return cmd
}

func newCalendarYearListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List calendar years",
		Example: `  bexio calendar-year list -o json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			years, err := client.ListCalendarYears(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderCalendarYears(cmd, years)
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newCalendarYearSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search calendar years",
		Long: `Search calendar years with --where clauses (raw API field names, ANDed).
Searchable fields include: id, start, end, is_vat_subject,
vat_accounting_method, vat_accounting_type.`,
		Example: `  bexio calendar-year search --where "start>=2025-01-01"
  bexio calendar-year search --where is_vat_subject=true`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: pass at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			years, err := client.SearchCalendarYears(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderCalendarYears(cmd, years)
		},
	}
	lookupFlags30(cmd, &opts)
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newCalendarYearViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single calendar year",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("calendar year", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			y, err := client.GetCalendarYear(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, y.Raw, calendarYearDetailOrder)
		},
	}
}

func newCalendarYearCreateCmd() *cobra.Command {
	var (
		year                                    string
		isVatSubject, isAnnualReporting         bool
		vatAccountingMethod, vatAccountingType  string
		defaultTaxIncomeID, defaultTaxExpenseID int
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a calendar year",
		Long: `Create a calendar year. Years from 2016 up to 10 years ahead are allowed;
creating a future year also generates every year in between (the API
returns all created years).`,
		Example: `  bexio calendar-year create --year 2027 --is-vat-subject \
      --vat-accounting-method effective --vat-accounting-type agreed \
      --default-tax-income-id 1 --default-tax-expense-id 2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if year == "" {
				return fmt.Errorf("--year is required")
			}
			fields := map[string]any{"year": year}
			setIfChanged(cmd, fields, "is-vat-subject", "is_vat_subject", isVatSubject)
			setIfChanged(cmd, fields, "is-annual-reporting", "is_annual_reporting", isAnnualReporting)
			setIfChanged(cmd, fields, "vat-accounting-method", "vat_accounting_method", vatAccountingMethod)
			setIfChanged(cmd, fields, "vat-accounting-type", "vat_accounting_type", vatAccountingType)
			setIfChanged(cmd, fields, "default-tax-income-id", "default_tax_income_id", defaultTaxIncomeID)
			setIfChanged(cmd, fields, "default-tax-expense-id", "default_tax_expense_id", defaultTaxExpenseID)
			client, err := newClient()
			if err != nil {
				return err
			}
			years, err := client.CreateCalendarYear(cmd.Context(), fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return renderCalendarYears(cmd, years)
			}
			for _, y := range years {
				fmt.Fprintf(cmd.OutOrStdout(), "Created calendar year %d (%s – %s)\n", y.ID, y.Start, y.End)
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&year, "year", "", "year to create, e.g. 2027 (required)")
	fl.BoolVar(&isVatSubject, "is-vat-subject", false, "the calendar year is subject to VAT")
	fl.BoolVar(&isAnnualReporting, "is-annual-reporting", false, "enable annual reporting for the calendar year")
	fl.StringVar(&vatAccountingMethod, "vat-accounting-method", "", "VAT accounting method: effective or net_tax")
	fl.StringVar(&vatAccountingType, "vat-accounting-type", "", "VAT accounting type: agreed or collected")
	fl.IntVar(&defaultTaxIncomeID, "default-tax-income-id", 0, "default tax id for income")
	fl.IntVar(&defaultTaxExpenseID, "default-tax-expense-id", 0, "default tax id for expense")
	return cmd
}

var vatPeriodDetailOrder = []string{"id", "start", "end", "type", "status", "closed_at"}

// newVatPeriodCmd exposes the VAT periods (read-only).
func newVatPeriodCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "vat-period",
		Aliases: []string{"vat-periods"},
		Short:   "List and view VAT periods",
	}
	var opts api.ListOptions
	list := &cobra.Command{
		Use:     "list",
		Short:   "List VAT periods",
		Example: `  bexio vat-period list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			periods, err := client.ListVatPeriods(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, periods,
				[]string{"id", "start", "end", "type", "status", "closed_at"},
				func(p api.VatPeriod) json.RawMessage { return p.Raw },
				func(p api.VatPeriod) []string {
					return []string{strconv.Itoa(p.ID), p.Start, p.End, p.Type, p.Status, p.ClosedAt}
				})
		},
	}
	lookupFlags30(list, &opts)
	view := &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single VAT period",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("vat period", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetVatPeriod(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, vatPeriodDetailOrder)
		},
	}
	cmd.AddCommand(list, view)
	return cmd
}

// newJournalCmd reads the accounting journal report.
func newJournalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Read the accounting journal report",
	}
	var opts api.JournalOptions
	list := &cobra.Command{
		Use:   "list",
		Short: "List journal entries",
		Long: `List the entries of the accounting journal (GET /3.0/accounting/journal).
--from/--to filter by booking date, --account-uuid by account (see
"bexio account list -o json" for the uuid).`,
		Example: `  bexio journal list --from 2026-01-01 --to 2026-03-31
  bexio journal list --account-uuid 474cc93a-2d6f-47e9-bd3f-a5b5a1941314 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			entries, err := client.ListJournalEntries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, entries,
				[]string{"id", "date", "debit", "credit", "amount", "currency_id", "ref_class", "description"},
				func(j api.JournalEntry) json.RawMessage { return j.Raw },
				func(j api.JournalEntry) []string {
					return []string{
						strconv.Itoa(j.ID),
						j.Date,
						strconv.Itoa(j.DebitAccountID),
						strconv.Itoa(j.CreditAccountID),
						formatAmount(j.Amount),
						strconv.Itoa(j.CurrencyID),
						j.RefClass,
						output.Truncate(j.Description, 40),
					}
				})
		},
	}
	list.Flags().StringVar(&opts.From, "from", "", "only entries on or after this date (YYYY-MM-DD)")
	list.Flags().StringVar(&opts.To, "to", "", "only entries up to this date (YYYY-MM-DD)")
	list.Flags().StringVar(&opts.AccountUUID, "account-uuid", "", "only entries booked on the account with this uuid")
	lookupFlags30(list, &opts.ListOptions)
	cmd.AddCommand(list)
	return cmd
}
