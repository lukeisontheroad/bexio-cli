package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newBankingPaymentCmd) }

// bankingPaymentMoneyWarning is repeated in the Long help of every command
// that instructs, changes, or revokes a transfer.
const bankingPaymentMoneyWarning = `THIS MOVES REAL MONEY: a banking payment is a transfer order sent to your
bank, not a bookkeeping entry. Once the bank has executed it the money is
gone. Every write command therefore requires --force.`

// newBankingPaymentCmd manages outgoing bank transfer orders (API resource
// "/4.0/banking/payments"). Named banking-payment to keep it apart from
// "kb-invoice payment", which only records incoming payments on an invoice.
func newBankingPaymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "banking-payment",
		// Deliberately no "payment" alias: `kb-invoice payment` records a
		// payment received on an invoice, while these commands instruct a
		// real outgoing bank transfer. The names must not be confusable.
		Aliases: []string{"bank-payment"},
		Short:   "List and manage outgoing bank payment orders (instructs real transfers)",
		Long: `List and manage outgoing bank payment orders (/4.0/banking/payments).

` + bankingPaymentMoneyWarning + `

Payments are addressed by their uuid (the "uuid" field), not by the numeric
"id". Only payments in status "open" can be updated or deleted.`,
	}
	cmd.AddCommand(
		newBankingPaymentListCmd(),
		newBankingPaymentViewCmd(),
		newBankingPaymentCreateCmd(),
		newBankingPaymentUpdateCmd(),
		newBankingPaymentDeleteCmd(),
		newBankingPaymentCancelCmd(),
	)
	return cmd
}

var bankingPaymentDetailOrder = []string{
	"uuid", "id", "status", "type", "amount", "currency", "execution_date",
	"due_date", "sender", "recipient", "allowance", "is_salary",
	"instruction_id", "document_no", "purchase_reference",
	"qr_reference_number", "additional_information", "message",
	"is_editing_restricted", "created_at",
}

// parseBankingPaymentID validates the uuid path argument (payments are not
// addressed by a numeric id, so parseID does not apply).
func parseBankingPaymentID(arg string) (string, error) {
	id := strings.TrimSpace(arg)
	if id == "" || strings.ContainsAny(id, "/?# ") {
		return "", fmt.Errorf("invalid payment id %q (expected the payment uuid)", arg)
	}
	return id, nil
}

func renderBankingPayments(cmd *cobra.Command, payments []api.BankingPayment) error {
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
			p.UUID,
			strconv.Itoa(p.ID),
			p.StatusName(),
			p.Type,
			p.Amount.String(),
			p.Currency,
			output.Truncate(p.Recipient.Name, 30),
			p.Recipient.IBAN,
			p.ExecutionDate,
		}
	}
	output.Table(cmd.OutOrStdout(), []string{
		"uuid", "id", "status", "type", "amount", "currency", "recipient",
		"iban", "execution_date",
	}, rows)
	return nil
}

// bankingPaymentSummary describes what a payment instructs, for the
// confirmation lines printed after a write.
func bankingPaymentSummary(p *api.BankingPayment) string {
	to := p.Recipient.Name
	if p.Recipient.IBAN != "" {
		to = strings.TrimSpace(to + " " + p.Recipient.IBAN)
	}
	if to == "" {
		to = "the recipient"
	}
	when := p.ExecutionDate
	if when == "" {
		when = "the execution date"
	}
	return fmt.Sprintf("%s %s to %s on %s", p.Amount.String(), p.Currency, to, when)
}

// bankingPaymentFieldFlags mirrors the writable fields of PaymentCreate /
// PaymentUpdate. The nested recipient object is flattened into
// --recipient-* flags; --json takes a raw body for anything not modelled.
type bankingPaymentFieldFlags struct {
	paymentType           string
	accountID             string
	amount                string
	currency              string
	executionDate         string
	isSalary              bool
	allowance             string
	recipientName         string
	recipientIBAN         string
	streetName            string
	houseNumber           string
	zip                   string
	city                  string
	countryCode           string
	qrReferenceNumber     string
	additionalInformation string
	message               string
	isEditingRestricted   bool
	billID                string
	billPaymentID         string
	raw                   string
}

