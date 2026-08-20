package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Relation links two contacts (API resource "contact_relation"). contact_id
// is the company side, contact_sub_id the person side.
type Relation struct {
	ID           int    `json:"id"`
	ContactID    int    `json:"contact_id"`
	ContactSubID int    `json:"contact_sub_id"`
	Description  string `json:"description"`
	UpdatedAt    string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (r *Relation) UnmarshalJSON(data []byte) error {
	type relation Relation
	var v relation
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = Relation(v)
	r.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListRelations(ctx context.Context, opts ListOptions) ([]Relation, error) {
	var out []Relation
	return out, c.Get(ctx, "/2.0/contact_relation", opts.values(), &out)
}

func (c *Client) SearchRelations(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Relation, error) {
	var out []Relation
	return out, c.Do(ctx, http.MethodPost, "/2.0/contact_relation/search", opts.values(), criteria, &out)
}

func (c *Client) GetRelation(ctx context.Context, id int) (*Relation, error) {
	var out Relation
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/contact_relation/%d", id), nil, &out)
}

func (c *Client) CreateRelation(ctx context.Context, fields map[string]any) (*Relation, error) {
	var out Relation
	return &out, c.Do(ctx, http.MethodPost, "/2.0/contact_relation", nil, fields, &out)
}

func (c *Client) UpdateRelation(ctx context.Context, id int, fields map[string]any) (*Relation, error) {
	var out Relation
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/contact_relation/%d", id), nil, fields, &out)
}

func (c *Client) DeleteRelation(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/contact_relation/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete contact relation %d: API reported failure", id)
	}
	return nil
}
