package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Invoice is a customer invoice (API resource "kb_invoice"). Raw preserves
// the full API object (including embedded positions on single fetches).
type Invoice struct {
	ID                     int    `json:"id"`
	DocumentNr             string `json:"document_nr"`
	Title                  string `json:"title"`
	ContactID              int    `json:"contact_id"`
	UserID                 int    `json:"user_id"`
	Total                  string `json:"total"`
	TotalNet               string `json:"total_net"`
	TotalReceivedPayments  string `json:"total_received_payments"`
	TotalRemainingPayments string `json:"total_remaining_payments"`
	CurrencyID             int    `json:"currency_id"`
	KbItemStatusID         int    `json:"kb_item_status_id"`
	IsValidFrom            string `json:"is_valid_from"`
	IsValidTo              string `json:"is_valid_to"`
	UpdatedAt              string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (i *Invoice) UnmarshalJSON(data []byte) error {
	type invoice Invoice
	var v invoice
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*i = Invoice(v)
	i.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// StatusName maps kb_item_status_id to its meaning. Invoice status ids
// differ from orders and offers.
func (i Invoice) StatusName() string {
	switch i.KbItemStatusID {
	case 7:
		return "draft"
	case 8:
		return "pending"
	case 9:
		return "paid"
	case 16:
		return "partial"
	case 19:
		return "canceled"
	case 31:
		return "unpaid"
	default:
		return fmt.Sprintf("status %d", i.KbItemStatusID)
	}
}

func invoicePath(id int) string { return fmt.Sprintf("/2.0/kb_invoice/%d", id) }

func (c *Client) ListInvoices(ctx context.Context, opts ListOptions) ([]Invoice, error) {
	var out []Invoice
	return out, c.Get(ctx, "/2.0/kb_invoice", opts.values(), &out)
}

func (c *Client) SearchInvoices(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Invoice, error) {
	var out []Invoice
	return out, c.Do(ctx, http.MethodPost, "/2.0/kb_invoice/search", opts.values(), criteria, &out)
}

func (c *Client) GetInvoice(ctx context.Context, id int) (*Invoice, error) {
	var out Invoice
	return &out, c.Get(ctx, invoicePath(id), nil, &out)
}

func (c *Client) CreateInvoice(ctx context.Context, fields map[string]any) (*Invoice, error) {
	var out Invoice
	return &out, c.Do(ctx, http.MethodPost, "/2.0/kb_invoice", nil, fields, &out)
}

func (c *Client) UpdateInvoice(ctx context.Context, id int, fields map[string]any) (*Invoice, error) {
	var out Invoice
	return &out, c.Do(ctx, http.MethodPost, invoicePath(id), nil, fields, &out)
}

// DeleteInvoice permanently deletes an invoice. Unlike contacts, this cannot
// be undone.
func (c *Client) DeleteInvoice(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, invoicePath(id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete invoice %d: API reported failure", id)
	}
	return nil
}

// InvoicePDF renders the invoice as PDF. logopaper: -1 = server default,
// 0 = plain, 1 = letterhead.
func (c *Client) InvoicePDF(ctx context.Context, id, logopaper int) (*PDF, error) {
	q := url.Values{}
	if logopaper >= 0 {
		q.Set("logopaper", fmt.Sprint(logopaper))
	}
	var out PDF
	return &out, c.Get(ctx, invoicePath(id)+"/pdf", q, &out)
}

// CopyInvoice copies the invoice to a new draft. fields must contain
// contact_id; contact_sub_id, title, and is_valid_from are optional.
func (c *Client) CopyInvoice(ctx context.Context, id int, fields map[string]any) (*Invoice, error) {
	var out Invoice
	return &out, c.Do(ctx, http.MethodPost, invoicePath(id)+"/copy", nil, fields, &out)
}

// invoiceAction performs one of the bodyless {"success": bool} actions.
func (c *Client) invoiceAction(ctx context.Context, path, verb string, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("%s invoice %d: API reported failure", verb, id)
	}
	return nil
}

// IssueInvoice moves a draft invoice to pending.
func (c *Client) IssueInvoice(ctx context.Context, id int) error {
	return c.invoiceAction(ctx, invoicePath(id)+"/issue", "issue", id)
}

// RevertIssueInvoice sets an issued invoice back to draft.
func (c *Client) RevertIssueInvoice(ctx context.Context, id int) error {
	return c.invoiceAction(ctx, invoicePath(id)+"/revert_issue", "revert issue of", id)
}

func (c *Client) CancelInvoice(ctx context.Context, id int) error {
	return c.invoiceAction(ctx, invoicePath(id)+"/cancel", "cancel", id)
}

func (c *Client) MarkInvoiceAsSent(ctx context.Context, id int) error {
	return c.invoiceAction(ctx, invoicePath(id)+"/mark_as_sent", "mark as sent", id)
}

// SendInvoice emails the invoice. body carries recipient_email, subject,
// message (all required by the API; message must contain "[Network Link]"),
// and optionally mark_as_open and attach_pdf.
func (c *Client) SendInvoice(ctx context.Context, id int, body map[string]any) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, invoicePath(id)+"/send", nil, body, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("send invoice %d: API reported failure", id)
	}
	return nil
}

// Payment is a payment on an invoice
// (API resource "kb_invoice/{id}/payment").
type Payment struct {
	ID            int    `json:"id"`
	Date          string `json:"date"`
	Value         string `json:"value"`
	BankAccountID int    `json:"bank_account_id"`
	KbInvoiceID   int    `json:"kb_invoice_id"`

	Raw json.RawMessage `json:"-"`
}

