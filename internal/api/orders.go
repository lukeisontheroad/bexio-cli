package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Order is a sales order (API resource "kb_order"). Raw preserves the full
// API object (including embedded positions on single fetches).
type Order struct {
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
	UpdatedAt      string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (o *Order) UnmarshalJSON(data []byte) error {
	type order Order
	var v order
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = Order(v)
	o.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// StatusName maps kb_item_status_id to its meaning.
func (o Order) StatusName() string {
	switch o.KbItemStatusID {
	case 5:
		return "pending"
	case 6:
		return "done"
	case 15:
		return "partial"
	case 21:
		return "canceled"
	default:
		return fmt.Sprintf("status %d", o.KbItemStatusID)
	}
}

func (c *Client) ListOrders(ctx context.Context, opts ListOptions) ([]Order, error) {
	var out []Order
	return out, c.Get(ctx, "/2.0/kb_order", opts.values(), &out)
}

func (c *Client) SearchOrders(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Order, error) {
	var out []Order
	return out, c.Do(ctx, http.MethodPost, "/2.0/kb_order/search", opts.values(), criteria, &out)
}

func (c *Client) GetOrder(ctx context.Context, id int) (*Order, error) {
	var out Order
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/kb_order/%d", id), nil, &out)
}

func (c *Client) CreateOrder(ctx context.Context, fields map[string]any) (*Order, error) {
	var out Order
	return &out, c.Do(ctx, http.MethodPost, "/2.0/kb_order", nil, fields, &out)
}

func (c *Client) UpdateOrder(ctx context.Context, id int, fields map[string]any) (*Order, error) {
	var out Order
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_order/%d", id), nil, fields, &out)
}

// DeleteOrder permanently deletes an order. Unlike contacts, this cannot be
// undone.
func (c *Client) DeleteOrder(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/kb_order/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete order %d: API reported failure", id)
	}
	return nil
}

// PDF is the response of the document PDF endpoints (base64 content).
type PDF struct {
	Name    string `json:"name"`
	Size    int    `json:"size"`
	Mime    string `json:"mime"`
	Content string `json:"content"`
}

// OrderPDF renders the order as PDF. logopaper: -1 = server default,
// 0 = plain, 1 = letterhead.
func (c *Client) OrderPDF(ctx context.Context, id, logopaper int) (*PDF, error) {
	q := url.Values{}
	if logopaper >= 0 {
		q.Set("logopaper", fmt.Sprint(logopaper))
	}
	var out PDF
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/kb_order/%d/pdf", id), q, &out)
}

// CreateInvoiceFromOrder creates an invoice from the order. A nil body
// includes all positions.
func (c *Client) CreateInvoiceFromOrder(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_order/%d/invoice", id), nil, struct{}{}, &out)
}

// CreateDeliveryFromOrder creates a delivery from the order. A nil body
// includes all positions.
func (c *Client) CreateDeliveryFromOrder(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_order/%d/delivery", id), nil, struct{}{}, &out)
}

func repetitionPath(id int) string { return fmt.Sprintf("/2.0/kb_order/%d/repetition", id) }

func (c *Client) GetOrderRepetition(ctx context.Context, id int) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Get(ctx, repetitionPath(id), nil, &out)
}

func (c *Client) EditOrderRepetition(ctx context.Context, id int, body json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, repetitionPath(id), nil, body, &out)
}

func (c *Client) DeleteOrderRepetition(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, repetitionPath(id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete repetition of order %d: API reported failure", id)
	}
	return nil
}

// PositionEndpoints maps CLI position type names to their API resource.
// These endpoints exist under /2.0/{kb_document_type}/{document_id}/...
// with kb_document_type one of kb_offer, kb_order, kb_invoice.
var PositionEndpoints = map[string]string{
	"article":     "kb_position_article",
	"custom":      "kb_position_custom",
	"text":        "kb_position_text",
	"subtotal":    "kb_position_subtotal",
	"discount":    "kb_position_discount",
	"pagebreak":   "kb_position_pagebreak",
	"subposition": "kb_position_subposition",
}

func positionPath(docType string, docID int, posType string) (string, error) {
	res, ok := PositionEndpoints[posType]
	if !ok {
		return "", fmt.Errorf("unknown position type %q", posType)
	}
	return fmt.Sprintf("/2.0/%s/%d/%s", docType, docID, res), nil
}

func (c *Client) CreatePosition(ctx context.Context, docType string, docID int, posType string, fields map[string]any) (json.RawMessage, error) {
	p, err := positionPath(docType, docID, posType)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, p, nil, fields, &out)
}

func (c *Client) UpdatePosition(ctx context.Context, docType string, docID int, posType string, posID int, fields map[string]any) (json.RawMessage, error) {
	p, err := positionPath(docType, docID, posType)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	return out, c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%d", p, posID), nil, fields, &out)
}

func (c *Client) DeletePosition(ctx context.Context, docType string, docID int, posType string, posID int) error {
	p, err := positionPath(docType, docID, posType)
	if err != nil {
		return err
	}
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", p, posID), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete position %d: API reported failure", posID)
	}
	return nil
}
