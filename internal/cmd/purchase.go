package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	registerModule(newBillCmd)
	registerModule(newExpenseCmd)
	registerModule(newPurchaseOrderCmd)
	registerModule(newOutgoingPaymentCmd)
}

// ---------------------------------------------------------------------------
// shared helpers for the purchase commands
// ---------------------------------------------------------------------------

// purchaseID validates a 4.0 resource id (a UUID string, not a number).
func purchaseID(kind, arg string) (string, error) {
	id := strings.TrimSpace(arg)
	if id == "" {
		return "", fmt.Errorf("invalid %s id %q", kind, arg)
	}
	return id, nil
}

// purchaseEnum validates a positional enum argument case-insensitively and
// returns it in the API's upper-case spelling.
func purchaseEnum(kind, arg string, allowed []string) (string, error) {
	v := strings.ToUpper(strings.TrimSpace(arg))
	for _, a := range allowed {
		if a == v {
			return a, nil
		}
	}
	return "", fmt.Errorf("invalid %s %q (want %s)", kind, arg, strings.Join(allowed, ", "))
}

// purchaseListFlags adds the paging parameters of the 4.0 purchase lists.
// Unlike the 2.0 lists these page (limit/page), they do not offset.
func purchaseListFlags(cmd *cobra.Command, opts *api.PurchaseListOptions, sortFields string) {
	cmd.Flags().IntVar(&opts.Limit, "limit", 100, "results per page (API max 500)")
	cmd.Flags().IntVar(&opts.Page, "page", 0, "page to fetch (1-based)")
	cmd.Flags().StringVar(&opts.Order, "order", "", `sorting order: "asc" or "desc"`)
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "field to sort by ("+sortFields+")")
}

// purchaseFilterSet registers the query-parameter filters of a 4.0 list and
// collects the ones the user actually passed.
type purchaseFilterSet struct {
	values map[string]*string
	order  []string
}

func (s *purchaseFilterSet) add(cmd *cobra.Command, apiName, usage string) {
	if s.values == nil {
		s.values = map[string]*string{}
	}
	v := new(string)
	flag := strings.ReplaceAll(apiName, "_", "-")
	cmd.Flags().StringVar(v, flag, "", usage)
	s.values[apiName] = v
	s.order = append(s.order, apiName)
}

// collect returns the filters the user passed, as raw query parameters.
func (s *purchaseFilterSet) collect(cmd *cobra.Command) map[string][]string {
	out := map[string][]string{}
	for _, apiName := range s.order {
		flag := strings.ReplaceAll(apiName, "_", "-")
		if cmd.Flags().Changed(flag) {
			out[apiName] = []string{*s.values[apiName]}
		}
	}
	return out
}

func purchaseAmount(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// requirePurchaseFields reports the first missing member of a create
// payload, naming the flag that fills it.
func requirePurchaseFields(payload map[string]any, required [][2]string) error {
	for _, r := range required {
		if payload[r[1]] == nil {
			return fmt.Errorf("%s is required", r[0])
		}
	}
	return nil
}

// errNothingToUpdate is returned when an update command got no field flag.
// The updates read the current object first, so this check must happen
// before the request.
var errNothingToUpdate = fmt.Errorf("nothing to update: pass at least one field flag")

// decodePurchaseObject decodes a raw API object for the read-modify-write
// update flow.
func decodePurchaseObject(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode current object: %w", err)
	}
	return m, nil
}

// pickPurchaseFields copies the allowed keys of a decoded API object into a
// fresh payload. The 4.0 endpoints update with PUT and validate the complete
// object, so an update has to re-send everything the user did not change —
// but only the members the request schema accepts (the responses carry
// read-only extras like status, created_at or tax_calc).
func pickPurchaseFields(m map[string]any, allowed []string) map[string]any {
	out := map[string]any{}
	for _, k := range allowed {
		if v, ok := m[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}

// pickPurchaseList applies pickPurchaseFields to every element of a nested
// array (line_items, discounts).
func pickPurchaseList(v any, allowed []string) []any {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, pickPurchaseFields(m, allowed))
		}
	}
	return out
}

// parsePurchaseSpec parses "position=1,amount=56.8,tax_id=15" into an API
// object. Keys are raw API field names.
func parsePurchaseSpec(kind, spec string, intKeys, floatKeys map[string]bool) (map[string]any, error) {
	fields := map[string]any{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid %s spec %q: %q is not key=value", kind, spec, part)
		}
		k = strings.TrimSpace(k)
		switch {
		case intKeys[k]:
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("invalid %s spec %q: %s must be a number", kind, spec, k)
			}
			fields[k] = n
		case floatKeys[k]:
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid %s spec %q: %s must be a number", kind, spec, k)
			}
			fields[k] = f
		default:
			fields[k] = v
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty %s spec %q", kind, spec)
	}
	return fields, nil
}

// purchaseAddressFields are the members of the shared "address" object of
// bills and expenses.
var purchaseAddressFields = []string{
	"title", "salutation", "firstname_suffix", "lastname_company",
	"address_line", "postcode", "city", "country_code", "main_contact_id",
	"contact_address_id", "type",
}

// purchaseAddressFlags registers the flattened --address-* flags.
type purchaseAddressFlags struct {
	title            string
	salutation       string
	firstnameSuffix  string
	lastnameCompany  string
	addressLine      string
	postcode         string
	city             string
	countryCode      string
	mainContactID    int
	contactAddressID int
	addressType      string
	json             string
}

func (f *purchaseAddressFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.title, "address-title", "", "address: title")
	fl.StringVar(&f.salutation, "address-salutation", "", "address: salutation")
	fl.StringVar(&f.firstnameSuffix, "address-firstname-suffix", "", "address: first name / suffix")
	fl.StringVar(&f.lastnameCompany, "address-lastname-company", "", "address: last name or company name (required on create)")
	fl.StringVar(&f.addressLine, "address-line", "", "address: street and house number")
	fl.StringVar(&f.postcode, "address-postcode", "", "address: postcode")
	fl.StringVar(&f.city, "address-city", "", "address: city")
	fl.StringVar(&f.countryCode, "address-country-code", "", "address: ISO country code, e.g. CH")
	fl.IntVar(&f.mainContactID, "address-main-contact-id", 0, "address: contact id the address belongs to")
	fl.IntVar(&f.contactAddressID, "address-contact-address-id", 0, "address: additional address id of the contact")
	fl.StringVar(&f.addressType, "address-type", "", "address: PRIVATE or COMPANY (required on create)")
	fl.StringVar(&f.json, "address-json", "", "address as a raw JSON object (advanced; replaces the --address-* flags)")
}

// apply merges the changed address flags into fields["address"].
func (f *purchaseAddressFlags) apply(cmd *cobra.Command, fields map[string]any) error {
	if cmd.Flags().Changed("address-json") {
		var m map[string]any
		if err := json.Unmarshal([]byte(f.json), &m); err != nil {
			return fmt.Errorf("--address-json is not a valid JSON object: %w", err)
		}
		fields["address"] = m
		return nil
	}
	addr, _ := fields["address"].(map[string]any)
	if addr == nil {
		addr = map[string]any{}
	}
	setIfChanged(cmd, addr, "address-title", "title", f.title)
	setIfChanged(cmd, addr, "address-salutation", "salutation", f.salutation)
	setIfChanged(cmd, addr, "address-firstname-suffix", "firstname_suffix", f.firstnameSuffix)
	setIfChanged(cmd, addr, "address-lastname-company", "lastname_company", f.lastnameCompany)
	setIfChanged(cmd, addr, "address-line", "address_line", f.addressLine)
	setIfChanged(cmd, addr, "address-postcode", "postcode", f.postcode)
	setIfChanged(cmd, addr, "address-city", "city", f.city)
	setIfChanged(cmd, addr, "address-country-code", "country_code", f.countryCode)
	setIfChanged(cmd, addr, "address-main-contact-id", "main_contact_id", f.mainContactID)
	setIfChanged(cmd, addr, "address-contact-address-id", "contact_address_id", f.contactAddressID)
	if cmd.Flags().Changed("address-type") {
		t, err := purchaseEnum("--address-type", f.addressType, []string{"PRIVATE", "COMPANY"})
		if err != nil {
			return err
		}
		addr["type"] = t
	}
	if len(addr) > 0 {
		fields["address"] = addr
	}
	return nil
}

