package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Quote is a quote/offer (API resource "kb_offer"). Raw preserves the full
// API object (including embedded positions on single fetches).
type Quote struct {
	ID             int    `json:"id"`
	DocumentNr     string `json:"document_nr"`
	Title          string `json:"title"`
	ContactID      int    `json:"contact_id"`
	UserID         int    `json:"user_id"`
	Total          string `json:"total"`
	TotalNet       string `json:"total_net"`
	CurrencyID     int    `json:"currency_id"`
	KbItemStatusID int    `json:"kb_item_status_id"`
	IsValidFrom    string `json:"is_valid_from"`
	IsValidUntil   string `json:"is_valid_until"`
	UpdatedAt      string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (q *Quote) UnmarshalJSON(data []byte) error {
	type quote Quote
	var v quote
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*q = Quote(v)
	q.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// StatusName maps kb_item_status_id to its meaning (quote statuses differ
// from orders and invoices).
func (q Quote) StatusName() string {
	switch q.KbItemStatusID {
	case 1:
		return "draft"
	case 2:
		return "pending"
	case 3:
		return "confirmed"
	case 4:
		return "declined"
	default:
		return fmt.Sprintf("status %d", q.KbItemStatusID)
	}
}

func (c *Client) ListQuotes(ctx context.Context, opts ListOptions) ([]Quote, error) {
	var out []Quote
	return out, c.Get(ctx, "/2.0/kb_offer", opts.values(), &out)
}

func (c *Client) SearchQuotes(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Quote, error) {
	var out []Quote
	return out, c.Do(ctx, http.MethodPost, "/2.0/kb_offer/search", opts.values(), criteria, &out)
}

func (c *Client) GetQuote(ctx context.Context, id int) (*Quote, error) {
	var out Quote
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/kb_offer/%d", id), nil, &out)
}

func (c *Client) CreateQuote(ctx context.Context, fields map[string]any) (*Quote, error) {
	var out Quote
	return &out, c.Do(ctx, http.MethodPost, "/2.0/kb_offer", nil, fields, &out)
}

func (c *Client) UpdateQuote(ctx context.Context, id int, fields map[string]any) (*Quote, error) {
	var out Quote
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d", id), nil, fields, &out)
}

// DeleteQuote permanently deletes a quote. Unlike contacts, this cannot be
// undone.
func (c *Client) DeleteQuote(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/kb_offer/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete quote %d: API reported failure", id)
	}
	return nil
}

// QuotePDF renders the quote as PDF. logopaper: -1 = server default,
// 0 = plain, 1 = letterhead.
func (c *Client) QuotePDF(ctx context.Context, id, logopaper int) (*PDF, error) {
	q := url.Values{}
	if logopaper >= 0 {
		q.Set("logopaper", fmt.Sprint(logopaper))
	}
	var out PDF
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/kb_offer/%d/pdf", id), q, &out)
}

// quoteAction posts to a bodyless status-transition endpoint of the quote
// (issue, revertIssue, accept, reject, reissue, mark_as_sent) and checks the
// {"success": true} response.
func (c *Client) quoteAction(ctx context.Context, id int, action string) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d/%s", id, action), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("%s quote %d: API reported failure", action, id)
	}
	return nil
}

// IssueQuote issues a draft quote (status draft -> pending).
func (c *Client) IssueQuote(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "issue")
}

// RevertIssueQuote reverts an issued quote back to draft. Note the camelCase
// API path (/revertIssue), unlike every other kb_offer action.
func (c *Client) RevertIssueQuote(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "revertIssue")
}

// AcceptQuote marks a pending quote as accepted (status confirmed).
func (c *Client) AcceptQuote(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "accept")
}

// RejectQuote declines a pending quote (status declined).
func (c *Client) RejectQuote(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "reject")
}

// ReissueQuote reissues a confirmed or declined quote (back to pending).
func (c *Client) ReissueQuote(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "reissue")
}

// MarkQuoteAsSent marks the quote as sent without emailing it.
func (c *Client) MarkQuoteAsSent(ctx context.Context, id int) error {
	return c.quoteAction(ctx, id, "mark_as_sent")
}

// SendQuote emails the quote. body requires recipient_email, subject and
// message (the message must contain the "[Network Link]" placeholder);
// optional: mark_as_open, attach_pdf.
func (c *Client) SendQuote(ctx context.Context, id int, body map[string]any) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d/send", id), nil, body, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("send quote %d: API reported failure", id)
	}
	return nil
}

// CopyQuote copies the quote to a new one. fields requires contact_id;
// optional: contact_sub_id, is_valid_from, pr_project_id, title.
func (c *Client) CopyQuote(ctx context.Context, id int, fields map[string]any) (*Quote, error) {
	var out Quote
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d/copy", id), nil, fields, &out)
}

// CreateOrderFromQuote creates an order from the quote. A nil body includes
// all positions.
func (c *Client) CreateOrderFromQuote(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d/order", id), nil, struct{}{}, &out)
}

// CreateInvoiceFromQuote creates an invoice from the quote. A nil body
// includes all positions.
func (c *Client) CreateInvoiceFromQuote(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_offer/%d/invoice", id), nil, struct{}{}, &out)
}
