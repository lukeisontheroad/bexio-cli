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

func init() { registerModule(newOrderCmd) }

// newOrderCmd manages sales orders (API resource "kb_order").
func newOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kb-order",
		Aliases: []string{"order"},
		Short:   "List, view, search, and modify sales orders",
	}
	cmd.AddCommand(
		newOrderListCmd(),
		newOrderViewCmd(),
		newOrderSearchCmd(),
		newOrderCreateCmd(),
		newOrderUpdateCmd(),
		newOrderDeleteCmd(),
		newOrderPDFCmd(),
		newOrderInvoiceCmd(),
		newOrderDeliveryCmd(),
		newOrderRepetitionCmd(),
		newOrderPositionCmd(),
	)
	return cmd
}

var orderDetailOrder = []string{
	"id", "document_nr", "title", "kb_item_status_id", "contact_id",
	"contact_sub_id", "user_id", "pr_project_id", "language_id",
	"bank_account_id", "currency_id", "payment_type_id", "mwst_type",
	"mwst_is_net", "show_position_taxes", "is_valid_from",
	"contact_address", "delivery_address_type", "delivery_address",
	"total_net", "total_taxes", "total_gross", "total", "is_recurring",
	"api_reference", "template_slug", "updated_at", "positions",
}

func renderOrders(cmd *cobra.Command, orders []api.Order) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(orders))
		for i, o := range orders {
			raws[i] = o.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(orders))
	for i, o := range orders {
		rows[i] = []string{
			strconv.Itoa(o.ID),
			o.DocumentNr,
			o.StatusName(),
			output.Truncate(o.Title, 40),
			strconv.Itoa(o.ContactID),
			o.Total,
			shortDate(o.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "document_nr", "status", "title", "contact_id", "total", "updated"}, rows)
	return nil
}

// orderFieldFlags mirrors the writable header fields of POST /2.0/kb_order.
type orderFieldFlags struct {
	documentNr, title       string
	contactID, contactSubID int
	userID                  int
	prProjectID             int
	languageID              int
	bankAccountID           int
	currencyID              int
	paymentTypeID           int
	header, footer          string
	mwstType                int
	mwstIsNet               bool
	showPositionTaxes       bool
	isValidFrom             string
	deliveryAddressType     int
	apiReference            string
	templateSlug            string
}

func (f *orderFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.documentNr, "document-nr", "", "document number (only if automatic numbering is off)")
	fl.StringVar(&f.title, "title", "", "order title")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id (required on create)")
	fl.IntVar(&f.contactSubID, "contact-sub-id", 0, "contact person id")
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "project id")
	fl.IntVar(&f.languageID, "language-id", 0, "language id")
	fl.IntVar(&f.bankAccountID, "bank-account-id", 0, "bank account id")
	fl.IntVar(&f.currencyID, "currency-id", 0, "currency id")
	fl.IntVar(&f.paymentTypeID, "payment-type-id", 0, "payment type id")
	fl.StringVar(&f.header, "header", "", "document header text")
	fl.StringVar(&f.footer, "footer", "", "document footer text")
	fl.IntVar(&f.mwstType, "mwst-type", 0, "tax mode: 0 including, 1 excluding, 2 exempt")
	fl.BoolVar(&f.mwstIsNet, "mwst-is-net", false, "taxes shown additionally to a total including taxes (with --mwst-type 0)")
	fl.BoolVar(&f.showPositionTaxes, "show-position-taxes", false, "show taxes per position")
	fl.StringVar(&f.isValidFrom, "is-valid-from", "", "order date (YYYY-MM-DD)")
	fl.IntVar(&f.deliveryAddressType, "delivery-address-type", 0, "delivery address: 0 invoice address, 1 delivery address")
	fl.StringVar(&f.apiReference, "api-reference", "", "free-form API reference field (only visible via API)")
	fl.StringVar(&f.templateSlug, "template-slug", "", "document template slug")
}

func (f *orderFieldFlags) payload(cmd *cobra.Command) map[string]any {
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
	setIfChanged(cmd, fields, "header", "header", f.header)
	setIfChanged(cmd, fields, "footer", "footer", f.footer)
	setIfChanged(cmd, fields, "mwst-type", "mwst_type", f.mwstType)
	setIfChanged(cmd, fields, "mwst-is-net", "mwst_is_net", f.mwstIsNet)
	setIfChanged(cmd, fields, "show-position-taxes", "show_position_taxes", f.showPositionTaxes)
	setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", f.isValidFrom)
	setIfChanged(cmd, fields, "delivery-address-type", "delivery_address_type", f.deliveryAddressType)
	setIfChanged(cmd, fields, "api-reference", "api_reference", f.apiReference)
	setIfChanged(cmd, fields, "template-slug", "template_slug", f.templateSlug)
	return fields
}