// newPurchaseDocumentNumberCmd builds the "document-number" subtree of bills
// and expenses. The endpoint validates a document number and reports the next
// free one.
func newPurchaseDocumentNumberCmd(kind string, check func(*api.Client, *cobra.Command, string) (*api.DocumentNumberCheck, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document-number",
		Short: "Check " + kind + " document numbers",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check <document-no>",
		Short: "Report whether a document number is still available",
		Long: `Report whether a document number is still available. The API answers with
"valid" and, when the number is taken, the next free "next_available_no".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			res, err := check(client, cmd, args[0])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), res.Raw)
			}
			output.Table(cmd.OutOrStdout(), []string{"document_no", "valid", "next_available_no"}, [][]string{{
				args[0], strconv.FormatBool(res.Valid), res.NextAvailableNo,
			}})
			return nil
		},
	})
	return cmd
}

// ---------------------------------------------------------------------------
// bills
// ---------------------------------------------------------------------------

// newBillCmd manages supplier bills (4.0 resource "purchase/bills").
func newBillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "purchase-bill",
		Aliases: []string{"bill"},
		Short:   "List, view, and modify supplier bills",
		Long: `Manage supplier bills (/4.0/purchase/bills).

Bill ids are UUIDs, not numbers. The 4.0 API updates with PUT and validates
the complete object, so "update" re-reads the bill and sends the merged
result — only the flags you pass change.`,
	}
	cmd.AddCommand(
		newBillListCmd(),
		newBillViewCmd(),
		newBillCreateCmd(),
		newBillUpdateCmd(),
		newBillDeleteCmd(),
		newBillBookCmd(),
		newBillActionCmd(),
		newPurchaseDocumentNumberCmd("bill", func(c *api.Client, cmd *cobra.Command, no string) (*api.DocumentNumberCheck, error) {
			return c.CheckBillDocumentNumber(cmd.Context(), no)
		}),
	)
	return cmd
}

var billDetailOrder = []string{
	"id", "document_no", "title", "status", "overdue", "supplier_id",
	"contact_partner_id", "firstname_suffix", "lastname_company",
	"vendor_ref", "bill_date", "due_date", "created_at", "currency_code",
	"base_currency_code", "exchange_rate", "base_currency_amount",
	"manual_amount", "amount_man", "amount_calc", "pending_amount",
	"item_net", "split_into_line_items", "purchase_order_id",
	"qr_bill_information", "address", "line_items", "discounts", "payment",
	"attachment_ids",
}

func renderBills(cmd *cobra.Command, bills []api.Bill) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(bills))
		for i, b := range bills {
			raws[i] = b.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(bills))
	for i, b := range bills {
		rows[i] = []string{
			b.ID,
			b.DocumentNo,
			b.Status,
			output.Truncate(b.Title, 30),
			output.Truncate(b.VendorName(), 24),
			b.BillDate,
			b.DueDate,
			b.CurrencyCode,
			purchaseAmount(b.Gross),
			purchaseAmount(b.PendingAmount),
		}
	}
	output.Table(cmd.OutOrStdout(),
		[]string{"id", "document_no", "status", "title", "vendor", "bill_date", "due_date", "currency", "gross", "pending"},
		rows)
	return nil
}

var (
	billLineItemIntKeys   = map[string]bool{"position": true, "tax_id": true, "booking_account_id": true}
	billLineItemFloatKeys = map[string]bool{"amount": true}
	billDiscountIntKeys   = map[string]bool{"position": true}
	billDiscountFloatKeys = map[string]bool{"amount": true}
)

// billUpdateFields are the members accepted by PUT /4.0/purchase/bills/{id}.
var billUpdateFields = []string{
	"document_no", "title", "supplier_id", "vendor_ref", "amount_man",
	"amount_calc", "manual_amount", "contact_partner_id", "bill_date",
	"due_date", "currency_code", "exchange_rate", "item_net",
	"split_into_line_items", "base_currency_amount", "attachment_ids",
	"address", "line_items", "discounts", "payment",
}

var (
	billLineItemFields = []string{"id", "position", "title", "tax_id", "amount", "booking_account_id"}
	billDiscountFields = []string{"id", "position", "amount"}
)

// billFieldFlags mirrors the writable fields of the bill create/update
// bodies.
type billFieldFlags struct {
	documentNo         string
	title              string
	vendorRef          string
	supplierID         int
	contactPartnerID   int
	billDate           string
	dueDate            string
	amountMan          float64
	amountCalc         float64
	manualAmount       bool
	itemNet            bool
	splitIntoLineItems bool
	currencyCode       string
	exchangeRate       float64
	baseCurrencyAmount float64
	purchaseOrderID    int
	qrBillInformation  string
	attachmentIDs      []string
	lineItems          []string
	lineItemsJSON      string
	discounts          []string
	discountsJSON      string
	paymentJSON        string
	address            purchaseAddressFlags
}

func (f *billFieldFlags) register(cmd *cobra.Command, update bool) {
	fl := cmd.Flags()
	if update {
		fl.StringVar(&f.documentNo, "document-no", "", "bill document number")
		fl.BoolVar(&f.splitIntoLineItems, "split-into-line-items", false, "split the bill amount into line items")
	}
	fl.StringVar(&f.title, "title", "", "bill title")
	fl.StringVar(&f.vendorRef, "vendor-ref", "", "reference of the vendor (their document number)")
	fl.IntVar(&f.supplierID, "supplier-id", 0, "contact id of the supplier (required on create)")
	fl.IntVar(&f.contactPartnerID, "contact-partner-id", 0, "contact id of the contact partner (required on create)")
	fl.StringVar(&f.billDate, "bill-date", "", "bill date, YYYY-MM-DD (required on create)")
	fl.StringVar(&f.dueDate, "due-date", "", "due date, YYYY-MM-DD (required on create)")
	fl.Float64Var(&f.amountMan, "amount-man", 0, "manually entered bill amount (used when --manual-amount)")
	fl.Float64Var(&f.amountCalc, "amount-calc", 0, "bill amount calculated from the line items (used without --manual-amount)")
	fl.BoolVar(&f.manualAmount, "manual-amount", false, "use amount_man instead of amount_calc as the bill amount")
	fl.BoolVar(&f.itemNet, "item-net", false, "line item amounts are net (default: gross)")
	fl.StringVar(&f.currencyCode, "currency-code", "", "currency code, e.g. CHF (required on create)")
	fl.Float64Var(&f.exchangeRate, "exchange-rate", 0, "exchange rate (required for a foreign currency)")
	fl.Float64Var(&f.baseCurrencyAmount, "base-currency-amount", 0, "amount in the base currency (required for a foreign currency)")
	if !update {
		fl.IntVar(&f.purchaseOrderID, "purchase-order-id", 0, "purchase order this bill belongs to")
		fl.StringVar(&f.qrBillInformation, "qr-bill-information", "", "payload of the scanned QR bill")
	}
	fl.StringArrayVar(&f.attachmentIDs, "attachment-id", nil, "file id to attach (repeatable)")
	fl.StringArrayVar(&f.lineItems, "line-item", nil, `line item spec, e.g. "position=1,title=Hosting,amount=56.8,tax_id=15,booking_account_id=16" (repeatable)`)
	fl.StringVar(&f.lineItemsJSON, "line-items-json", "", "line items as a raw JSON array (advanced)")
	fl.StringArrayVar(&f.discounts, "discount", nil, `discount spec, e.g. "position=1,amount=5.5" (repeatable)`)
	fl.StringVar(&f.discountsJSON, "discounts-json", "", "discounts as a raw JSON array (advanced)")
	fl.StringVar(&f.paymentJSON, "payment-json", "", "payment object as raw JSON (advanced)")
	f.address.register(cmd)
}

// apply merges the flags the user passed into fields (empty on create, the
// current object on update).
func (f *billFieldFlags) apply(cmd *cobra.Command, fields map[string]any) error {
	setIfChanged(cmd, fields, "document-no", "document_no", f.documentNo)
	setIfChanged(cmd, fields, "title", "title", f.title)
	setIfChanged(cmd, fields, "vendor-ref", "vendor_ref", f.vendorRef)
	setIfChanged(cmd, fields, "supplier-id", "supplier_id", f.supplierID)
	setIfChanged(cmd, fields, "contact-partner-id", "contact_partner_id", f.contactPartnerID)
	setIfChanged(cmd, fields, "bill-date", "bill_date", f.billDate)
	setIfChanged(cmd, fields, "due-date", "due_date", f.dueDate)
	setIfChanged(cmd, fields, "amount-man", "amount_man", f.amountMan)
	setIfChanged(cmd, fields, "amount-calc", "amount_calc", f.amountCalc)
	setIfChanged(cmd, fields, "manual-amount", "manual_amount", f.manualAmount)
	setIfChanged(cmd, fields, "item-net", "item_net", f.itemNet)
	setIfChanged(cmd, fields, "split-into-line-items", "split_into_line_items", f.splitIntoLineItems)
	setIfChanged(cmd, fields, "currency-code", "currency_code", f.currencyCode)
	setIfChanged(cmd, fields, "exchange-rate", "exchange_rate", f.exchangeRate)
	setIfChanged(cmd, fields, "base-currency-amount", "base_currency_amount", f.baseCurrencyAmount)
	setIfChanged(cmd, fields, "purchase-order-id", "purchase_order_id", f.purchaseOrderID)
	setIfChanged(cmd, fields, "qr-bill-information", "qr_bill_information", f.qrBillInformation)
	setIfChanged(cmd, fields, "attachment-id", "attachment_ids", f.attachmentIDs)

	items, err := purchaseSpecList("line item", f.lineItems, f.lineItemsJSON, "--line-item", "--line-items-json", billLineItemIntKeys, billLineItemFloatKeys)
	if err != nil {
		return err
	}
	if items != nil {
		fields["line_items"] = items
	}
	discounts, err := purchaseSpecList("discount", f.discounts, f.discountsJSON, "--discount", "--discounts-json", billDiscountIntKeys, billDiscountFloatKeys)
	if err != nil {
		return err
	}
	if discounts != nil {
		fields["discounts"] = discounts
	}
	if cmd.Flags().Changed("payment-json") {
		var m map[string]any
		if err := json.Unmarshal([]byte(f.paymentJSON), &m); err != nil {
			return fmt.Errorf("--payment-json is not a valid JSON object: %w", err)
		}
		fields["payment"] = m
	}
	return f.address.apply(cmd, fields)
}

// purchaseSpecList turns repeatable key=value specs or a raw JSON array into
// a nested API array. It returns nil when the user passed neither.
func purchaseSpecList(kind string, specs []string, rawJSON, specFlag, jsonFlag string, intKeys, floatKeys map[string]bool) ([]any, error) {
	var out []any
	for _, spec := range specs {
		m, err := parsePurchaseSpec(kind, spec, intKeys, floatKeys)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if rawJSON != "" {
		if len(out) > 0 {
			return nil, fmt.Errorf("%s and %s are mutually exclusive", specFlag, jsonFlag)
		}
		var raw []any
		if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
			return nil, fmt.Errorf("%s is not a valid JSON array: %w", jsonFlag, err)
		}
		out = raw
	}
	return out, nil
}

func newBillListCmd() *cobra.Command {
	var opts api.PurchaseListOptions
	var filters purchaseFilterSet
	var searchFields []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List and filter supplier bills",
		Long: `List supplier bills. The 4.0 API has no /search endpoint: filtering
happens with query parameters, so every filter is a flag here.

--search-term matches document_no, title, vendor_ref, currency_code and the
supplier name; restrict it with --field. --status selects a bucket:
DRAFTS, TODO, PAID or OVERDUE (OVERDUE = unpaid with a due_date in the past).`,
		Example: `  bexio purchase-bill list --status TODO
  bexio purchase-bill list --search-term hosting --field title
  bexio purchase-bill list --supplier-id 17 --due-date-end 2026-09-30 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			opts.Filters = filters.collect(cmd)
			opts.Filters["fields[]"] = append(opts.Filters["fields[]"], searchFields...)
			client, err := newClient()
			if err != nil {
				return err
			}
			bills, err := client.ListBills(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderBills(cmd, bills)
		},
	}
	purchaseListFlags(cmd, &opts, "document_no, title, bill_date, due_date, gross, net")
	filters.add(cmd, "search_term", "search term (3-255 characters)")
	filters.add(cmd, "status", "status bucket: "+strings.Join(api.BillListStatuses, ", "))
	filters.add(cmd, "bill_date_start", "earliest bill_date (YYYY-MM-DD)")
	filters.add(cmd, "bill_date_end", "latest bill_date (YYYY-MM-DD)")
	filters.add(cmd, "due_date_start", "earliest due_date (YYYY-MM-DD)")
	filters.add(cmd, "due_date_end", "latest due_date (YYYY-MM-DD)")
	filters.add(cmd, "vendor_ref", "vendor reference contains")
	filters.add(cmd, "title", "title contains")
	filters.add(cmd, "currency_code", "currency code contains")
	filters.add(cmd, "vendor", "vendor name contains")
	filters.add(cmd, "document_no", "document number contains")
	filters.add(cmd, "supplier_id", "contact id of the supplier")
	filters.add(cmd, "pending_amount_min", "lowest pending_amount")
	filters.add(cmd, "pending_amount_max", "highest pending_amount")
	filters.add(cmd, "gross_min", "lowest gross amount")
	filters.add(cmd, "gross_max", "highest gross amount")
	filters.add(cmd, "net_min", "lowest net amount")
	filters.add(cmd, "net_max", "highest net amount")
	filters.add(cmd, "average_exchange_rate_enabled", "true to list only bills using the average exchange rate")
	cmd.Flags().StringArrayVar(&searchFields, "field", nil,
		"restrict --search-term to a field (repeatable): "+strings.Join(api.BillSearchFields, ", "))
	return cmd
}

func newBillViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a bill (including line items, discounts, and payment)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("bill", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			bill, err := client.GetBill(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, bill.Raw, billDetailOrder)
		},
	}
}

func newBillCreateCmd() *cobra.Command {
	var fields billFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a bill (as draft)",
		Long: `Create a supplier bill. Required: --supplier-id, --contact-partner-id,
--currency-code, --bill-date, --due-date, an address (at least
--address-lastname-company and --address-type) and at least one --line-item.

The bill is created in status DRAFT; "bexio purchase-bill book <id> BOOKED"
posts it to the ledger.`,
		Example: `  bexio purchase-bill create --supplier-id 17 --contact-partner-id 17 \
      --currency-code CHF --bill-date 2026-08-01 --due-date 2026-08-31 \
      --address-lastname-company "bexio AG" --address-type COMPANY \
      --line-item "position=1,title=Hosting,amount=120.50,booking_account_id=16"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := map[string]any{}
			if err := fields.apply(cmd, payload); err != nil {
				return err
			}
			// Members the create schema requires even when empty.
			payload["manual_amount"] = fields.manualAmount
			payload["item_net"] = fields.itemNet
			if payload["attachment_ids"] == nil {
				payload["attachment_ids"] = []string{}
			}
			if payload["discounts"] == nil {
				payload["discounts"] = []any{}
			}
			if err := requirePurchaseFields(payload, [][2]string{
				{"--supplier-id", "supplier_id"},
				{"--contact-partner-id", "contact_partner_id"},
				{"--currency-code", "currency_code"},
				{"--bill-date", "bill_date"},
				{"--due-date", "due_date"},
			}); err != nil {
				return err
			}
			if payload["line_items"] == nil {
				return fmt.Errorf("at least one --line-item (or --line-items-json) is required")
			}
			addr, _ := payload["address"].(map[string]any)
			if addr["lastname_company"] == nil || addr["type"] == nil {
				return fmt.Errorf("--address-lastname-company and --address-type are required (or pass --address-json)")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			bill, err := client.CreateBill(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), bill.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created bill %s (%s, %s %s)\n",
				bill.ID, bill.DocumentNo, bill.CurrencyCode, purchaseAmount(bill.Amount()))
			return nil
		},
	}
	fields.register(cmd, false)
	return cmd
}

func newBillUpdateCmd() *cobra.Command {
	var fields billFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a bill",
		Long: `Update a bill. The 4.0 API replaces the whole object with PUT and
validates all required members, so the CLI reads the current bill first and
sends it back with your changes applied. Passing --line-item (or
--line-items-json) replaces the complete line item list.`,
		Example: `  bexio purchase-bill update 7572f70e-6bf5-47be-9a28-466423d8e3b1 --title "Hosting 2026"
  bexio purchase-bill update 7572f70e-6bf5-47be-9a28-466423d8e3b1 --due-date 2026-09-30`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("bill", args[0])
			if err != nil {
				return err
			}
			changes := map[string]any{}
			if err := fields.apply(cmd, changes); err != nil {
				return err
			}
			if len(changes) == 0 {
				return errNothingToUpdate
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			current, err := client.GetBill(cmd.Context(), id)
			if err != nil {
				return err
			}
			obj, err := decodePurchaseObject(current.Raw)
			if err != nil {
				return err
			}
			payload := pickPurchaseFields(obj, billUpdateFields)
			if addr, ok := payload["address"].(map[string]any); ok {
				payload["address"] = pickPurchaseFields(addr, purchaseAddressFields)
			}
			if items, ok := payload["line_items"]; ok {
				payload["line_items"] = pickPurchaseList(items, billLineItemFields)
			}
			payload["discounts"] = pickPurchaseList(payload["discounts"], billDiscountFields)
			if payload["attachment_ids"] == nil {
				payload["attachment_ids"] = []string{}
			}
			if err := fields.apply(cmd, payload); err != nil {
				return err
			}
			bill, err := client.UpdateBill(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), bill.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated bill %s (%s)\n", bill.ID, bill.DocumentNo)
			return nil
		},
	}
	fields.register(cmd, true)
	return cmd
}

func newBillDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a bill",
		Long:  "Permanently delete a bill. This cannot be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := purchaseID("bill", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a bill is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteBill(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted bill %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newBillBookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "book <id> <status>",
		Short: "Move a bill between DRAFT and BOOKED",
		Long: `Change the booking status of a bill (PUT .../bookings/{status}).

  BOOKED  book the draft, which posts it to the ledger
  DRAFT   revert a booked bill to draft`,
		Example: `  bexio purchase-bill book 7572f70e-6bf5-47be-9a28-466423d8e3b1 BOOKED
  bexio purchase-bill book 7572f70e-6bf5-47be-9a28-466423d8e3b1 DRAFT`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("bill", args[0])
			if err != nil {
				return err
			}
			status, err := purchaseEnum("status", args[1], api.BillBookingStatuses)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			bill, err := client.UpdateBillBookingStatus(cmd.Context(), id, status)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), bill.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bill %s (%s) is now %s\n", bill.ID, bill.DocumentNo, bill.Status)
			return nil
		},
	}
}

func newBillActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <id> <action>",
		Short: "Execute a bill action (" + strings.Join(api.PurchaseActions, ", ") + ")",
		Long: `Execute an action on a bill (POST .../actions).

  DUPLICATE  create a copy of the bill as a new draft`,
		Example: `  bexio purchase-bill action 7572f70e-6bf5-47be-9a28-466423d8e3b1 DUPLICATE`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("bill", args[0])
			if err != nil {
				return err
			}
			action, err := purchaseEnum("action", args[1], api.PurchaseActions)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			bill, err := client.BillAction(cmd.Context(), id, action)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), bill.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s on bill %s created bill %s (%s)\n", action, id, bill.ID, bill.DocumentNo)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// expenses
// ---------------------------------------------------------------------------

// newExpenseCmd manages purchase expenses (4.0 resource "expenses").
func newExpenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expense",
		Short: "List, view, and modify expenses",
		Long: `Manage expenses (/4.0/expenses).

Expense ids are UUIDs, not numbers. The 4.0 API updates with PUT and
validates the complete object, so "update" re-reads the expense and sends the
merged result — only the flags you pass change.`,
	}
	cmd.AddCommand(
		newExpenseListCmd(),
		newExpenseViewCmd(),
		newExpenseCreateCmd(),
		newExpenseUpdateCmd(),
		newExpenseDeleteCmd(),
		newExpenseBookCmd(),
		newExpenseActionCmd(),
		newPurchaseDocumentNumberCmd("expense", func(c *api.Client, cmd *cobra.Command, no string) (*api.DocumentNumberCheck, error) {
			return c.CheckExpenseDocumentNumber(cmd.Context(), no)
		}),
	)
	return cmd
}

var expenseDetailOrder = []string{
	"id", "document_no", "title", "status", "supplier_id",
	"firstname_suffix", "lastname_company", "paid_on", "created_at",
	"currency_code", "base_currency_code", "exchange_rate",
	"base_currency_amount", "amount", "tax_id", "tax_man", "tax_calc",
	"bank_account_id", "booking_account_id", "project_id",
	"chargeable_contact_id", "transaction_id", "invoice_id", "address",
	"attachment_ids",
}

// expenseUpdateFields are the members accepted by PUT /4.0/expenses/{id}.
var expenseUpdateFields = []string{
	"paid_on", "currency_code", "exchange_rate", "supplier_id",
	"document_no", "title", "bank_account_id", "booking_account_id",
	"amount", "tax_id", "base_currency_amount", "attachment_ids", "address",
}

func renderExpenses(cmd *cobra.Command, expenses []api.Expense) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(expenses))
		for i, e := range expenses {
			raws[i] = e.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(expenses))
	for i, e := range expenses {
		amount := e.Gross
		if amount == 0 {
			amount = e.Amount
		}
		rows[i] = []string{
			e.ID,
			e.DocumentNo,
			e.Status,
			output.Truncate(e.Title, 30),
			output.Truncate(e.VendorName(), 24),
			e.PaidOn,
			e.CurrencyCode,
			purchaseAmount(e.Net),
			purchaseAmount(amount),
		}
	}
	output.Table(cmd.OutOrStdout(),
		[]string{"id", "document_no", "status", "title", "vendor", "paid_on", "currency", "net", "gross"},
		rows)
	return nil
}

// expenseFieldFlags mirrors the writable fields of the expense create/update
// bodies.
type expenseFieldFlags struct {
	documentNo         string
	title              string
	supplierID         int
	paidOn             string
	amount             float64
	currencyCode       string
	exchangeRate       float64
	baseCurrencyAmount float64
	bankAccountID      int
	bookingAccountID   int
	taxID              int
	attachmentIDs      []string
	address            purchaseAddressFlags
}

func (f *expenseFieldFlags) register(cmd *cobra.Command, update bool) {
	fl := cmd.Flags()
	if update {
		fl.StringVar(&f.documentNo, "document-no", "", "expense document number")
	}
	fl.StringVar(&f.title, "title", "", "expense title")
	fl.IntVar(&f.supplierID, "supplier-id", 0, "contact id of the supplier")
	fl.StringVar(&f.paidOn, "paid-on", "", "payment date, YYYY-MM-DD (required on create)")
	fl.Float64Var(&f.amount, "amount", 0, "expense amount (required on create)")
	fl.StringVar(&f.currencyCode, "currency-code", "", "currency code, e.g. CHF (required on create)")
	fl.Float64Var(&f.exchangeRate, "exchange-rate", 0, "exchange rate (required for a foreign currency)")
	fl.Float64Var(&f.baseCurrencyAmount, "base-currency-amount", 0, "amount in the base currency (required for a foreign currency)")
	fl.IntVar(&f.bankAccountID, "bank-account-id", 0, "bank account the expense was paid from")
	fl.IntVar(&f.bookingAccountID, "booking-account-id", 0, "booking (ledger) account id")
	fl.IntVar(&f.taxID, "tax-id", 0, "tax id")
	fl.StringArrayVar(&f.attachmentIDs, "attachment-id", nil, "file id to attach (repeatable)")
	f.address.register(cmd)
}

func (f *expenseFieldFlags) apply(cmd *cobra.Command, fields map[string]any) error {
	setIfChanged(cmd, fields, "document-no", "document_no", f.documentNo)
	setIfChanged(cmd, fields, "title", "title", f.title)
	setIfChanged(cmd, fields, "supplier-id", "supplier_id", f.supplierID)
	setIfChanged(cmd, fields, "paid-on", "paid_on", f.paidOn)
	setIfChanged(cmd, fields, "amount", "amount", f.amount)
	setIfChanged(cmd, fields, "currency-code", "currency_code", f.currencyCode)
	setIfChanged(cmd, fields, "exchange-rate", "exchange_rate", f.exchangeRate)
	setIfChanged(cmd, fields, "base-currency-amount", "base_currency_amount", f.baseCurrencyAmount)
	setIfChanged(cmd, fields, "bank-account-id", "bank_account_id", f.bankAccountID)
	setIfChanged(cmd, fields, "booking-account-id", "booking_account_id", f.bookingAccountID)
	setIfChanged(cmd, fields, "tax-id", "tax_id", f.taxID)
	setIfChanged(cmd, fields, "attachment-id", "attachment_ids", f.attachmentIDs)
	return f.address.apply(cmd, fields)
}

func newExpenseListCmd() *cobra.Command {
	var opts api.PurchaseListOptions
	var filters purchaseFilterSet
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List and filter expenses",
		Long: `List expenses. The 4.0 API has no /search endpoint: filtering happens
with query parameters, so every filter is a flag here.`,
		Example: `  bexio expense list --paid-on-start 2026-01-01
  bexio expense list --supplier-id 17 --gross-min 100 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			opts.Filters = filters.collect(cmd)
			client, err := newClient()
			if err != nil {
				return err
			}
			expenses, err := client.ListExpenses(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderExpenses(cmd, expenses)
		},
	}
	purchaseListFlags(cmd, &opts, "document_no, title, paid_on, gross, net")
	filters.add(cmd, "vendor", "vendor name contains")
	filters.add(cmd, "title", "title contains")
	filters.add(cmd, "currency_code", "currency code contains")
	filters.add(cmd, "document_no", "document number contains")
	filters.add(cmd, "supplier_id", "contact id of the supplier")
	filters.add(cmd, "project_id", "project id (UUID)")
	filters.add(cmd, "paid_on_start", "earliest paid_on (YYYY-MM-DD)")
	filters.add(cmd, "paid_on_end", "latest paid_on (YYYY-MM-DD)")
	filters.add(cmd, "created_at_start", "earliest created_at (YYYY-MM-DD)")
	filters.add(cmd, "created_at_end", "latest created_at (YYYY-MM-DD)")
	filters.add(cmd, "gross_min", "lowest gross amount")
	filters.add(cmd, "gross_max", "highest gross amount")
	filters.add(cmd, "net_min", "lowest net amount")
	filters.add(cmd, "net_max", "highest net amount")
	return cmd
}

func newExpenseViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show an expense",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("expense", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			exp, err := client.GetExpense(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, exp.Raw, expenseDetailOrder)
		},
	}
}

func newExpenseCreateCmd() *cobra.Command {
	var fields expenseFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an expense (as draft)",
		Long: `Create an expense. Required: --paid-on, --currency-code and --amount.
The expense is created in status DRAFT; "bexio expense book <id> DONE" books
it.`,
		Example: `  bexio expense create --paid-on 2026-08-01 --currency-code CHF --amount 30.90 \
      --booking-account-id 4 --title "Office supplies"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := map[string]any{}
			if err := fields.apply(cmd, payload); err != nil {
				return err
			}
			if payload["attachment_ids"] == nil {
				payload["attachment_ids"] = []string{}
			}
			if err := requirePurchaseFields(payload, [][2]string{
				{"--paid-on", "paid_on"},
				{"--currency-code", "currency_code"},
				{"--amount", "amount"},
			}); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			exp, err := client.CreateExpense(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), exp.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created expense %s (%s, %s %s)\n",
				exp.ID, exp.DocumentNo, exp.CurrencyCode, purchaseAmount(exp.Amount))
			return nil
		},
	}
	fields.register(cmd, false)
	return cmd
}

func newExpenseUpdateCmd() *cobra.Command {
	var fields expenseFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an expense",
		Long: `Update an expense. The 4.0 API replaces the whole object with PUT and
validates all required members, so the CLI reads the current expense first
and sends it back with your changes applied.`,
		Example: `  bexio expense update 1355499f-aa07-4382-887e-acaf0323e6f6 --title "Office supplies"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("expense", args[0])
			if err != nil {
				return err
			}
			changes := map[string]any{}
			if err := fields.apply(cmd, changes); err != nil {
				return err
			}
			if len(changes) == 0 {
				return errNothingToUpdate
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			current, err := client.GetExpense(cmd.Context(), id)
			if err != nil {
				return err
			}
			obj, err := decodePurchaseObject(current.Raw)
			if err != nil {
				return err
			}
			payload := pickPurchaseFields(obj, expenseUpdateFields)
			if addr, ok := payload["address"].(map[string]any); ok {
				payload["address"] = pickPurchaseFields(addr, purchaseAddressFields)
			}
			if payload["attachment_ids"] == nil {
				payload["attachment_ids"] = []string{}
			}
			if err := fields.apply(cmd, payload); err != nil {
				return err
			}
			exp, err := client.UpdateExpense(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), exp.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated expense %s (%s)\n", exp.ID, exp.DocumentNo)
			return nil
		},
	}
	fields.register(cmd, true)
	return cmd
}

func newExpenseDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete an expense",
		Long:  "Permanently delete an expense. This cannot be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := purchaseID("expense", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting an expense is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteExpense(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted expense %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newExpenseBookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "book <id> <status>",
		Short: "Move an expense between DRAFT and DONE",
		Long: `Change the booking status of an expense (PUT .../bookings/{status}).

  DONE   book the draft, which posts it to the ledger
  DRAFT  revert a booked expense to draft`,
		Example: `  bexio expense book 1355499f-aa07-4382-887e-acaf0323e6f6 DONE`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("expense", args[0])
			if err != nil {
				return err
			}
			status, err := purchaseEnum("status", args[1], api.ExpenseStatuses)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			exp, err := client.UpdateExpenseBookingStatus(cmd.Context(), id, status)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), exp.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Expense %s (%s) is now %s\n", exp.ID, exp.DocumentNo, exp.Status)
			return nil
		},
	}
}

func newExpenseActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <id> <action>",
		Short: "Execute an expense action (" + strings.Join(api.PurchaseActions, ", ") + ")",
		Long: `Execute an action on an expense (POST .../actions).

  DUPLICATE  create a copy of the expense as a new draft`,
		Example: `  bexio expense action 1355499f-aa07-4382-887e-acaf0323e6f6 DUPLICATE`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("expense", args[0])
			if err != nil {
				return err
			}
			action, err := purchaseEnum("action", args[1], api.PurchaseActions)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			exp, err := client.ExpenseAction(cmd.Context(), id, action)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), exp.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s on expense %s created expense %s (%s)\n", action, id, exp.ID, exp.DocumentNo)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// purchase orders
// ---------------------------------------------------------------------------

// newPurchaseOrderCmd manages purchase orders (3.0 resource
// "purchase_orders").
func newPurchaseOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purchase-order",
		Short: "List, view, and modify purchase orders",
		Long: `Manage purchase orders (/3.0/purchase_orders).

Purchase orders are a 3.0 kb document with integer ids and limit/offset
paging. Unlike the 2.0 documents the update is a PUT, so the CLI reads the
order first and sends it back with your changes applied.

Status ids (read-only): 22 draft, 23 open, 24 partly, 25 done, 26 canceled.`,
	}
	cmd.AddCommand(
		newPurchaseOrderListCmd(),
		newPurchaseOrderViewCmd(),
		newPurchaseOrderCreateCmd(),
		newPurchaseOrderUpdateCmd(),
		newPurchaseOrderDeleteCmd(),
	)
	return cmd
}

var purchaseOrderDetailOrder = []string{
	"id", "document_nr", "title", "kb_item_status_id", "contact_id",
	"contact_sub_id", "user_id", "salesman_user_id", "project_id",
	"language_id", "currency_id", "bank_account_id", "payment_type_id",
	"kb_payment_template_id", "logopaper_id", "template_slug", "mwst_type",
	"mwst_is_net", "is_compact_view", "show_position_taxes", "is_valid_from",
	"is_valid_to", "is_valid_until", "delivery_address_type",
	"contact_address_manual", "delivery_address_manual",
	"terms_of_payment_text", "header", "footer", "reference",
	"api_reference", "mail", "nb_decimals_amount", "nb_decimals_price",
	"total_rounding_difference", "viewed_by_client_at", "created_at",
	"updated_at", "positions",
}

// purchaseOrderUpdateFields are the writable members of
// PUT /3.0/purchase_orders/{id} (the read-only kb_item_status_id,
// total_rounding_difference and viewed_by_client_at are dropped).
var purchaseOrderUpdateFields = []string{
	"document_nr", "kb_payment_template_id", "payment_type_id", "title",
	"contact_id", "contact_sub_id", "template_slug", "user_id", "project_id",
	"logopaper_id", "language_id", "bank_account_id", "currency_id",
	"header", "footer", "mwst_type", "mwst_is_net", "is_compact_view",
	"show_position_taxes", "salesman_user_id", "is_valid_from", "is_valid_to",
	"delivery_address_type", "contact_address_manual",
	"delivery_address_manual", "nb_decimals_amount", "nb_decimals_price",
	"terms_of_payment_text", "reference", "api_reference", "mail",
	"is_valid_until",
}

func renderPurchaseOrders(cmd *cobra.Command, orders []api.PurchaseOrder) error {
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
			shortDate(o.IsValidFrom),
			shortDate(o.IsValidTo),
			shortDate(o.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(),
		[]string{"id", "document_nr", "status", "title", "contact_id", "valid_from", "valid_to", "updated"},
		rows)
	return nil
}

