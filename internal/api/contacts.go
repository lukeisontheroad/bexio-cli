package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Contact holds the fields the CLI renders. Raw preserves the full API
// object for --output json.
type Contact struct {
	ID            int    `json:"id"`
	Nr            string `json:"nr"`
	ContactTypeID int    `json:"contact_type_id"`
	Name1         string `json:"name_1"`
	Name2         string `json:"name_2"`
	Mail          string `json:"mail"`
	PhoneFixed    string `json:"phone_fixed"`
	PhoneMobile   string `json:"phone_mobile"`
	Postcode      string `json:"postcode"`
	City          string `json:"city"`
	UpdatedAt     string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (c *Contact) UnmarshalJSON(data []byte) error {
	type contact Contact // avoid recursion
	var v contact
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*c = Contact(v)
	c.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// TypeName maps contact_type_id to its meaning (1 company, 2 person).
func (c Contact) TypeName() string {
	switch c.ContactTypeID {
	case 1:
		return "company"
	case 2:
		return "person"
	default:
		return fmt.Sprintf("type %d", c.ContactTypeID)
	}
}

// Name renders name_1 (+ name_2 if present). For persons name_1 is the last
// name and name_2 the first name; for companies name_2 is the name addition.
func (c Contact) Name() string {
	if c.Name2 == "" {
		return c.Name1
	}
	if c.ContactTypeID == 2 {
		return c.Name2 + " " + c.Name1
	}
	return c.Name1 + " " + c.Name2
}

// ListContacts fetches /2.0/contact.
func (c *Client) ListContacts(ctx context.Context, opts ListOptions) ([]Contact, error) {
	var out []Contact
	return out, c.Get(ctx, "/2.0/contact", opts.values(), &out)
}

// SearchContacts posts to /2.0/contact/search.
func (c *Client) SearchContacts(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Contact, error) {
	var out []Contact
	return out, c.Do(ctx, http.MethodPost, "/2.0/contact/search", opts.values(), criteria, &out)
}

// GetContact fetches a single contact.
func (c *Client) GetContact(ctx context.Context, id int, showArchived bool) (*Contact, error) {
	q := url.Values{}
	if showArchived {
		q.Set("show_archived", "true")
	}
	var out Contact
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/contact/%d", id), q, &out)
}

// CreateContact creates a contact. fields uses the raw API field names.
func (c *Client) CreateContact(ctx context.Context, fields map[string]any) (*Contact, error) {
	var out Contact
	return &out, c.Do(ctx, http.MethodPost, "/2.0/contact", nil, fields, &out)
}

// UpdateContact edits a contact (only the provided fields change).
func (c *Client) UpdateContact(ctx context.Context, id int, fields map[string]any) (*Contact, error) {
	var out Contact
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/contact/%d", id), nil, fields, &out)
}

// DeleteContact archives a contact (restorable via RestoreContact).
func (c *Client) DeleteContact(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/contact/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete contact %d: API reported failure", id)
	}
	return nil
}

// RestoreContact restores a deleted contact.
func (c *Client) RestoreContact(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodPatch, fmt.Sprintf("/2.0/contact/%d/restore", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("restore contact %d: API reported failure", id)
	}
	return nil
}