// positionTypeNames maps the CLI type names to the API discriminator used in
// embedded position arrays.
var positionTypeNames = map[string]string{
	"article":     "KbPositionArticle",
	"custom":      "KbPositionCustom",
	"text":        "KbPositionText",
	"subtotal":    "KbPositionSubtotal",
	"discount":    "KbPositionDiscount",
	"pagebreak":   "KbPositionPagebreak",
	"subposition": "KbPositionSubposition",
}

var positionIntKeys = map[string]bool{"article_id": true, "unit_id": true, "tax_id": true, "account_id": true}
var positionBoolKeys = map[string]bool{"is_optional": true, "show_pos_nr": true, "is_percentual": true}

// parsePositionSpec parses "type=article,article_id=5,amount=2" into an API
// position object. The type key routes to the position kind; remaining keys
// are raw API field names.
func parsePositionSpec(spec string) (posType string, fields map[string]any, err error) {
	fields = map[string]any{}
	for _, part := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return "", nil, fmt.Errorf("invalid position spec %q: %q is not key=value", spec, part)
		}
		k = strings.TrimSpace(k)
		switch {
		case k == "type":
			posType = v
		case positionIntKeys[k]:
			n, err := strconv.Atoi(v)
			if err != nil {
				return "", nil, fmt.Errorf("invalid position spec %q: %s must be a number", spec, k)
			}
			fields[k] = n
		case positionBoolKeys[k]:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return "", nil, fmt.Errorf("invalid position spec %q: %s must be true or false", spec, k)
			}
			fields[k] = b
		default:
			fields[k] = v
		}
	}
	if posType == "" {
		return "", nil, fmt.Errorf("invalid position spec %q: missing type=article|custom|text|subtotal|discount|pagebreak", spec)
	}
	if _, ok := positionTypeNames[posType]; !ok {
		return "", nil, fmt.Errorf("unknown position type %q", posType)
	}
	return posType, fields, nil
}

func newOrderListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sales orders",
		Example: `  bexio kb-order list
  bexio kb-order list --order-by updated_at_desc --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			orders, err := client.ListOrders(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderOrders(cmd, orders)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	return cmd
}

func newOrderViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a sales order (including its positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			o, err := client.GetOrder(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, o.Raw, orderDetailOrder)
		},
	}
}

func newOrderSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search sales orders",
		Long: `Search sales orders. A bare term matches the title partially. --where
clauses use the raw API field names and add AND conditions (see
"bexio contact search --help" for the operator syntax).

Searchable fields: id, kb_item_status_id, document_nr, title, contact_id,
contact_sub_id, user_id, currency_id, total_gross, total_net, updated_at.
Status ids: 5 pending, 6 done, 15 partial, 21 canceled.`,
		Example: `  bexio kb-order search --where contact_id=17
  bexio kb-order search --where kb_item_status_id=5
  bexio kb-order search --where "updated_at>2026-01-01" -o json`,
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
			orders, err := client.SearchOrders(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderOrders(cmd, orders)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

func newOrderCreateCmd() *cobra.Command {
	var fields orderFieldFlags
	var positions []string
	var positionsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a sales order",
		Long: `Create a sales order. --contact-id is required; user_id defaults to the
authenticated user. Positions are passed as repeatable --position specs
("type=..." plus raw API fields) or as raw JSON via --positions-json:

  type=article  article_id, amount, and optionally unit_price, tax_id, text
  type=custom   text, amount, unit_price, and optionally unit_id, tax_id
  type=text     text
  type=subtotal / discount / pagebreak`,
		Example: `  bexio kb-order create --contact-id 17 --title "Website relaunch" \
      --position "type=custom,text=Consulting,amount=8,unit_price=150" \
      --position "type=article,article_id=5,amount=2"
  bexio kb-order create --contact-id 17 --positions-json '[{"type":"KbPositionText","text":"Hi"}]'`,
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
			o, err := client.CreateOrder(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), o.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created order %d (%s, total %s)\n", o.ID, o.DocumentNr, o.Total)
			return nil
		},
	}
	fields.register(cmd)
	cmd.Flags().StringArrayVar(&positions, "position", nil, `position spec, e.g. "type=article,article_id=5,amount=2" (repeatable)`)
	cmd.Flags().StringVar(&positionsJSON, "positions-json", "", "positions as a raw JSON array (advanced)")
	return cmd
}

func newOrderUpdateCmd() *cobra.Command {
	var fields orderFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update header fields of a sales order",
		Long:  "Update a sales order. Only the flags you pass are changed. Positions are managed with `bexio kb-order position`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
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
			o, err := client.UpdateOrder(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), o.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated order %d (%s)\n", o.ID, o.DocumentNr)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newOrderDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a sales order",
		Long:  "Permanently delete a sales order. Unlike contacts this CANNOT be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting an order is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteOrder(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted order %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newOrderPDFCmd() *cobra.Command {
	var out string
	var logopaper int
	cmd := &cobra.Command{
		Use:   "pdf <id>",
		Short: "Download the order as PDF",
		Example: `  bexio kb-order pdf 4
  bexio kb-order pdf 4 --out offer.pdf --logopaper 1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			pdf, err := client.OrderPDF(cmd.Context(), id, logopaper)
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
				path = fmt.Sprintf("order-%d.pdf", id)
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

func newOrderInvoiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "invoice <id>",
		Short: "Create an invoice from the order (all positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CreateInvoiceFromOrder(cmd.Context(), id)
			if err != nil {
				return err
			}
			return reportCreatedDocument(cmd, raw, "invoice")
		},
	}
}

func newOrderDeliveryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delivery <id>",
		Short: "Create a delivery from the order (all positions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CreateDeliveryFromOrder(cmd.Context(), id)
			if err != nil {
				return err
			}
			return reportCreatedDocument(cmd, raw, "delivery")
		},
	}
}

// reportCreatedDocument prints the id/document_nr of a document created from
// an order (invoice or delivery).
func reportCreatedDocument(cmd *cobra.Command, raw json.RawMessage, kind string) error {
	if flagOutput == "json" {
		return output.JSON(cmd.OutOrStdout(), raw)
	}
	var m struct {
		ID         int    `json:"id"`
		DocumentNr string `json:"document_nr"`
	}
	if err := json.Unmarshal(raw, &m); err != nil || m.ID == 0 {
		return output.JSON(cmd.OutOrStdout(), raw)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created %s %d (%s)\n", kind, m.ID, m.DocumentNr)
	return nil
}

func newOrderRepetitionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repetition",
		Short: "Show, edit, or delete the repetition of a recurring order",
	}
	view := &cobra.Command{
		Use:   "view <order-id>",
		Short: "Show the repetition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.GetOrderRepetition(cmd.Context(), id)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	var repJSON string
	edit := &cobra.Command{
		Use:   "edit <order-id>",
		Short: "Edit the repetition (raw JSON body)",
		Example: `  bexio kb-order repetition edit 4 --json \
      '{"start":"2026-09-01","end":null,"repetition":{"type":"monthly","interval":1}}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			if repJSON == "" {
				return fmt.Errorf("--json is required")
			}
			var body json.RawMessage
			if err := json.Unmarshal([]byte(repJSON), &body); err != nil {
				return fmt.Errorf("--json is not valid JSON: %w", err)
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.EditOrderRepetition(cmd.Context(), id, body)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	edit.Flags().StringVar(&repJSON, "json", "", "repetition object as JSON (see docs.bexio.com)")
	del := &cobra.Command{
		Use:   "delete <order-id>",
		Short: "Delete the repetition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteOrderRepetition(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted repetition of order %d\n", id)
			return nil
		},
	}
	cmd.AddCommand(view, edit, del)
	return cmd
}

func newOrderPositionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "Add, update, or delete positions of a sales order",
		Long: `Manage the positions of an existing sales order. Position specs use the
same "type=...,field=value,..." syntax as "kb-order create --position";
"bexio kb-order view <id>" shows the current positions with their ids.`,
	}
	add := &cobra.Command{
		Use:   "add <order-id> <spec>",
		Short: "Add a position",
		Example: `  bexio kb-order position add 4 "type=article,article_id=5,amount=2"
  bexio kb-order position add 4 "type=custom,text=Consulting,amount=8,unit_price=150"
  bexio kb-order position add 4 "type=text,text=Delivery in calendar week 40"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
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
			raw, err := client.CreatePosition(cmd.Context(), "kb_order", id, posType, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	update := &cobra.Command{
		Use:   "update <order-id> <position-id> <spec>",
		Short: "Update a position",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("order", args[0])
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
			raw, err := client.UpdatePosition(cmd.Context(), "kb_order", id, posType, posID, fields)
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	}
	var delType string
	del := &cobra.Command{
		Use:   "delete <order-id> <position-id>",
		Short: "Delete a position",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("order", args[0])
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
			if err := client.DeletePosition(cmd.Context(), "kb_order", id, delType, posID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted position %d from order %d\n", posID, id)
			return nil
		},
	}
	del.Flags().StringVar(&delType, "type", "", "position type: article|custom|text|subtotal|discount|pagebreak|subposition (required)")
	_ = del.MarkFlagRequired("type")
	cmd.AddCommand(add, update, del)
	return cmd
}
