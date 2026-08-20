package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newInvoiceCmd) }

// newInvoiceCmd manages customer invoices (API resource "kb_invoice").
func newInvoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kb-invoice",
		Aliases: []string{"invoice"},
		Short:   "List, view, search, and modify customer invoices",
	}
	cmd.AddCommand(
		newInvoiceListCmd(),
		newInvoiceViewCmd(),
		newInvoiceSearchCmd(),
		newInvoiceCreateCmd(),
		newInvoiceUpdateCmd(),
		newInvoiceDeleteCmd(),
		newInvoicePDFCmd(),
		newInvoiceCopyCmd(),
		newInvoiceIssueCmd(),
		newInvoiceRevertIssueCmd(),
		newInvoiceCancelCmd(),
		newInvoiceMarkAsSentCmd(),
		newInvoiceSendCmd(),
		newInvoicePaymentCmd(),
		newInvoiceReminderCmd(),
		newInvoicePositionCmd(),
	)
	return cmd
}

var invoiceDetailOrder = []string{
	"id", "document_nr", "title", "kb_item_status_id", "contact_id",
	"contact_sub_id", "user_id", "pr_project_id", "project_id",
	"language_id", "bank_account_id", "currency_id", "payment_type_id",
	"mwst_type", "mwst_is_net", "show_position_taxes", "is_valid_from",
	"is_valid_to", "contact_address", "contact_address_manual", "reference",
	"total_net", "total_taxes", "total_gross", "total_received_payments",
	"total_credit_vouchers", "total_remaining_payments", "total",
	"api_reference", "template_slug", "esr_id", "qr_invoice_id",
	"network_link", "updated_at", "positions",
}

func renderInvoices(cmd *cobra.Command, invoices []api.Invoice) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(invoices))
		for i, inv := range invoices {
			raws[i] = inv.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(invoices))
	for i, inv := range invoices {
		rows[i] = []string{
			strconv.Itoa(inv.ID),
			inv.DocumentNr,
			inv.StatusName(),
			output.Truncate(inv.Title, 40),
			strconv.Itoa(inv.ContactID),
			inv.Total,
			inv.TotalRemainingPayments,
			shortDate(inv.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "document_nr", "status", "title", "contact_id", "total", "remaining", "updated"}, rows)
	return nil
}

// invoiceFieldFlags mirrors the writable header fields of
// POST /2.0/kb_invoice.
type invoiceFieldFlags struct {
	documentNr, title       string
	contactID, contactSubID int
	userID                  int
	prProjectID             int
	logopaperID             int
	languageID              int
	bankAccountID           int
	currencyID              int
	paymentTypeID           int
	header, footer          string
	mwstType                int
	mwstIsNet               bool
	showPositionTaxes       bool
	isValidFrom             string
	isValidTo               string
	contactAddressManual    string
	reference               string
	apiReference            string
	templateSlug            string
}

func (f *invoiceFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.documentNr, "document-nr", "", "document number (only if automatic numbering is off)")
	fl.StringVar(&f.title, "title", "", "invoice title")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id (required on create)")
	fl.IntVar(&f.contactSubID, "contact-sub-id", 0, "contact person id")
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "project id")
	fl.IntVar(&f.logopaperID, "logopaper-id", 0, "logopaper id")
	fl.IntVar(&f.languageID, "language-id", 0, "language id")
	fl.IntVar(&f.bankAccountID, "bank-account-id", 0, "bank account id")
	fl.IntVar(&f.currencyID, "currency-id", 0, "currency id")
	fl.IntVar(&f.paymentTypeID, "payment-type-id", 0, "payment type id")
	fl.StringVar(&f.header, "header", "", "document header text")
	fl.StringVar(&f.footer, "footer", "", "document footer text")
	fl.IntVar(&f.mwstType, "mwst-type", 0, "tax mode: 0 including, 1 excluding, 2 exempt")
	fl.BoolVar(&f.mwstIsNet, "mwst-is-net", false, "taxes shown additionally to a total including taxes (with --mwst-type 0)")
	fl.BoolVar(&f.showPositionTaxes, "show-position-taxes", false, "show taxes per position")
	fl.StringVar(&f.isValidFrom, "is-valid-from", "", "invoice date (YYYY-MM-DD)")
	fl.StringVar(&f.isValidTo, "is-valid-to", "", "due date (YYYY-MM-DD)")
	fl.StringVar(&f.contactAddressManual, "contact-address-manual", "", "manual invoice address (default: address of the contact)")
	fl.StringVar(&f.reference, "reference", "", "reference shown on the document")
	fl.StringVar(&f.apiReference, "api-reference", "", "free-form API reference field (only visible via API)")
	fl.StringVar(&f.templateSlug, "template-slug", "", "document template slug")
}

