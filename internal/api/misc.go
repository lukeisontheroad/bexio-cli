package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// This file covers the remaining small read-only resources — communication
// types, document settings, document templates — plus the bulk contact
// creation endpoint.

// CommunicationKind is a communication type (API resource
// "communication_kind", 2.0, read-only).
type CommunicationKind struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (k *CommunicationKind) UnmarshalJSON(data []byte) error {
	type communicationKind CommunicationKind // avoid recursion
	var v communicationKind
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*k = CommunicationKind(v)
	k.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListCommunicationKinds fetches /2.0/communication_kind.
func (c *Client) ListCommunicationKinds(ctx context.Context, opts ListOptions) ([]CommunicationKind, error) {
	var out []CommunicationKind
	return out, c.Get(ctx, "/2.0/communication_kind", opts.values(), &out)
}

// SearchCommunicationKinds posts to /2.0/communication_kind/search (the
// only searchable field is name).
func (c *Client) SearchCommunicationKinds(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]CommunicationKind, error) {
	var out []CommunicationKind
	return out, c.Do(ctx, http.MethodPost, "/2.0/communication_kind/search", opts.values(), criteria, &out)
}

// DocumentSetting is the numbering/default configuration of one document
// type (API resource "kb_item_setting", 2.0, read-only).
type DocumentSetting struct {
	ID                      int    `json:"id"`
	Text                    string `json:"text"`
	KbItemClass             string `json:"kb_item_class"`
	EnumerationFormat       string `json:"enumeration_format"`
	UseAutomaticEnumeration bool   `json:"use_automatic_enumeration"`
	UseYearlyEnumeration    bool   `json:"use_yearly_enumeration"`
	NextNr                  int    `json:"next_nr"`
	NrMinLength             int    `json:"nr_min_length"`
	DefaultTimePeriodInDays int    `json:"default_time_period_in_days"`
	DefaultCurrencyID       int    `json:"default_currency_id"`
	DefaultLanguageID       int    `json:"default_language_id"`
	DefaultTitle            string `json:"default_title"`

	Raw json.RawMessage `json:"-"`
}

func (s *DocumentSetting) UnmarshalJSON(data []byte) error {
	type documentSetting DocumentSetting
	var v documentSetting
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = DocumentSetting(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListDocumentSettings fetches /2.0/kb_item_setting (one entry per document
// type). The endpoint supports order_by only — no limit/offset.
func (c *Client) ListDocumentSettings(ctx context.Context, orderBy string) ([]DocumentSetting, error) {
	q := url.Values{}
	if orderBy != "" {
		q.Set("order_by", orderBy)
	}
	var out []DocumentSetting
	return out, c.Get(ctx, "/2.0/kb_item_setting", q, &out)
}

// DocumentTemplate is a PDF document template (API resource
// "document_templates", 3.0, read-only). Templates are identified by a slug,
// not a numeric id.
type DocumentTemplate struct {
	TemplateSlug            string   `json:"template_slug"`
	Name                    string   `json:"name"`
	IsDefault               bool     `json:"is_default"`
	DefaultForDocumentTypes []string `json:"default_for_document_types"`

	Raw json.RawMessage `json:"-"`
}

func (t *DocumentTemplate) UnmarshalJSON(data []byte) error {
	type documentTemplate DocumentTemplate
	var v documentTemplate
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = DocumentTemplate(v)
	t.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListDocumentTemplates fetches /3.0/document_templates (no parameters).
func (c *Client) ListDocumentTemplates(ctx context.Context) ([]DocumentTemplate, error) {
	var out []DocumentTemplate
	return out, c.Get(ctx, "/3.0/document_templates", nil, &out)
}

// BulkCreateContacts creates several contacts in one request
// (POST /2.0/contact/_bulk_create) and returns the created contacts. Each
// entry uses the raw API field names of a contact; contact_type_id, name_1,
// user_id, and owner_id are required per entry.
func (c *Client) BulkCreateContacts(ctx context.Context, contacts []map[string]any) ([]Contact, error) {
	var out []Contact
	return out, c.Do(ctx, http.MethodPost, "/2.0/contact/_bulk_create", nil, contacts, &out)
}
