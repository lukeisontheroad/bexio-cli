package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newQuoteCmd) }

// newQuoteCmd manages quotes/offers (API resource "kb_offer").
func newQuoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kb-offer",
		Aliases: []string{"quote", "offer"},
		Short:   "List, view, search, and modify quotes (offers)",
	}
	cmd.AddCommand(
		newQuoteListCmd(),
		newQuoteViewCmd(),
		newQuoteSearchCmd(),
		newQuoteCreateCmd(),
		newQuoteUpdateCmd(),
		newQuoteDeleteCmd(),
		newQuotePDFCmd(),
		newQuoteActionCmd("issue", "Issue the draft quote (draft → pending)", "Issued", (*api.Client).IssueQuote),
		newQuoteActionCmd("revert-issue", "Revert an issued quote back to draft", "Reverted issue of", (*api.Client).RevertIssueQuote),
		newQuoteActionCmd("accept", "Accept the pending quote (pending → confirmed)", "Accepted", (*api.Client).AcceptQuote),
		newQuoteActionCmd("reject", "Decline the pending quote (pending → declined)", "Rejected", (*api.Client).RejectQuote),
		newQuoteActionCmd("reissue", "Reissue the confirmed/declined quote (back to pending)", "Reissued", (*api.Client).ReissueQuote),
		newQuoteActionCmd("mark-as-sent", "Mark the quote as sent without emailing it", "Marked as sent", (*api.Client).MarkQuoteAsSent),
		newQuoteSendCmd(),
		newQuoteCopyCmd(),
		newQuoteOrderCmd(),
		newQuoteInvoiceCmd(),
		newQuotePositionCmd(),
	)
	return cmd
}

var quoteDetailOrder = []string{
	"id", "document_nr", "title", "kb_item_status_id", "contact_id",
	"contact_sub_id", "user_id", "pr_project_id", "language_id",
	"bank_account_id", "currency_id", "payment_type_id",
	"kb_terms_of_payment_template_id", "mwst_type", "mwst_is_net",
	"show_position_taxes", "is_valid_from", "is_valid_until",
	"contact_address", "delivery_address_type", "delivery_address",
	"total_net", "total_taxes", "total_gross", "total",
	"viewed_by_client_at", "network_link", "api_reference",
	"template_slug", "updated_at", "positions",
}

func renderQuotes(cmd *cobra.Command, quotes []api.Quote) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(quotes))
		for i, q := range quotes {
			raws[i] = q.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(quotes))
	for i, q := range quotes {
		rows[i] = []string{
			strconv.Itoa(q.ID),
			q.DocumentNr,
			q.StatusName(),
			output.Truncate(q.Title, 40),
			strconv.Itoa(q.ContactID),
			q.Total,
			shortDate(q.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "document_nr", "status", "title", "contact_id", "total", "updated"}, rows)
	return nil
}

// quoteFieldFlags mirrors the writable header fields of POST /2.0/kb_offer.
type quoteFieldFlags struct {
	documentNr, title         string
	contactID, contactSubID   int
	userID                    int
	prProjectID               int
	languageID                int
	bankAccountID             int
	currencyID                int
	paymentTypeID             int
	termsOfPaymentTemplateID  int
	header, footer            string
	mwstType                  int
	mwstIsNet                 bool
	showPositionTaxes         bool
	isValidFrom, isValidUntil string
	deliveryAddressType       int
	apiReference              string
	templateSlug              string
}