func (f *invoiceFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "document-nr", "document_nr", f.documentNr)
	setIfChanged(cmd, fields, "title", "title", f.title)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", f.contactSubID)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", f.prProjectID)
	setIfChanged(cmd, fields, "logopaper-id", "logopaper_id", f.logopaperID)
	setIfChanged(cmd, fields, "language-id", "language_id", f.languageID)
	setIfChanged(cmd, fields, "bank-account-id", "bank_account_id", f.bankAccountID)
	setIfChanged(cmd, fields, "currency-id", "currency_id", f.currencyID)
	setIfChanged(cmd, fields, "payment-type-id", "payment_type_id", f.paymentTypeID)
	setIfChanged(cmd, fields, "header", "header", f.header)
	setIfChanged(cmd, fields, "footer", "footer", f.footer)
	setIfChanged(cmd, fields, "mwst-type", "mwst_type", f.mwstType)
	setIfChanged(cmd, fields, "mwst-is-net", "mwst_is_net", f.mwstIsNet)
	setIfChanged(cmd, fields, "show-position-taxes", "show_position_taxes", f.showPositionTaxes)
	setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", f.isValidFrom)
	setIfChanged(cmd, fields, "is-valid-to", "is_valid_to", f.isValidTo)
	setIfChanged(cmd, fields, "contact-address-manual", "contact_address_manual", f.contactAddressManual)
	setIfChanged(cmd, fields, "reference", "reference", f.reference)
	setIfChanged(cmd, fields, "api-reference", "api_reference", f.apiReference)
	setIfChanged(cmd, fields, "template-slug", "template_slug", f.templateSlug)
	return fields
}

// writeInvoicePDF decodes a PDF response and writes it to out (or the
// API-provided name, or fallback).
func writeInvoicePDF(cmd *cobra.Command, pdf *api.PDF, out, fallback string) error {
	data, err := base64.StdEncoding.DecodeString(pdf.Content)
	if err != nil {
		return fmt.Errorf("decode PDF content: %w", err)
	}
	path := out
	if path == "" {
		path = pdf.Name
	}
	if path == "" {
		path = fallback
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d bytes)\n", path, len(data))
	return nil
}

func newInvoiceListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices",
		Example: `  bexio kb-invoice list
  bexio kb-invoice list --order-by updated_at_desc --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			invoices, err := client.ListInvoices(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderInvoices(cmd, invoices)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	return cmd
}

func newInvoiceViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show an invoice (including its positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			inv, err := client.GetInvoice(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, inv.Raw, invoiceDetailOrder)
		},
	}
}

func newInvoiceSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search invoices",
		Long: `Search invoices. A bare term matches the title partially. --where clauses
use the raw API field names and add AND conditions (see
"bexio contact search --help" for the operator syntax).

Searchable fields: id, kb_item_status_id, document_nr, title,
api_reference, contact_id, contact_sub_id, user_id, currency_id,
total_gross, total_net, total, is_valid_from, is_valid_to, updated_at.
Status ids: 7 draft, 8 pending, 9 paid, 16 partial, 19 canceled, 31 unpaid.`,
		Example: `  bexio kb-invoice search --where contact_id=17
  bexio kb-invoice search --where kb_item_status_id=8
  bexio kb-invoice search --where "is_valid_to<2026-01-01" -o json`,
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
					Field: "title", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			invoices, err := client.SearchInvoices(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderInvoices(cmd, invoices)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newInvoiceCreateCmd() *cobra.Command {
	var fields invoiceFieldFlags
	var positions []string
	var positionsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an invoice (as draft)",
		Long: `Create an invoice. --contact-id is required; user_id defaults to the