// purchaseOrderFieldFlags mirrors the writable header fields of a purchase
// order.
type purchaseOrderFieldFlags struct {
	documentNr            string
	title                 string
	contactID             int
	contactSubID          int
	userID                int
	salesmanUserID        int
	projectID             int
	languageID            int
	currencyID            int
	bankAccountID         int
	paymentTypeID         int
	kbPaymentTemplateID   int
	logopaperID           int
	templateSlug          string
	header                string
	footer                string
	mwstType              string
	mwstIsNet             bool
	isCompactView         bool
	showPositionTaxes     bool
	isValidFrom           string
	isValidTo             string
	isValidUntil          string
	deliveryAddressType   string
	contactAddressManual  string
	deliveryAddressManual string
	nbDecimalsAmount      int
	nbDecimalsPrice       int
	termsOfPaymentText    string
	reference             string
	apiReference          string
	mail                  string
}

func (f *purchaseOrderFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.documentNr, "document-nr", "", "document number (only if automatic numbering is off)")
	fl.StringVar(&f.title, "title", "", "purchase order title")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id of the supplier (required on create)")
	fl.IntVar(&f.contactSubID, "contact-sub-id", 0, "contact person id")
	fl.IntVar(&f.userID, "user-id", 0, "user id")
	fl.IntVar(&f.salesmanUserID, "salesman-user-id", 0, "salesman user id")
	fl.IntVar(&f.projectID, "project-id", 0, "project id")
	fl.IntVar(&f.languageID, "language-id", 0, "language id")
	fl.IntVar(&f.currencyID, "currency-id", 0, "currency id")
	fl.IntVar(&f.bankAccountID, "bank-account-id", 0, "bank account id")
	fl.IntVar(&f.paymentTypeID, "payment-type-id", 0, "payment type id")
	fl.IntVar(&f.kbPaymentTemplateID, "kb-payment-template-id", 0, "payment template id")
	fl.IntVar(&f.logopaperID, "logopaper-id", 0, "logopaper id")
	fl.StringVar(&f.templateSlug, "template-slug", "", "document template slug")
	fl.StringVar(&f.header, "header", "", "document header text")
	fl.StringVar(&f.footer, "footer", "", "document footer text")
	fl.StringVar(&f.mwstType, "mwst-type", "", "tax mode: included, excluded, or exempt")
	fl.BoolVar(&f.mwstIsNet, "mwst-is-net", false, "taxes shown additionally to a total including taxes")
	fl.BoolVar(&f.isCompactView, "is-compact-view", false, "render the document in compact view")
	fl.BoolVar(&f.showPositionTaxes, "show-position-taxes", false, "show taxes per position")
	fl.StringVar(&f.isValidFrom, "is-valid-from", "", "document date (YYYY-MM-DD)")
	fl.StringVar(&f.isValidTo, "is-valid-to", "", "due date (YYYY-MM-DD)")
	fl.StringVar(&f.isValidUntil, "is-valid-until", "", "delivery date (YYYY-MM-DD)")
	fl.StringVar(&f.deliveryAddressType, "delivery-address-type", "", "delivery address: contact_address or manual")
	fl.StringVar(&f.contactAddressManual, "contact-address-manual", "", "manual supplier address")
	fl.StringVar(&f.deliveryAddressManual, "delivery-address-manual", "", "manual delivery address")
	fl.IntVar(&f.nbDecimalsAmount, "nb-decimals-amount", 0, "decimal digits for amounts")
	fl.IntVar(&f.nbDecimalsPrice, "nb-decimals-price", 0, "decimal digits for prices")
	fl.StringVar(&f.termsOfPaymentText, "terms-of-payment-text", "", "terms of payment text")
	fl.StringVar(&f.reference, "reference", "", "reference shown on the document")
	fl.StringVar(&f.apiReference, "api-reference", "", "free-form API reference field (only visible via API)")
	fl.StringVar(&f.mail, "mail", "", "email address of the supplier")
}

func (f *purchaseOrderFieldFlags) apply(cmd *cobra.Command, fields map[string]any) {
	setIfChanged(cmd, fields, "document-nr", "document_nr", f.documentNr)
	setIfChanged(cmd, fields, "title", "title", f.title)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", f.contactSubID)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "salesman-user-id", "salesman_user_id", f.salesmanUserID)
	setIfChanged(cmd, fields, "project-id", "project_id", f.projectID)
	setIfChanged(cmd, fields, "language-id", "language_id", f.languageID)
	setIfChanged(cmd, fields, "currency-id", "currency_id", f.currencyID)
	setIfChanged(cmd, fields, "bank-account-id", "bank_account_id", f.bankAccountID)
	setIfChanged(cmd, fields, "payment-type-id", "payment_type_id", f.paymentTypeID)
	setIfChanged(cmd, fields, "kb-payment-template-id", "kb_payment_template_id", f.kbPaymentTemplateID)
	setIfChanged(cmd, fields, "logopaper-id", "logopaper_id", f.logopaperID)
	setIfChanged(cmd, fields, "template-slug", "template_slug", f.templateSlug)
	setIfChanged(cmd, fields, "header", "header", f.header)
	setIfChanged(cmd, fields, "footer", "footer", f.footer)
	setIfChanged(cmd, fields, "mwst-type", "mwst_type", f.mwstType)
	setIfChanged(cmd, fields, "mwst-is-net", "mwst_is_net", f.mwstIsNet)
	setIfChanged(cmd, fields, "is-compact-view", "is_compact_view", f.isCompactView)
	setIfChanged(cmd, fields, "show-position-taxes", "show_position_taxes", f.showPositionTaxes)
	setIfChanged(cmd, fields, "is-valid-from", "is_valid_from", f.isValidFrom)
	setIfChanged(cmd, fields, "is-valid-to", "is_valid_to", f.isValidTo)
	setIfChanged(cmd, fields, "is-valid-until", "is_valid_until", f.isValidUntil)
	setIfChanged(cmd, fields, "delivery-address-type", "delivery_address_type", f.deliveryAddressType)
	setIfChanged(cmd, fields, "contact-address-manual", "contact_address_manual", f.contactAddressManual)
	setIfChanged(cmd, fields, "delivery-address-manual", "delivery_address_manual", f.deliveryAddressManual)
	setIfChanged(cmd, fields, "nb-decimals-amount", "nb_decimals_amount", f.nbDecimalsAmount)
	setIfChanged(cmd, fields, "nb-decimals-price", "nb_decimals_price", f.nbDecimalsPrice)
	setIfChanged(cmd, fields, "terms-of-payment-text", "terms_of_payment_text", f.termsOfPaymentText)
	setIfChanged(cmd, fields, "reference", "reference", f.reference)
	setIfChanged(cmd, fields, "api-reference", "api_reference", f.apiReference)
	setIfChanged(cmd, fields, "mail", "mail", f.mail)
}

func newPurchaseOrderListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List purchase orders",
		Example: `  bexio purchase-order list
  bexio purchase-order list --order-by updated_at_desc --limit 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			orders, err := client.ListPurchaseOrders(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderPurchaseOrders(cmd, orders)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	return cmd
}

func newPurchaseOrderViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a purchase order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("purchase order", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			order, err := client.GetPurchaseOrder(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, order.Raw, purchaseOrderDetailOrder)
		},
	}
}

func newPurchaseOrderCreateCmd() *cobra.Command {
	var fields purchaseOrderFieldFlags
	var positionsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a purchase order",
		Long: `Create a purchase order. --contact-id (the supplier) is required.

Positions are a nested object with "required", "optional", and "discount"
arrays, so they are passed as raw JSON via --positions-json.`,
		Example: `  bexio purchase-order create --contact-id 17 --title "Hardware order"
  bexio purchase-order create --contact-id 17 \
      --positions-json '{"required":[{"type":"text","text":"Delivery in week 40"}]}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := map[string]any{}
			fields.apply(cmd, payload)
			if payload["contact_id"] == nil {
				return fmt.Errorf("--contact-id is required")
			}
			if positionsJSON != "" {
				var positions map[string]any
				if err := json.Unmarshal([]byte(positionsJSON), &positions); err != nil {
					return fmt.Errorf("--positions-json is not a valid JSON object: %w", err)
				}
				payload["positions"] = positions
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			order, err := client.CreatePurchaseOrder(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), order.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created purchase order %d (%s)\n", order.ID, order.DocumentNr)
			return nil
		},
	}
	fields.register(cmd)
	cmd.Flags().StringVar(&positionsJSON, "positions-json", "",
		`positions as a raw JSON object with "required"/"optional"/"discount" arrays`)
	return cmd
}