// register adds the field flags. create-only fields (type, account_id and
// the purchase reference) are omitted for update, which the API does not
// accept there.
func (f *bankingPaymentFieldFlags) register(cmd *cobra.Command, create bool) {
	fl := cmd.Flags()
	if create {
		fl.StringVar(&f.paymentType, "type", "", "payment type: "+strings.Join(api.BankingPaymentTypes, " or ")+` ("qr" additionally needs --qr-reference-number)`)
		fl.StringVar(&f.accountID, "account-id", "", "uuid of the bank account to debit (required)")
	}
	fl.StringVar(&f.amount, "amount", "", "amount to transfer, decimal (required)")
	fl.StringVar(&f.currency, "currency", "", "currency, ISO 4217, e.g. CHF (required)")
	fl.StringVar(&f.executionDate, "execution-date", "", "date the bank should execute the payment, YYYY-MM-DD (required)")
	fl.BoolVar(&f.isSalary, "is-salary", false, "flag the transfer as a salary payment")
	fl.StringVar(&f.allowance, "allowance", "", "fee handling for cross-border/foreign-currency payments: "+strings.Join(api.BankingPaymentAllowances, ", "))
	fl.StringVar(&f.recipientName, "recipient-name", "", "account holder receiving the money (required)")
	fl.StringVar(&f.recipientIBAN, "recipient-iban", "", "IBAN receiving the money (required)")
	fl.StringVar(&f.streetName, "recipient-address-street-name", "", "recipient street")
	fl.StringVar(&f.houseNumber, "recipient-address-house-number", "", "recipient house number")
	fl.StringVar(&f.zip, "recipient-address-zip", "", "recipient zip code")
	fl.StringVar(&f.city, "recipient-address-city", "", "recipient city")
	fl.StringVar(&f.countryCode, "recipient-address-country-code", "", "recipient country, ISO 3166-1 alpha-2 (default CH)")
	fl.StringVar(&f.qrReferenceNumber, "qr-reference-number", "", "QR or SCOR reference number (required for --type qr)")
	fl.StringVar(&f.additionalInformation, "additional-information", "", "additional information on the payment slip")
	fl.StringVar(&f.message, "message", "", "message to the recipient")
	fl.BoolVar(&f.isEditingRestricted, "is-editing-restricted", false, "restrict later edits to the API client that created the payment")
	if create {
		fl.StringVar(&f.billID, "purchase-reference-bill-id", "", "uuid of the purchase bill this payment settles")
		fl.StringVar(&f.billPaymentID, "purchase-reference-bill-payment-id", "", "uuid of the bill payment this payment settles")
	}
	fl.StringVar(&f.raw, "json", "", "raw JSON request body (merged first; the flags above override it)")
}

// recipient assembles the nested recipient object when any recipient flag
// was given. The API requires name, iban and a complete address whenever a
// recipient is sent, so this is all-or-nothing.
func (f *bankingPaymentFieldFlags) recipient(cmd *cobra.Command) map[string]any {
	address := map[string]any{}
	setIfChanged(cmd, address, "recipient-address-street-name", "street_name", f.streetName)
	setIfChanged(cmd, address, "recipient-address-house-number", "house_number", f.houseNumber)
	setIfChanged(cmd, address, "recipient-address-zip", "zip", f.zip)
	setIfChanged(cmd, address, "recipient-address-city", "city", f.city)
	setIfChanged(cmd, address, "recipient-address-country-code", "country_code", f.countryCode)

	recipient := map[string]any{}
	setIfChanged(cmd, recipient, "recipient-name", "name", f.recipientName)
	setIfChanged(cmd, recipient, "recipient-iban", "iban", f.recipientIBAN)
	if len(address) > 0 {
		recipient["address"] = address
	}
	if len(recipient) == 0 {
		return nil
	}
	return recipient
}