authenticated user. The invoice starts as a draft — use "bexio kb-invoice
issue" to make it pending. Positions are passed as repeatable --position
specs ("type=..." plus raw API fields) or as raw JSON via --positions-json:

  type=article  article_id, amount, and optionally unit_price, tax_id, text
  type=custom   text, amount, unit_price, and optionally unit_id, tax_id
  type=text     text
  type=subtotal / discount / pagebreak`,
		Example: `  bexio kb-invoice create --contact-id 17 --title "Website relaunch" \
      --position "type=custom,text=Consulting,amount=8,unit_price=150" \
      --position "type=article,article_id=5,amount=2"
  bexio kb-invoice create --contact-id 17 --positions-json '[{"type":"KbPositionText","text":"Hi"}]'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["contact_id"] == nil {
				return fmt.Errorf("--contact-id is required")
			}

			var posList []any
			for _, spec := range positions {
				posType, posFields, err := parsePositionSpec(spec)
				if err != nil {
					return err
				}
				posFields["type"] = positionTypeNames[posType]
				posList = append(posList, posFields)
			}
			if positionsJSON != "" {
				if len(posList) > 0 {
					return fmt.Errorf("--position and --positions-json are mutually exclusive")
				}
				var raw []any
				if err := json.Unmarshal([]byte(positionsJSON), &raw); err != nil {
					return fmt.Errorf("--positions-json is not a valid JSON array: %w", err)
				}
				posList = raw
			}
			if len(posList) > 0 {
				payload["positions"] = posList
			}

			client, err := newClient()
			if err != nil {
				return err
			}
			if payload["user_id"] == nil {
				me, err := client.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolve default user_id: %w", err)
				}
				payload["user_id"] = me.ID
			}
			inv, err := client.CreateInvoice(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), inv.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created invoice %d (%s, total %s)\n", inv.ID, inv.DocumentNr, inv.Total)
			return nil
		},
	}
	fields.register(cmd)
	cmd.Flags().StringArrayVar(&positions, "position", nil, `position spec, e.g. "type=article,article_id=5,amount=2" (repeatable)`)
	cmd.Flags().StringVar(&positionsJSON, "positions-json", "", "positions as a raw JSON array (advanced)")
	return cmd
}