func (f *quoteFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.documentNr, "document-nr", "", "document number (only if automatic numbering is off)")
	fl.StringVar(&f.title, "title", "", "quote title")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id (required on create)")
	fl.IntVar(&f.contactSubID, "contact-sub-id", 0, "contact person id")
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "project id")
	fl.IntVar(&f.languageID, "language-id", 0, "language id")
	fl.IntVar(&f.bankAccountID, "bank-account-id", 0, "bank account id")
	fl.IntVar(&f.currencyID, "currency-id", 0, "currency id")
	fl.IntVar(&f.paymentTypeID, "payment-type-id", 0, "payment type id")
	fl.IntVar(&f.termsOfPaymentTemplateID, "kb-terms-of-payment-template-id", 0, "terms of payment template id")
	fl.StringVar(&f.header, "header", "", "document header text")
	fl.StringVar(&f.footer, "footer", "", "document footer text")
	fl.IntVar(&f.mwstType, "mwst-type", 0, "tax mode: 0 including, 1 excluding, 2 exempt")
	fl.BoolVar(&f.mwstIsNet, "mwst-is-net", false, "taxes shown additionally to a total including taxes (with --mwst-type 0)")
	fl.BoolVar(&f.showPositionTaxes, "show-position-taxes", false, "show taxes per position")
	fl.StringVar(&f.isValidFrom, "is-valid-from", "", "quote date (YYYY-MM-DD)")
	fl.StringVar(&f.isValidUntil, "is-valid-until", "", "quote validity end date (YYYY-MM-DD)")
	fl.IntVar(&f.deliveryAddressType, "delivery-address-type", 0, "delivery address: 0 invoice address, 1 custom address")
	fl.StringVar(&f.apiReference, "api-reference", "", "free-form API reference field (only visible via API)")
	fl.StringVar(&f.templateSlug, "template-slug", "", "document template slug")
}

func (f *quoteFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "document-nr", "document_nr", f.documentNr)
	setIfChanged(cmd, fields, "title", "title", f.title)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", f.contactSubID)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", f.prProjectID)
	setIfChanged(cmd, fields, "language-id", "language_id", f.languageID)
	setIfChanged(cmd, fields, "bank-account-id", "bank_account_id", f.bankAccountID)
	setIfChanged(cmd, fields, "currency-id", "currency_id", f.currencyID)
	setIfChanged(cmd, fields, "payment-type-id", "payment_type_id", f.paymentTypeID)
	setIfChanged(cmd, fields, "kb-terms-of-payment-template-id", "kb_terms_of_payment_template_id", f.termsOfPaymentTemplateID)
	setIfChanged(cmd, fields, "header", "header", f.header)
	setIfChanged(cmd, fields, "footer", "footer", f.footer)
	setIfChanged(cmd, fields, "mwst-type", "mwst_type", f.mwstType)
	setIfChanged(cmd, fields, "mwst-is-net", "mwst_is_net", f.mwstIsNet)
	setIfChanged(cmd, fields, "show-position-taxes", "show_position_taxes", f.showPositionTaxes)
	setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", f.isValidFrom)
	setIfChanged(cmd, fields, "is-valid-until", "is_valid_until", f.isValidUntil)
	setIfChanged(cmd, fields, "delivery-address-type", "delivery_address_type", f.deliveryAddressType)
	setIfChanged(cmd, fields, "api-reference", "api_reference", f.apiReference)
	setIfChanged(cmd, fields, "template-slug", "template_slug", f.templateSlug)
	return fields
}

func newQuoteListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List quotes",
		Example: `  bexio kb-offer list
  bexio kb-offer list --order-by updated_at_desc --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			quotes, err := client.ListQuotes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderQuotes(cmd, quotes)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	return cmd
}

func newQuoteViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a quote (including its positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			q, err := client.GetQuote(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, q.Raw, quoteDetailOrder)
		},
	}
}

func newQuoteSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search quotes",
		Long: `Search quotes. A bare term matches the title partially. --where clauses
use the raw API field names and add AND conditions (see
"bexio contact search --help" for the operator syntax).

Searchable fields: id, kb_item_status_id, document_nr, title, contact_id,
contact_sub_id, user_id, currency_id, total_gross, total_net, total,
is_valid_from, is_valid_to, is_valid_until, updated_at.
Status ids: 1 draft, 2 pending, 3 confirmed, 4 declined.`,
		Example: `  bexio kb-offer search --where contact_id=17
  bexio kb-offer search --where kb_item_status_id=2
  bexio kb-offer search --where "updated_at>2026-01-01" -o json`,
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
			quotes, err := client.SearchQuotes(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderQuotes(cmd, quotes)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newQuoteCreateCmd() *cobra.Command {
	var fields quoteFieldFlags
	var positions []string
	var positionsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a quote",
		Long: `Create a quote. --contact-id is required; user_id defaults to the