// payload builds the request body: the --json body first, then every flag
// the user actually passed on top.
func (f *bankingPaymentFieldFlags) payload(cmd *cobra.Command) (map[string]any, error) {
	fields := map[string]any{}
	if f.raw != "" {
		dec := json.NewDecoder(bytes.NewReader([]byte(f.raw)))
		dec.UseNumber()
		if err := dec.Decode(&fields); err != nil {
			return nil, fmt.Errorf("invalid --json body: %w", err)
		}
	}
	setIfChanged(cmd, fields, "type", "type", f.paymentType)
	setIfChanged(cmd, fields, "account-id", "account_id", f.accountID)
	if cmd.Flags().Changed("amount") {
		amount, err := parseBankingPaymentAmount(f.amount)
		if err != nil {
			return nil, err
		}
		fields["amount"] = amount
	}
	setIfChanged(cmd, fields, "currency", "currency", f.currency)
	setIfChanged(cmd, fields, "execution-date", "execution_date", f.executionDate)
	setIfChanged(cmd, fields, "is-salary", "is_salary", f.isSalary)
	setIfChanged(cmd, fields, "allowance", "allowance", f.allowance)
	setIfChanged(cmd, fields, "qr-reference-number", "qr_reference_number", f.qrReferenceNumber)
	setIfChanged(cmd, fields, "additional-information", "additional_information", f.additionalInformation)
	setIfChanged(cmd, fields, "message", "message", f.message)
	setIfChanged(cmd, fields, "is-editing-restricted", "is_editing_restricted", f.isEditingRestricted)
	if recipient := f.recipient(cmd); recipient != nil {
		fields["recipient"] = recipient
	}
	purchase := map[string]any{}
	setIfChanged(cmd, purchase, "purchase-reference-bill-id", "bill_id", f.billID)
	setIfChanged(cmd, purchase, "purchase-reference-bill-payment-id", "bill_payment_id", f.billPaymentID)
	if len(purchase) > 0 {
		fields["purchase_reference"] = purchase
	}
	return fields, nil
}

// parseBankingPaymentAmount keeps the decimal exactly as typed (no float
// rounding) while rejecting anything that is not a positive number.
func parseBankingPaymentAmount(s string) (json.Number, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return "", fmt.Errorf("invalid --amount %q (want a decimal number like 1250.00)", s)
	}
	if v <= 0 {
		return "", fmt.Errorf("invalid --amount %q (must be greater than 0)", s)
	}
	return json.Number(strings.TrimSpace(s)), nil
}