func newInvoiceUpdateCmd() *cobra.Command {
	var fields invoiceFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update header fields of an invoice",
		Long:  "Update an invoice. Only the flags you pass are changed. Positions are managed with `bexio kb-invoice position`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			inv, err := client.UpdateInvoice(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), inv.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated invoice %d (%s)\n", inv.ID, inv.DocumentNr)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newInvoiceDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete an invoice",
		Long:  "Permanently delete an invoice. Unlike contacts this CANNOT be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting an invoice is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteInvoice(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted invoice %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newInvoicePDFCmd() *cobra.Command {
	var out string
	var logopaper int
	cmd := &cobra.Command{
		Use:   "pdf <id>",
		Short: "Download the invoice as PDF",
		Example: `  bexio kb-invoice pdf 4
  bexio kb-invoice pdf 4 --out invoice.pdf --logopaper 1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			pdf, err := client.InvoicePDF(cmd.Context(), id, logopaper)
			if err != nil {
				return err
			}
			return writeInvoicePDF(cmd, pdf, out, fmt.Sprintf("invoice-%d.pdf", id))
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: name from the API)")
	cmd.Flags().IntVar(&logopaper, "logopaper", -1, "1 = render with letterhead, 0 = without (default: server setting)")
	return cmd
}

func newInvoiceCopyCmd() *cobra.Command {
	var contactID, contactSubID int
	var title, isValidFrom string
	cmd := &cobra.Command{
		Use:   "copy <id>",
		Short: "Copy the invoice to a new draft",
		Example: `  bexio kb-invoice copy 4 --contact-id 17
  bexio kb-invoice copy 4 --contact-id 17 --title "Copy" --is-valid-from 2026-09-01`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{"contact_id": contactID}
			setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", contactSubID)
			setIfChanged(cmd, fields, "title", "title", title)
			setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", isValidFrom)
			client, err := newClient()
			if err != nil {
				return err
			}
			inv, err := client.CopyInvoice(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), inv.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created invoice %d (%s) as copy of %d\n", inv.ID, inv.DocumentNr, id)
			return nil
		},
	}
	cmd.Flags().IntVar(&contactID, "contact-id", 0, "contact id of the copy (required)")
	cmd.Flags().IntVar(&contactSubID, "contact-sub-id", 0, "contact person id of the copy")
	cmd.Flags().StringVar(&title, "title", "", "title of the copy")
	cmd.Flags().StringVar(&isValidFrom, "is-valid-from", "", "invoice date of the copy (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("contact-id")
	return cmd
}

// newInvoiceActionCmd builds a bodyless status-change subcommand.
func newInvoiceActionCmd(use, short, done string, call func(*api.Client, *cobra.Command, int) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := call(client, cmd, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), done+"\n", id)
			return nil
		},
	}
}

func newInvoiceIssueCmd() *cobra.Command {
	return newInvoiceActionCmd("issue", "Issue a draft invoice (draft -> pending)", "Issued invoice %d",
		func(c *api.Client, cmd *cobra.Command, id int) error { return c.IssueInvoice(cmd.Context(), id) })
}

func newInvoiceRevertIssueCmd() *cobra.Command {
	return newInvoiceActionCmd("revert-issue", "Set an issued invoice back to draft", "Reverted invoice %d to draft",
		func(c *api.Client, cmd *cobra.Command, id int) error { return c.RevertIssueInvoice(cmd.Context(), id) })
}

func newInvoiceCancelCmd() *cobra.Command {
	return newInvoiceActionCmd("cancel", "Cancel an invoice", "Canceled invoice %d",
		func(c *api.Client, cmd *cobra.Command, id int) error { return c.CancelInvoice(cmd.Context(), id) })
}

func newInvoiceMarkAsSentCmd() *cobra.Command {
	return newInvoiceActionCmd("mark-as-sent", "Mark an invoice as sent", "Marked invoice %d as sent",
		func(c *api.Client, cmd *cobra.Command, id int) error { return c.MarkInvoiceAsSent(cmd.Context(), id) })
}

func newInvoiceSendCmd() *cobra.Command {
	var recipient, subject, message string
	var markAsOpen, attachPDF bool
	cmd := &cobra.Command{
		Use:   "send <id>",
		Short: "Send the invoice by email",
		Long: `Send the invoice by email through bexio. The message must contain the
placeholder "[Network Link]" (replaced with the customer link to the
invoice).`,
		Example: `  bexio kb-invoice send 4 --recipient-email anna@example.com \
      --subject "Invoice RE-00004" --message "Hello, please find your invoice here: [Network Link]" \
      --attach-pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			body := map[string]any{
				"recipient_email": recipient,
				"subject":         subject,
				"message":         message,
			}
			setIfChanged(cmd, body, "mark-as-open", "mark_as_open", markAsOpen)
			setIfChanged(cmd, body, "attach-pdf", "attach_pdf", attachPDF)
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.SendInvoice(cmd.Context(), id, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sent invoice %d to %s\n", id, recipient)
			return nil
		},
	}
	cmd.Flags().StringVar(&recipient, "recipient-email", "", "recipient email address (required)")
	cmd.Flags().StringVar(&subject, "subject", "", "email subject (required)")
	cmd.Flags().StringVar(&message, "message", "", `email body; must contain "[Network Link]" (required)`)
	cmd.Flags().BoolVar(&markAsOpen, "mark-as-open", false, "mark the invoice as open after sending")
	cmd.Flags().BoolVar(&attachPDF, "attach-pdf", false, "attach the invoice PDF to the email")
	_ = cmd.MarkFlagRequired("recipient-email")
	_ = cmd.MarkFlagRequired("subject")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func renderInvoicePayments(cmd *cobra.Command, payments []api.Payment) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(payments))
		for i, p := range payments {
			raws[i] = p.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(payments))
	for i, p := range payments {
		rows[i] = []string{
			strconv.Itoa(p.ID),
			shortDate(p.Date),
			p.Value,
			strconv.Itoa(p.BankAccountID),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "date", "value", "bank_account_id"}, rows)
	return nil
}

func newInvoicePaymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payment",
		Short: "List, record, and delete payments of an invoice",
	}

	var listOpts api.ListOptions
	list := &cobra.Command{
		Use:   "list <invoice-id>",
		Short: "List the payments of an invoice",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			payments, err := client.ListInvoicePayments(cmd.Context(), id, listOpts)
			if err != nil {
				return err
			}
			return renderInvoicePayments(cmd, payments)
		},
	}
	listFlags(list, &listOpts, "id, date, value")

	view := &cobra.Command{
		Use:   "view <invoice-id> <payment-id>",
		Short: "Show a payment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			payID, err := parseID("payment", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetInvoicePayment(cmd.Context(), id, payID)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, []string{
				"id", "date", "value", "bank_account_id", "payment_service_id",
				"title", "kb_invoice_id",
			})
		},
	}

	var value, date string
	var bankAccountID, paymentServiceID int
	create := &cobra.Command{
		Use:   "create <invoice-id>",
		Short: "Record a payment on an invoice",
		Example: `  bexio kb-invoice payment create 4 --value 150.00
  bexio kb-invoice payment create 4 --value 150.00 --date 2026-08-21 --bank-account-id 1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{"value": value}
			setIfChanged(cmd, fields, "date", "date", date)
			setIfChanged(cmd, fields, "bank-account-id", "bank_account_id", bankAccountID)
			setIfChanged(cmd, fields, "payment-service-id", "payment_service_id", paymentServiceID)
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.CreateInvoicePayment(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded payment %d of %s on invoice %d\n", p.ID, p.Value, id)
			return nil
		},
	}
	create.Flags().StringVar(&value, "value", "", "payment amount (required)")
	create.Flags().StringVar(&date, "date", "", "payment date (YYYY-MM-DD)")
	create.Flags().IntVar(&bankAccountID, "bank-account-id", 0, "bank account id")
	create.Flags().IntVar(&paymentServiceID, "payment-service-id", 0, "payment service: 1 PayPal, 2 Stripe, 3 SIX Payments")
	_ = create.MarkFlagRequired("value")

	del := &cobra.Command{
		Use:   "delete <invoice-id> <payment-id>",
		Short: "Delete a payment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			payID, err := parseID("payment", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteInvoicePayment(cmd.Context(), id, payID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted payment %d from invoice %d\n", payID, id)
			return nil
		},
	}

	cmd.AddCommand(list, view, create, del)
	return cmd
}

var invoiceReminderDetailOrder = []string{
	"id", "kb_invoice_id", "title", "reminder_level", "is_sent",
	"is_valid_from", "is_valid_to", "reminder_period_in_days",
	"show_positions", "remaining_price", "received_total", "header",
	"footer",
}

func renderInvoiceReminders(cmd *cobra.Command, reminders []api.Reminder) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(reminders))
		for i, r := range reminders {
			raws[i] = r.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(reminders))
	for i, r := range reminders {
		rows[i] = []string{
			strconv.Itoa(r.ID),
			strconv.Itoa(r.ReminderLevel),
			output.Truncate(r.Title, 40),
			strconv.FormatBool(r.IsSent),
			shortDate(r.IsValidFrom),
			shortDate(r.IsValidTo),
			r.RemainingPrice,
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "level", "title", "sent", "valid_from", "valid_to", "remaining"}, rows)
	return nil
}

func newInvoiceReminderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reminder",
		Short: "Manage payment reminders of an invoice",
	}

	var listOpts api.ListOptions
	list := &cobra.Command{
		Use:   "list <invoice-id>",
		Short: "List the reminders of an invoice",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			reminders, err := client.ListInvoiceReminders(cmd.Context(), id, listOpts)
			if err != nil {
				return err
			}
			return renderInvoiceReminders(cmd, reminders)
		},
	}
	listFlags(list, &listOpts, "id, is_valid_from, is_valid_to")

	var searchOpts api.ListOptions
	var where []string
	search := &cobra.Command{
		Use:   "search <invoice-id>",
		Short: "Search the reminders of an invoice",
		Long: `Search the reminders of an invoice with --where clauses (see
"bexio contact search --help" for the operator syntax).

Searchable fields: title, reminder_level, is_sent, is_valid_from,
is_valid_to.`,
		Example: `  bexio kb-invoice reminder search 4 --where is_sent=false
  bexio kb-invoice reminder search 4 --where reminder_level=2 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
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
			reminders, err := client.SearchInvoiceReminders(cmd.Context(), id, criteria, searchOpts)
			if err != nil {
				return err
			}
			return renderInvoiceReminders(cmd, reminders)
		},
	}
	listFlags(search, &searchOpts, "id, is_valid_from, is_valid_to")
	search.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")

	view := &cobra.Command{
		Use:   "view <invoice-id> <reminder-id>",
		Short: "Show a reminder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			remID, err := parseID("reminder", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.GetInvoiceReminder(cmd.Context(), id, remID)
			if err != nil {
				return err
			}
			return renderDetail(cmd, r.Raw, invoiceReminderDetailOrder)
		},
	}

	var title, isValidFrom, isValidTo, header, footer string
	var periodInDays int
	var showPositions bool
	create := &cobra.Command{
		Use:   "create <invoice-id>",
		Short: "Create the next payment reminder",
		Long: `Create the next payment reminder for an overdue invoice. The API picks
the next reminder level automatically; all flags are optional.`,
		Example: `  bexio kb-invoice reminder create 4
  bexio kb-invoice reminder create 4 --title "2nd reminder" --reminder-period-in-days 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{}
			setIfChanged(cmd, fields, "title", "title", title)
			setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", isValidFrom)
			setIfChanged(cmd, fields, "is-valid-to", "is_valid_to", isValidTo)
			setIfChanged(cmd, fields, "reminder-period-in-days", "reminder_period_in_days", periodInDays)
			setIfChanged(cmd, fields, "show-positions", "show_positions", showPositions)
			setIfChanged(cmd, fields, "header", "header", header)
			setIfChanged(cmd, fields, "footer", "footer", footer)
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.CreateInvoiceReminder(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), r.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created reminder %d (level %d) for invoice %d\n", r.ID, r.ReminderLevel, id)
			return nil
		},
	}
	create.Flags().StringVar(&title, "title", "", "reminder title")
	create.Flags().StringVar(&isValidFrom, "is-valid-from", "", "reminder date (YYYY-MM-DD)")
	create.Flags().StringVar(&isValidTo, "is-valid-to", "", "new due date (YYYY-MM-DD)")
	create.Flags().IntVar(&periodInDays, "reminder-period-in-days", 0, "payment period of the reminder in days")
	create.Flags().BoolVar(&showPositions, "show-positions", false, "show the invoice positions on the reminder")
	create.Flags().StringVar(&header, "header", "", "document header text")
	create.Flags().StringVar(&footer, "footer", "", "document footer text")

	del := &cobra.Command{
		Use:   "delete <invoice-id> <reminder-id>",
		Short: "Delete a reminder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			remID, err := parseID("reminder", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteInvoiceReminder(cmd.Context(), id, remID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted reminder %d from invoice %d\n", remID, id)
			return nil
		},
	}

	markSent := newInvoiceReminderActionCmd("mark-as-sent", "Mark a reminder as sent", "Marked reminder %d of invoice %d as sent",
		func(c *api.Client, cmd *cobra.Command, id, remID int) error {
			return c.MarkInvoiceReminderAsSent(cmd.Context(), id, remID)
		})
	markUnsent := newInvoiceReminderActionCmd("mark-as-unsent", "Mark a reminder as unsent", "Marked reminder %d of invoice %d as unsent",
		func(c *api.Client, cmd *cobra.Command, id, remID int) error {
			return c.MarkInvoiceReminderAsUnsent(cmd.Context(), id, remID)
		})

	var recipient, subject, message string
	send := &cobra.Command{
		Use:   "send <invoice-id> <reminder-id>",
		Short: "Send the reminder by email",
		Long: `Send the reminder by email through bexio. The message must contain the
placeholder "[Network Link]".`,
		Example: `  bexio kb-invoice reminder send 4 2 --recipient-email anna@example.com \
      --subject "Payment reminder" --message "Please pay: [Network Link]"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			remID, err := parseID("reminder", args[1])
			if err != nil {
				return err
			}
			body := map[string]any{
				"recipient_email": recipient,
				"subject":         subject,
				"message":         message,
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.SendInvoiceReminder(cmd.Context(), id, remID, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sent reminder %d of invoice %d to %s\n", remID, id, recipient)
			return nil
		},
	}
	send.Flags().StringVar(&recipient, "recipient-email", "", "recipient email address (required)")
	send.Flags().StringVar(&subject, "subject", "", "email subject (required)")
	send.Flags().StringVar(&message, "message", "", `email body; must contain "[Network Link]" (required)`)
	_ = send.MarkFlagRequired("recipient-email")
	_ = send.MarkFlagRequired("subject")
	_ = send.MarkFlagRequired("message")

	var pdfOut string
	var logopaper int
	pdf := &cobra.Command{
		Use:   "pdf <invoice-id> <reminder-id>",
		Short: "Download the reminder as PDF",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			remID, err := parseID("reminder", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			doc, err := client.InvoiceReminderPDF(cmd.Context(), id, remID, logopaper)
			if err != nil {
				return err
			}
			return writeInvoicePDF(cmd, doc, pdfOut, fmt.Sprintf("invoice-%d-reminder-%d.pdf", id, remID))
		},
	}
	pdf.Flags().StringVar(&pdfOut, "out", "", "output file (default: name from the API)")
	pdf.Flags().IntVar(&logopaper, "logopaper", -1, "1 = render with letterhead, 0 = without (default: server setting)")

	cmd.AddCommand(list, search, view, create, del, markSent, markUnsent, send, pdf)
	return cmd
}