authenticated user. Positions are passed as repeatable --position specs
("type=..." plus raw API fields) or as raw JSON via --positions-json:

  type=article  article_id, amount, and optionally unit_price, tax_id, text
  type=custom   text, amount, unit_price, and optionally unit_id, tax_id
  type=text     text
  type=subtotal / discount / pagebreak`,
		Example: `  bexio kb-offer create --contact-id 17 --title "Website relaunch" \
      --position "type=custom,text=Consulting,amount=8,unit_price=150" \
      --position "type=article,article_id=5,amount=2"
  bexio kb-offer create --contact-id 17 --positions-json '[{"type":"KbPositionText","text":"Hi"}]'`,
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
			q, err := client.CreateQuote(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), q.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created quote %d (%s, total %s)\n", q.ID, q.DocumentNr, q.Total)
			return nil
		},
	}
	fields.register(cmd)
	cmd.Flags().StringArrayVar(&positions, "position", nil, `position spec, e.g. "type=article,article_id=5,amount=2" (repeatable)`)
	cmd.Flags().StringVar(&positionsJSON, "positions-json", "", "positions as a raw JSON array (advanced)")
	return cmd
}

func newQuoteUpdateCmd() *cobra.Command {
	var fields quoteFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update header fields of a quote",
		Long:  "Update a quote. Only the flags you pass are changed. Positions are managed with `bexio kb-offer position`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
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
			q, err := client.UpdateQuote(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), q.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated quote %d (%s)\n", q.ID, q.DocumentNr)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newQuoteDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a quote",
		Long:  "Permanently delete a quote. Unlike contacts this CANNOT be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a quote is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteQuote(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted quote %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newQuotePDFCmd() *cobra.Command {
	var out string
	var logopaper int
	cmd := &cobra.Command{
		Use:   "pdf <id>",
		Short: "Download the quote as PDF",
		Example: `  bexio kb-offer pdf 4
  bexio kb-offer pdf 4 --out quote.pdf --logopaper 1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			pdf, err := client.QuotePDF(cmd.Context(), id, logopaper)
			if err != nil {
				return err
			}
			data, err := base64.StdEncoding.DecodeString(pdf.Content)
			if err != nil {
				return fmt.Errorf("decode PDF content: %w", err)
			}
			path := out
			if path == "" {
				path = pdf.Name
			}
			if path == "" {
				path = fmt.Sprintf("quote-%d.pdf", id)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d bytes)\n", path, len(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: name from the API)")
	cmd.Flags().IntVar(&logopaper, "logopaper", -1, "1 = render with letterhead, 0 = without (default: server setting)")
	return cmd
}

// newQuoteActionCmd builds a bodyless status-transition subcommand (issue,
// revert-issue, accept, reject, reissue, mark-as-sent).
func newQuoteActionCmd(use, short, done string, action func(*api.Client, context.Context, int) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := action(client, cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s quote %d\n", done, id)
			return nil
		},
	}
}