// requireBankingPaymentFields checks the fields the API requires on create,
// so a typo fails locally instead of half-way through the bank's validation.
func requireBankingPaymentFields(fields map[string]any) error {
	var missing []string
	for _, k := range []string{"type", "account_id", "amount", "currency", "execution_date"} {
		if v, ok := fields[k]; !ok || v == "" {
			missing = append(missing, k)
		}
	}
	recipient, _ := fields["recipient"].(map[string]any)
	for _, k := range []string{"name", "iban"} {
		if v, ok := recipient[k]; !ok || v == "" {
			missing = append(missing, "recipient."+k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s (pass the matching flags or supply them via --json)", strings.Join(missing, ", "))
	}
	if t, _ := fields["type"].(string); t == "qr" {
		if v, ok := fields["qr_reference_number"]; !ok || v == "" {
			return fmt.Errorf("--type qr requires --qr-reference-number")
		}
	}
	return nil
}

func newBankingPaymentListCmd() *cobra.Command {
	var opts api.BankingPaymentListOptions
	var filters []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bank payment orders",
		Long: `List bank payment orders (GET /4.0/banking/payments).

Unlike the 2.0 endpoints this one paginates with --page/--per-page and
filters with --filter-by clauses of the form "field:value" (a range is
"field:from_to"; repeated flags are joined with ";").

Filterable fields: account_id, status, currency, execution_date, amount,
recipient.name, recipient.iban, document_no.
Statuses: ` + strings.Join(api.BankingPaymentStatuses, ", ") + `.`,
		Example: `  bexio banking-payment list
  bexio banking-payment list --filter-by status:open
  bexio banking-payment list --filter-by execution_date:2026-01-01_2026-12-31 --per-page 20 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if len(filters) > 0 {
				opts.FilterBy = strings.Join(filters, ";")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			payments, err := client.ListBankingPayments(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderBankingPayments(cmd, payments)
		},
	}
	cmd.Flags().StringArrayVar(&filters, "filter-by", nil, `filter clause "field:value" (repeatable, joined with ";")`)
	cmd.Flags().IntVar(&opts.Page, "page", 0, "page to fetch (0-based)")
	cmd.Flags().IntVar(&opts.PerPage, "per-page", 0, "results per page (API max 2000, default 500)")
	return cmd
}

func newBankingPaymentViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <uuid>",
		Short: "Show a bank payment order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseBankingPaymentID(args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetBankingPayment(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, bankingPaymentDetailOrder)
		},
	}
}

func newBankingPaymentCreateCmd() *cobra.Command {
	var fields bankingPaymentFieldFlags
	var force bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Instruct a bank transfer (requires --force)",
		Long: `Create a payment order (POST /4.0/banking/payments).

` + bankingPaymentMoneyWarning + `

The payment debits the bank account given by --account-id and credits
--recipient-iban on --execution-date (which must be at least the next
banking day). Required: --type, --account-id, --amount, --currency,
--execution-date, --recipient-name, --recipient-iban; the API also wants a
complete recipient address (street name, house number, zip, city, country
code). --type qr additionally requires --qr-reference-number.

Fields this CLI does not model can be supplied with --json '<body>'; the
flags above override matching keys of that body.`,
		Example: `  bexio banking-payment create --force --type iban \
      --account-id 0c295adb-91ff-4cd5-8a8c-009ee4330f69 \
      --amount 1250.00 --currency CHF --execution-date 2026-09-01 \
      --recipient-name "Bexio AG" --recipient-iban CH3000784295116252003 \
      --recipient-address-street-name Föhrenstrasse --recipient-address-house-number 34 \
      --recipient-address-zip 5003 --recipient-address-city Zürich
  bexio banking-payment create --force --json '{"type":"qr", ...}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("creating a payment instructs a real bank transfer: re-run with --force to confirm")
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if err := requireBankingPaymentFields(payload); err != nil {
				return err
			}
			if _, ok := payload["is_salary"]; !ok {
				payload["is_salary"] = false // required by the API
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.CreateBankingPayment(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created payment %s (status %s): transfers %s\n",
				p.UUID, p.StatusName(), bankingPaymentSummary(p))
			return nil
		},
	}
	fields.register(cmd, true)
	cmd.Flags().BoolVar(&force, "force", false, "confirm that a real bank transfer should be instructed")
	return cmd
}

func newBankingPaymentUpdateCmd() *cobra.Command {
	var fields bankingPaymentFieldFlags
	var force bool
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Change a pending bank transfer (requires --force)",
		Long: `Update a payment order (PUT /4.0/banking/payments/{uuid}).

` + bankingPaymentMoneyWarning + `

Only fields you pass are sent, and only payments in status "open" can be
changed. The payment type and the debited account cannot be changed — delete
the payment and create a new one instead. When any --recipient-* flag is
given the API expects the complete recipient (name, iban and full address),
so pass them all together.`,
		Example: `  bexio banking-payment update 4f0e… --force --amount 990.00
  bexio banking-payment update 4f0e… --force --execution-date 2026-09-15`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseBankingPaymentID(args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("changing a payment changes a real bank transfer: re-run with --force to confirm")
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag or --json")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.UpdateBankingPayment(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated payment %s (status %s): transfers %s\n",
				p.UUID, p.StatusName(), bankingPaymentSummary(p))
			return nil
		},
	}
	fields.register(cmd, false)
	cmd.Flags().BoolVar(&force, "force", false, "confirm that a real bank transfer should be changed")
	return cmd
}

func newBankingPaymentDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Permanently delete a bank payment order (requires --force)",
		Long: `Delete a payment order (DELETE /4.0/banking/payments/{uuid}).

Removes the transfer order before the bank executes it. This CANNOT be
undone, so --force is required. A payment already handed to the bank cannot
be deleted — use "banking-payment cancel".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseBankingPaymentID(args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("deleting a payment is permanent and cannot be undone: re-run with --force")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteBankingPayment(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted payment %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm the permanent deletion")
	return cmd
}

func newBankingPaymentCancelCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "cancel <uuid>",
		Short: "Cancel a bank payment order (requires --force)",
		Long: `Cancel a payment order (POST /4.0/banking/payments/{uuid}/cancel).

Revokes a transfer that was already transmitted to the bank. Whether the
bank still honours the revocation depends on how far the payment has
progressed, so --force is required. The payment ends up in status
"cancelled".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseBankingPaymentID(args[0])
			if err != nil {
				return err
			}
			if !force {
				return fmt.Errorf("cancelling a payment revokes a real bank transfer: re-run with --force to confirm")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.CancelBankingPayment(cmd.Context(), id)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cancelled payment %s (status %s)\n", id, p.StatusName())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "confirm that a real bank transfer should be revoked")
	return cmd
}