func (p *Payment) UnmarshalJSON(data []byte) error {
	type payment Payment
	var v payment
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = Payment(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func invoicePaymentPath(invoiceID int) string { return invoicePath(invoiceID) + "/payment" }

func (c *Client) ListInvoicePayments(ctx context.Context, invoiceID int, opts ListOptions) ([]Payment, error) {
	var out []Payment
	return out, c.Get(ctx, invoicePaymentPath(invoiceID), opts.values(), &out)
}

// CreateInvoicePayment records a payment. fields must contain value;
// date, bank_account_id, and payment_service_id are optional.
func (c *Client) CreateInvoicePayment(ctx context.Context, invoiceID int, fields map[string]any) (*Payment, error) {
	var out Payment
	return &out, c.Do(ctx, http.MethodPost, invoicePaymentPath(invoiceID), nil, fields, &out)
}

func (c *Client) GetInvoicePayment(ctx context.Context, invoiceID, paymentID int) (*Payment, error) {
	var out Payment
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", invoicePaymentPath(invoiceID), paymentID), nil, &out)
}

func (c *Client) DeleteInvoicePayment(ctx context.Context, invoiceID, paymentID int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", invoicePaymentPath(invoiceID), paymentID), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete payment %d of invoice %d: API reported failure", paymentID, invoiceID)
	}
	return nil
}

// Reminder is a payment reminder of an invoice
// (API resource "kb_invoice/{id}/kb_reminder").
type Reminder struct {
	ID             int    `json:"id"`
	KbInvoiceID    int    `json:"kb_invoice_id"`
	Title          string `json:"title"`
	IsValidFrom    string `json:"is_valid_from"`
	IsValidTo      string `json:"is_valid_to"`
	ReminderLevel  int    `json:"reminder_level"`
	IsSent         bool   `json:"is_sent"`
	RemainingPrice string `json:"remaining_price"`

	Raw json.RawMessage `json:"-"`
}

func (r *Reminder) UnmarshalJSON(data []byte) error {
	type reminder Reminder
	var v reminder
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = Reminder(v)
	r.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func invoiceReminderPath(invoiceID int) string { return invoicePath(invoiceID) + "/kb_reminder" }

func (c *Client) ListInvoiceReminders(ctx context.Context, invoiceID int, opts ListOptions) ([]Reminder, error) {
	var out []Reminder
	return out, c.Get(ctx, invoiceReminderPath(invoiceID), opts.values(), &out)
}

// CreateInvoiceReminder creates the next reminder (the API increments the
// reminder level automatically). All fields are optional.
func (c *Client) CreateInvoiceReminder(ctx context.Context, invoiceID int, fields map[string]any) (*Reminder, error) {
	var out Reminder
	return &out, c.Do(ctx, http.MethodPost, invoiceReminderPath(invoiceID), nil, fields, &out)
}

func (c *Client) SearchInvoiceReminders(ctx context.Context, invoiceID int, criteria []SearchCriterion, opts ListOptions) ([]Reminder, error) {
	var out []Reminder
	return out, c.Do(ctx, http.MethodPost, invoiceReminderPath(invoiceID)+"/search", opts.values(), criteria, &out)
}

func (c *Client) GetInvoiceReminder(ctx context.Context, invoiceID, reminderID int) (*Reminder, error) {
	var out Reminder
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", invoiceReminderPath(invoiceID), reminderID), nil, &out)
}

func (c *Client) DeleteInvoiceReminder(ctx context.Context, invoiceID, reminderID int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", invoiceReminderPath(invoiceID), reminderID), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete reminder %d of invoice %d: API reported failure", reminderID, invoiceID)
	}
	return nil
}

// reminderAction performs one of the bodyless {"success": bool} reminder
// actions.
func (c *Client) reminderAction(ctx context.Context, invoiceID, reminderID int, action, verb string) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%d/%s", invoiceReminderPath(invoiceID), reminderID, action), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("%s reminder %d of invoice %d: API reported failure", verb, reminderID, invoiceID)
	}
	return nil
}

func (c *Client) MarkInvoiceReminderAsSent(ctx context.Context, invoiceID, reminderID int) error {
	return c.reminderAction(ctx, invoiceID, reminderID, "mark_as_sent", "mark as sent")
}

func (c *Client) MarkInvoiceReminderAsUnsent(ctx context.Context, invoiceID, reminderID int) error {
	return c.reminderAction(ctx, invoiceID, reminderID, "mark_as_unsent", "mark as unsent")
}

// SendInvoiceReminder emails the reminder. body carries recipient_email,
// subject, and message (all required; message must contain "[Network Link]").
func (c *Client) SendInvoiceReminder(ctx context.Context, invoiceID, reminderID int, body map[string]any) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%d/send", invoiceReminderPath(invoiceID), reminderID), nil, body, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("send reminder %d of invoice %d: API reported failure", reminderID, invoiceID)
	}
	return nil
}

// InvoiceReminderPDF renders the reminder as PDF. logopaper: -1 = server
// default, 0 = plain, 1 = letterhead.
func (c *Client) InvoiceReminderPDF(ctx context.Context, invoiceID, reminderID, logopaper int) (*PDF, error) {
	q := url.Values{}
	if logopaper >= 0 {
		q.Set("logopaper", fmt.Sprint(logopaper))
	}
	var out PDF
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d/pdf", invoiceReminderPath(invoiceID), reminderID), q, &out)
}
