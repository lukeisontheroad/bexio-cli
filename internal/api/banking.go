package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// BankingPaymentSender is the debited bank account of a banking payment.
type BankingPaymentSender struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
	IBAN string `json:"iban"`
}

// BankingPaymentAddress is the postal address of a payment recipient.
type BankingPaymentAddress struct {
	StreetName  string `json:"street_name"`
	HouseNumber string `json:"house_number"`
	Zip         string `json:"zip"`
	City        string `json:"city"`
	CountryCode string `json:"country_code"`
}

// BankingPaymentRecipient is the credited party of a banking payment.
type BankingPaymentRecipient struct {
	Name    string                `json:"name"`
	IBAN    string                `json:"iban"`
	Address BankingPaymentAddress `json:"address"`
}

// BankingPayment is an outgoing bank payment order (API resource
// "/4.0/banking/payments"). These objects instruct the bank to move money
// out of the company account once they are transmitted. Raw preserves the
// full API object.
//
// Note: the resource is addressed by its uuid, not by the numeric id.
type BankingPayment struct {
	ID                    int                     `json:"id"`
	UUID                  string                  `json:"uuid"`
	Sender                BankingPaymentSender    `json:"sender"`
	Recipient             BankingPaymentRecipient `json:"recipient"`
	Amount                json.Number             `json:"amount"`
	Currency              string                  `json:"currency"`
	ExecutionDate         string                  `json:"execution_date"`
	DueDate               string                  `json:"due_date"`
	Allowance             string                  `json:"allowance"`
	IsSalary              bool                    `json:"is_salary"`
	InstructionID         string                  `json:"instruction_id"`
	DocumentNo            string                  `json:"document_no"`
	QRReferenceNumber     string                  `json:"qr_reference_number"`
	AdditionalInformation string                  `json:"additional_information"`
	Status                string                  `json:"status"`
	Type                  string                  `json:"type"`
	CreatedAt             string                  `json:"created_at"`
	IsEditingRestricted   bool                    `json:"is_editing_restricted"`

	Raw json.RawMessage `json:"-"`
}

func (p *BankingPayment) UnmarshalJSON(data []byte) error {
	type bankingPayment BankingPayment
	var v bankingPayment
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = BankingPayment(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// BankingPaymentStatuses are the values of the payment status enum. Only
// "open" payments can still be updated, deleted, or cancelled.
var BankingPaymentStatuses = []string{"open", "transmitted", "downloaded", "paid", "failed", "cancelled"}

// BankingPaymentTypes are the values of the payment type enum.
var BankingPaymentTypes = []string{"iban", "qr"}

// BankingPaymentAllowances are the values of the fee allowance enum (used
// for cross-border or foreign-currency payments).
var BankingPaymentAllowances = []string{"fee_paid_by_payer", "fee_paid_by_payee", "fee_split", "no_fee"}

// StatusName returns the payment status. The 4.0 API already reports a
// string enum (unlike the kb_* documents with their numeric status ids);
// this normalizes the empty/unknown cases for display.
func (p BankingPayment) StatusName() string {
	if p.Status == "" {
		return "unknown"
	}
	for _, s := range BankingPaymentStatuses {
		if s == p.Status {
			return s
		}
	}
	return p.Status + " (unknown)"
}

// BankingPaymentListOptions are the list parameters of GET
// /4.0/banking/payments. The 4.0 API paginates with page/per-page (not
// order_by/limit/offset) and filters with a single "filter-by" string.
type BankingPaymentListOptions struct {
	// FilterBy is the raw filter-by value, e.g.
	// "status:open;execution_date:2026-01-01_2026-12-31".
	FilterBy string
	Page     int
	PerPage  int
}

func (o BankingPaymentListOptions) values() url.Values {
	q := url.Values{}
	if o.FilterBy != "" {
		q.Set("filter-by", o.FilterBy)
	}
	if o.Page > 0 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.PerPage > 0 {
		q.Set("per-page", strconv.Itoa(o.PerPage))
	}
	return q
}

const bankingPaymentsPath = "/4.0/banking/payments"

// bankingPaymentPath addresses a single payment by uuid. The id is escaped
// because it comes from user input.
func bankingPaymentPath(id string) string {
	return bankingPaymentsPath + "/" + url.PathEscape(id)
}

// ListBankingPayments lists payment orders. The spec documents the 200 body
// as a single object, but 4.0 list endpoints ship either a bare array or a
// {"data": [...], "paging": {...}} envelope, so all three shapes are
// accepted; the item Raw is preserved either way.
func (c *Client) ListBankingPayments(ctx context.Context, opts BankingPaymentListOptions) ([]BankingPayment, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, bankingPaymentsPath, opts.values(), &raw); err != nil {
		return nil, err
	}
	return decodeBankingPaymentList(raw)
}

func decodeBankingPaymentList(raw json.RawMessage) ([]BankingPayment, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	if data[0] == '[' {
		var out []BankingPayment
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return out, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if items, ok := envelope["data"]; ok {
		var out []BankingPayment
		if err := json.Unmarshal(items, &out); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return out, nil
	}
	// A single object (what the OpenAPI document literally declares).
	var one BankingPayment
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return []BankingPayment{one}, nil
}

func (c *Client) GetBankingPayment(ctx context.Context, id string) (*BankingPayment, error) {
	var out BankingPayment
	return &out, c.Get(ctx, bankingPaymentPath(id), nil, &out)
}

// CreateBankingPayment registers a payment order with the bank. fields must
// carry type, account_id, recipient, amount, currency, execution_date and
// is_salary (see the "iban" and "qr" variants of PaymentCreate).
//
// This is a real transfer instruction, not a bookkeeping entry.
func (c *Client) CreateBankingPayment(ctx context.Context, fields map[string]any) (*BankingPayment, error) {
	var out BankingPayment
	return &out, c.Do(ctx, http.MethodPost, bankingPaymentsPath, nil, fields, &out)
}

// UpdateBankingPayment changes a pending payment order. The 4.0 API uses
// PUT here (not the POST of the 2.0 endpoints); fields not sent are left
// untouched, but a "recipient" object must always be complete.
func (c *Client) UpdateBankingPayment(ctx context.Context, id string, fields map[string]any) (*BankingPayment, error) {
	var out BankingPayment
	return &out, c.Do(ctx, http.MethodPut, bankingPaymentPath(id), nil, fields, &out)
}

// DeleteBankingPayment permanently removes a payment order.
func (c *Client) DeleteBankingPayment(ctx context.Context, id string) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, bankingPaymentPath(id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete payment %s: API reported failure", id)
	}
	return nil
}

// CancelBankingPayment revokes a payment order that was already handed to
// the bank. It takes no body and returns the updated payment.
func (c *Client) CancelBankingPayment(ctx context.Context, id string) (*BankingPayment, error) {
	var out BankingPayment
	return &out, c.Do(ctx, http.MethodPost, bankingPaymentPath(id)+"/cancel", nil, nil, &out)
}