func newPurchaseOrderUpdateCmd() *cobra.Command {
	var fields purchaseOrderFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update header fields of a purchase order",
		Long: `Update a purchase order. The 3.0 API replaces the document with PUT, so
the CLI reads the order first and sends it back with your changes applied.
Only the flags you pass change.`,
		Example: `  bexio purchase-order update 4 --title "Hardware order 2026"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("purchase order", args[0])
			if err != nil {
				return err
			}
			changes := map[string]any{}
			fields.apply(cmd, changes)
			if len(changes) == 0 {
				return errNothingToUpdate
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			current, err := client.GetPurchaseOrder(cmd.Context(), id)
			if err != nil {
				return err
			}
			obj, err := decodePurchaseObject(current.Raw)
			if err != nil {
				return err
			}
			payload := pickPurchaseFields(obj, purchaseOrderUpdateFields)
			fields.apply(cmd, payload)
			order, err := client.UpdatePurchaseOrder(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), order.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated purchase order %d (%s)\n", order.ID, order.DocumentNr)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newPurchaseOrderDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a purchase order",
		Long:  "Permanently delete a purchase order. This cannot be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("purchase order", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a purchase order is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeletePurchaseOrder(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted purchase order %d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

// ---------------------------------------------------------------------------
// outgoing payments
// ---------------------------------------------------------------------------

// newOutgoingPaymentCmd manages the payments of supplier bills (4.0 resource
// "purchase/outgoing-payments").
func newOutgoingPaymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outgoing-payment",
		Short: "List, view, and modify the payments of supplier bills",
		Long: `Manage outgoing payments (/4.0/purchase/outgoing-payments).

Payments always belong to a bill, so "list" requires --bill-id. Ids are
UUIDs. Quirk: the update PUT goes to the collection path and addresses the
payment through "payment_id" in the body, not through a path segment.`,
	}
	cmd.AddCommand(
		newOutgoingPaymentListCmd(),
		newOutgoingPaymentViewCmd(),
		newOutgoingPaymentCreateCmd(),
		newOutgoingPaymentUpdateCmd(),
		newOutgoingPaymentDeleteCmd(),
	)
	return cmd
}

var outgoingPaymentDetailOrder = []string{
	"id", "bill_id", "status", "payment_type", "execution_date", "amount",
	"currency_code", "exchange_rate", "is_salary_payment", "fee_type",
	"reference_no", "message", "booking_text", "note",
	"sender_bank_account_id", "sender_iban", "sender_name", "sender_street",
	"sender_house_no", "sender_postcode", "sender_city",
	"sender_country_code", "sender_bc_no", "sender_bank_no",
	"sender_bank_name", "receiver_account_no", "receiver_iban",
	"receiver_name", "receiver_street", "receiver_house_no",
	"receiver_postcode", "receiver_city", "receiver_country_code",
	"receiver_bc_no", "receiver_bank_no", "receiver_bank_name",
	"banking_payment_id", "banking_payment_entry_id", "transaction_id",
	"created_at",
}

// outgoingPaymentUpdateFields are the members accepted by
// PUT /4.0/purchase/outgoing-payments (payment_id is added separately).
var outgoingPaymentUpdateFields = []string{
	"execution_date", "amount", "fee_type", "is_salary_payment",
	"reference_no", "message", "receiver_iban", "receiver_name",
	"receiver_street", "receiver_house_no", "receiver_city",
	"receiver_postcode", "receiver_country_code",
}

func renderOutgoingPayments(cmd *cobra.Command, payments []api.OutgoingPayment) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(payments))
		for i, p := range payments {
			raws[i] = p.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(payments))
	for i, p := range payments {
		receiver := p.ReceiverIBAN
		if receiver == "" {
			receiver = p.ReceiverAccountNo
		}
		rows[i] = []string{
			p.ID,
			p.Status,
			p.PaymentType,
			p.ExecutionDate,
			purchaseAmount(p.Amount),
			strconv.Itoa(p.SenderBankAccountID),
			receiver,
		}
	}
	output.Table(cmd.OutOrStdout(),
		[]string{"id", "status", "payment_type", "execution_date", "amount", "sender_bank_account_id", "receiver"},
		rows)
	return nil
}

// outgoingPaymentFieldFlags mirrors the writable fields of the outgoing
// payment create/update bodies.
type outgoingPaymentFieldFlags struct {
	billID              string
	paymentType         string
	executionDate       string
	amount              float64
	currencyCode        string
	exchangeRate        float64
	note                string
	senderBankAccountID int
	senderIBAN          string
	senderName          string
	senderStreet        string
	senderHouseNo       string
	senderCity          string
	senderPostcode      string
	senderCountryCode   string
	senderBcNo          string
	senderBankNo        string
	senderBankName      string
	receiverAccountNo   string
	receiverIBAN        string
	receiverName        string
	receiverStreet      string
	receiverHouseNo     string
	receiverCity        string
	receiverPostcode    string
	receiverCountryCode string
	receiverBcNo        string
	receiverBankNo      string
	receiverBankName    string
	feeType             string
	isSalaryPayment     bool
	referenceNo         string
	message             string
	bookingText         string
}

func (f *outgoingPaymentFieldFlags) register(cmd *cobra.Command, update bool) {
	fl := cmd.Flags()
	if !update {
		fl.StringVar(&f.billID, "bill-id", "", "id of the bill being paid (required)")
		fl.StringVar(&f.paymentType, "payment-type", "", "payment type: "+strings.Join(api.OutgoingPaymentTypes, ", ")+" (required)")
		fl.StringVar(&f.currencyCode, "currency-code", "", "currency code, e.g. CHF (required)")
		fl.Float64Var(&f.exchangeRate, "exchange-rate", 0, "exchange rate (required)")
		fl.IntVar(&f.senderBankAccountID, "sender-bank-account-id", 0, "bank account the payment is sent from (required)")
		fl.StringVar(&f.note, "note", "", "internal note")
		fl.StringVar(&f.senderIBAN, "sender-iban", "", "IBAN of the sender")
		fl.StringVar(&f.senderName, "sender-name", "", "name of the sender")
		fl.StringVar(&f.senderStreet, "sender-street", "", "street of the sender")
		fl.StringVar(&f.senderHouseNo, "sender-house-no", "", "house number of the sender")
		fl.StringVar(&f.senderCity, "sender-city", "", "city of the sender")
		fl.StringVar(&f.senderPostcode, "sender-postcode", "", "postcode of the sender")
		fl.StringVar(&f.senderCountryCode, "sender-country-code", "", "country code of the sender")
		fl.StringVar(&f.senderBcNo, "sender-bc-no", "", "bank clearing number of the sender")
		fl.StringVar(&f.senderBankNo, "sender-bank-no", "", "bank number of the sender")
		fl.StringVar(&f.senderBankName, "sender-bank-name", "", "bank name of the sender")
		fl.StringVar(&f.receiverAccountNo, "receiver-account-no", "", "account number of the receiver")
		fl.StringVar(&f.receiverBcNo, "receiver-bc-no", "", "bank clearing number of the receiver")
		fl.StringVar(&f.receiverBankNo, "receiver-bank-no", "", "bank number of the receiver")
		fl.StringVar(&f.receiverBankName, "receiver-bank-name", "", "bank name of the receiver")
		fl.StringVar(&f.bookingText, "booking-text", "", "booking text")
	}
	fl.StringVar(&f.executionDate, "execution-date", "", "execution date, YYYY-MM-DD (required on create)")
	fl.Float64Var(&f.amount, "amount", 0, "payment amount (required on create)")
	fl.StringVar(&f.receiverIBAN, "receiver-iban", "", "IBAN of the receiver")
	fl.StringVar(&f.receiverName, "receiver-name", "", "name of the receiver")
	fl.StringVar(&f.receiverStreet, "receiver-street", "", "street of the receiver")
	fl.StringVar(&f.receiverHouseNo, "receiver-house-no", "", "house number of the receiver")
	fl.StringVar(&f.receiverCity, "receiver-city", "", "city of the receiver")
	fl.StringVar(&f.receiverPostcode, "receiver-postcode", "", "postcode of the receiver")
	fl.StringVar(&f.receiverCountryCode, "receiver-country-code", "", "country code of the receiver")
	fl.StringVar(&f.feeType, "fee-type", "", "who pays the transfer fee: "+strings.Join(api.OutgoingPaymentFeeTypes, ", "))
	fl.BoolVar(&f.isSalaryPayment, "is-salary-payment", false, "mark the payment as a salary payment")
	fl.StringVar(&f.referenceNo, "reference-no", "", "payment reference number (ESR/QR reference)")
	fl.StringVar(&f.message, "message", "", "message to the receiver")
}

func (f *outgoingPaymentFieldFlags) apply(cmd *cobra.Command, fields map[string]any) {
	setIfChanged(cmd, fields, "bill-id", "bill_id", f.billID)
	setIfChanged(cmd, fields, "payment-type", "payment_type", strings.ToUpper(f.paymentType))
	setIfChanged(cmd, fields, "execution-date", "execution_date", f.executionDate)
	setIfChanged(cmd, fields, "amount", "amount", f.amount)
	setIfChanged(cmd, fields, "currency-code", "currency_code", f.currencyCode)
	setIfChanged(cmd, fields, "exchange-rate", "exchange_rate", f.exchangeRate)
	setIfChanged(cmd, fields, "note", "note", f.note)
	setIfChanged(cmd, fields, "sender-bank-account-id", "sender_bank_account_id", f.senderBankAccountID)
	setIfChanged(cmd, fields, "sender-iban", "sender_iban", f.senderIBAN)
	setIfChanged(cmd, fields, "sender-name", "sender_name", f.senderName)
	setIfChanged(cmd, fields, "sender-street", "sender_street", f.senderStreet)
	setIfChanged(cmd, fields, "sender-house-no", "sender_house_no", f.senderHouseNo)
	setIfChanged(cmd, fields, "sender-city", "sender_city", f.senderCity)
	setIfChanged(cmd, fields, "sender-postcode", "sender_postcode", f.senderPostcode)
	setIfChanged(cmd, fields, "sender-country-code", "sender_country_code", f.senderCountryCode)
	setIfChanged(cmd, fields, "sender-bc-no", "sender_bc_no", f.senderBcNo)
	setIfChanged(cmd, fields, "sender-bank-no", "sender_bank_no", f.senderBankNo)
	setIfChanged(cmd, fields, "sender-bank-name", "sender_bank_name", f.senderBankName)
	setIfChanged(cmd, fields, "receiver-account-no", "receiver_account_no", f.receiverAccountNo)
	setIfChanged(cmd, fields, "receiver-iban", "receiver_iban", f.receiverIBAN)
	setIfChanged(cmd, fields, "receiver-name", "receiver_name", f.receiverName)
	setIfChanged(cmd, fields, "receiver-street", "receiver_street", f.receiverStreet)
	setIfChanged(cmd, fields, "receiver-house-no", "receiver_house_no", f.receiverHouseNo)
	setIfChanged(cmd, fields, "receiver-city", "receiver_city", f.receiverCity)
	setIfChanged(cmd, fields, "receiver-postcode", "receiver_postcode", f.receiverPostcode)
	setIfChanged(cmd, fields, "receiver-country-code", "receiver_country_code", f.receiverCountryCode)
	setIfChanged(cmd, fields, "receiver-bc-no", "receiver_bc_no", f.receiverBcNo)
	setIfChanged(cmd, fields, "receiver-bank-no", "receiver_bank_no", f.receiverBankNo)
	setIfChanged(cmd, fields, "receiver-bank-name", "receiver_bank_name", f.receiverBankName)
	setIfChanged(cmd, fields, "fee-type", "fee_type", strings.ToUpper(f.feeType))
	setIfChanged(cmd, fields, "is-salary-payment", "is_salary_payment", f.isSalaryPayment)
	setIfChanged(cmd, fields, "reference-no", "reference_no", f.referenceNo)
	setIfChanged(cmd, fields, "message", "message", f.message)
	setIfChanged(cmd, fields, "booking-text", "booking_text", f.bookingText)
}

func newOutgoingPaymentListCmd() *cobra.Command {
	var opts api.PurchaseListOptions
	var billID string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the payments of a bill",
		Example: `  bexio outgoing-payment list --bill-id 176a1442-d66d-4907-b8c8-6dad090452a8`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			payments, err := client.ListOutgoingPayments(cmd.Context(), billID, opts)
			if err != nil {
				return err
			}
			return renderOutgoingPayments(cmd, payments)
		},
	}
	purchaseListFlags(cmd, &opts, "execution_date, amount, status")
	cmd.Flags().StringVar(&billID, "bill-id", "", "id of the bill whose payments to list (required)")
	_ = cmd.MarkFlagRequired("bill-id")
	return cmd
}

func newOutgoingPaymentViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show an outgoing payment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("outgoing payment", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetOutgoingPayment(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, outgoingPaymentDetailOrder)
		},
	}
}

func newOutgoingPaymentCreateCmd() *cobra.Command {
	var fields outgoingPaymentFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a payment for a bill",
		Long: `Create an outgoing payment. Required: --bill-id, --payment-type,
--execution-date, --amount, --currency-code, --exchange-rate and
--sender-bank-account-id (pass --exchange-rate 1 in the base currency).`,
		Example: `  bexio outgoing-payment create --bill-id 176a1442-d66d-4907-b8c8-6dad090452a8 \
      --payment-type IBAN --execution-date 2026-09-01 --amount 120.50 \
      --currency-code CHF --exchange-rate 1 --sender-bank-account-id 2 \
      --receiver-iban CH121234567812345678900 --receiver-name "bexio AG"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if cmd.Flags().Changed("payment-type") {
				if _, err := purchaseEnum("--payment-type", fields.paymentType, api.OutgoingPaymentTypes); err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("fee-type") {
				if _, err := purchaseEnum("--fee-type", fields.feeType, api.OutgoingPaymentFeeTypes); err != nil {
					return err
				}
			}
			payload := map[string]any{}
			fields.apply(cmd, payload)
			payload["is_salary_payment"] = fields.isSalaryPayment
			if err := requirePurchaseFields(payload, [][2]string{
				{"--bill-id", "bill_id"},
				{"--payment-type", "payment_type"},
				{"--execution-date", "execution_date"},
				{"--amount", "amount"},
				{"--currency-code", "currency_code"},
				{"--exchange-rate", "exchange_rate"},
				{"--sender-bank-account-id", "sender_bank_account_id"},
			}); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.CreateOutgoingPayment(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created outgoing payment %s (%s %s, %s)\n",
				p.ID, p.CurrencyCode, purchaseAmount(p.Amount), p.Status)
			return nil
		},
	}
	fields.register(cmd, false)
	return cmd
}