// newInvoiceReminderActionCmd builds a bodyless reminder status subcommand.
func newInvoiceReminderActionCmd(use, short, done string, call func(*api.Client, *cobra.Command, int, int) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <invoice-id> <reminder-id>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			remID, err := parseID("reminder", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := call(client, cmd, id, remID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), done+"\n", remID, id)
			return nil
		},
	}
}

func newInvoicePositionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "Add, update, or delete positions of an invoice",
		Long: `Manage the positions of an existing invoice. Position specs use the
same "type=...,field=value,..." syntax as "kb-invoice create --position";
"bexio kb-invoice view <id>" shows the current positions with their ids.`,
	}
	add := &cobra.Command{
		Use:   "add <invoice-id> <spec>",
		Short: "Add a position",
		Example: `  bexio kb-invoice position add 4 "type=article,article_id=5,amount=2"
  bexio kb-invoice position add 4 "type=custom,text=Consulting,amount=8,unit_price=150"
  bexio kb-invoice position add 4 "type=text,text=Payable within 30 days"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			posType, fields, err := parsePositionSpec(args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CreatePosition(cmd.Context(), "kb_invoice", id, posType, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	update := &cobra.Command{
		Use:   "update <invoice-id> <position-id> <spec>",
		Short: "Update a position",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			posID, err := parseID("position", args[1])
			if err != nil {
				return err
			}
			posType, fields, err := parsePositionSpec(args[2])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.UpdatePosition(cmd.Context(), "kb_invoice", id, posType, posID, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	var delType string
	del := &cobra.Command{
		Use:   "delete <invoice-id> <position-id>",
		Short: "Delete a position",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("invoice", args[0])
			if err != nil {
				return err
			}
			posID, err := parseID("position", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeletePosition(cmd.Context(), "kb_invoice", id, delType, posID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted position %d from invoice %d\n", posID, id)
			return nil
		},
	}
	del.Flags().StringVar(&delType, "type", "", "position type: article|custom|text|subtotal|discount|pagebreak|subposition (required)")
	_ = del.MarkFlagRequired("type")
	cmd.AddCommand(add, update, del)
	return cmd
}