func newQuoteSendCmd() *cobra.Command {
	var recipientEmail, subject, message string
	var markAsOpen, attachPDF bool
	cmd := &cobra.Command{
		Use:   "send <id>",
		Short: "Send the quote by email",
		Long: `Send the quote by email. --recipient-email, --subject, and --message are
required; the message must contain the "[Network Link]" placeholder.`,
		Example: `  bexio kb-offer send 4 --recipient-email anna@example.com \
      --subject "Your quote" --message "Please find the quote at [Network Link]" \
      --attach-pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			body := map[string]any{
				"recipient_email": recipientEmail,
				"subject":         subject,
				"message":         message,
			}
			setIfChanged(cmd, body, "mark-as-open", "mark_as_open", markAsOpen)
			setIfChanged(cmd, body, "attach-pdf", "attach_pdf", attachPDF)
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.SendQuote(cmd.Context(), id, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sent quote %d to %s\n", id, recipientEmail)
			return nil
		},
	}
	cmd.Flags().StringVar(&recipientEmail, "recipient-email", "", "recipient email address (required)")
	cmd.Flags().StringVar(&subject, "subject", "", "email subject (required)")
	cmd.Flags().StringVar(&message, "message", "", `email body; must contain "[Network Link]" (required)`)
	cmd.Flags().BoolVar(&markAsOpen, "mark-as-open", false, "mark the quote as open after sending")
	cmd.Flags().BoolVar(&attachPDF, "attach-pdf", false, "attach the PDF directly to the email")
	_ = cmd.MarkFlagRequired("recipient-email")
	_ = cmd.MarkFlagRequired("subject")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func newQuoteCopyCmd() *cobra.Command {
	var contactID, contactSubID, prProjectID int
	var title, isValidFrom string
	cmd := &cobra.Command{
		Use:   "copy <id>",
		Short: "Copy the quote to a new quote",
		Example: `  bexio kb-offer copy 4 --contact-id 17
  bexio kb-offer copy 4 --contact-id 17 --title "Second round" --is-valid-from 2026-09-01`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{"contact_id": contactID}
			setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", contactSubID)
			setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", prProjectID)
			setIfChanged(cmd, fields, "title", "title", title)
			setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", isValidFrom)
			client, err := newClient()
			if err != nil {
				return err
			}
			q, err := client.CopyQuote(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), q.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Copied quote %d to %d (%s)\n", id, q.ID, q.DocumentNr)
			return nil
		},
	}
	cmd.Flags().IntVar(&contactID, "contact-id", 0, "contact id of the new quote (required)")
	cmd.Flags().IntVar(&contactSubID, "contact-sub-id", 0, "contact person id")
	cmd.Flags().IntVar(&prProjectID, "pr-project-id", 0, "project id")
	cmd.Flags().StringVar(&title, "title", "", "title of the new quote")
	cmd.Flags().StringVar(&isValidFrom, "is-valid-from", "", "quote date of the new quote (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("contact-id")
	return cmd
}

func newQuoteOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "order <id>",
		Short: "Create a sales order from the quote (all positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CreateOrderFromQuote(cmd.Context(), id)
			if err != nil {
				return err
			}
			return reportCreatedDocument(cmd, raw, "order")
		},
	}
}

func newQuoteInvoiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "invoice <id>",
		Short: "Create an invoice from the quote (all positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CreateInvoiceFromQuote(cmd.Context(), id)
			if err != nil {
				return err
			}
			return reportCreatedDocument(cmd, raw, "invoice")
		},
	}
}

func newQuotePositionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "Add, update, or delete positions of a quote",
		Long: `Manage the positions of an existing quote. Position specs use the same
"type=...,field=value,..." syntax as "kb-offer create --position";
"bexio kb-offer view <id>" shows the current positions with their ids.`,
	}
	add := &cobra.Command{
		Use:   "add <quote-id> <spec>",
		Short: "Add a position",
		Example: `  bexio kb-offer position add 4 "type=article,article_id=5,amount=2"
  bexio kb-offer position add 4 "type=custom,text=Consulting,amount=8,unit_price=150"
  bexio kb-offer position add 4 "type=text,text=Valid for 30 days"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
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
			raw, err := client.CreatePosition(cmd.Context(), "kb_offer", id, posType, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	update := &cobra.Command{
		Use:   "update <quote-id> <position-id> <spec>",
		Short: "Update a position",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("quote", args[0])
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
			raw, err := client.UpdatePosition(cmd.Context(), "kb_offer", id, posType, posID, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	var delType string
	del := &cobra.Command{
		Use:   "delete <quote-id> <position-id>",
		Short: "Delete a position",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("quote", args[0])
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
			if err := client.DeletePosition(cmd.Context(), "kb_offer", id, delType, posID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted position %d from quote %d\n", posID, id)
			return nil
		},
	}
	del.Flags().StringVar(&delType, "type", "", "position type: article|custom|text|subtotal|discount|pagebreak|subposition (required)")
	_ = del.MarkFlagRequired("type")
	cmd.AddCommand(add, update, del)
	return cmd
}