func newOutgoingPaymentUpdateCmd() *cobra.Command {
	var fields outgoingPaymentFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an outgoing payment",
		Long: `Update an outgoing payment. The API replaces the payment with a PUT on
the collection path (the id travels as "payment_id" in the body), so the CLI
reads the current payment first and sends it back with your changes applied.`,
		Example: `  bexio outgoing-payment update 46913fdc-802b-49ba-99d7-4ccc13cccfc2 --execution-date 2026-09-15`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := purchaseID("outgoing payment", args[0])
			if err != nil {
				return err
			}
			changes := map[string]any{}
			fields.apply(cmd, changes)
			if len(changes) == 0 {
				return errNothingToUpdate
			}
			if cmd.Flags().Changed("fee-type") {
				if _, err := purchaseEnum("--fee-type", fields.feeType, api.OutgoingPaymentFeeTypes); err != nil {
					return err
				}
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			current, err := client.GetOutgoingPayment(cmd.Context(), id)
			if err != nil {
				return err
			}
			obj, err := decodePurchaseObject(current.Raw)
			if err != nil {
				return err
			}
			payload := pickPurchaseFields(obj, outgoingPaymentUpdateFields)
			payload["payment_id"] = id
			if payload["is_salary_payment"] == nil {
				payload["is_salary_payment"] = false
			}
			fields.apply(cmd, payload)
			p, err := client.UpdateOutgoingPayment(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated outgoing payment %s (%s %s)\n",
				p.ID, p.CurrencyCode, purchaseAmount(p.Amount))
			return nil
		},
	}
	fields.register(cmd, true)
	return cmd
}

func newOutgoingPaymentDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete an outgoing payment",
		Long:  "Permanently delete an outgoing payment. This cannot be undone, so --force is required.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := purchaseID("outgoing payment", args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting an outgoing payment is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteOutgoingPayment(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted outgoing payment %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}
