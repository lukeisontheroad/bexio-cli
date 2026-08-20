package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Group is a contact group (API resource "contact_group").
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (g *Group) UnmarshalJSON(data []byte) error {
	type group Group
	var v group
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*g = Group(v)
	g.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListGroups(ctx context.Context, opts ListOptions) ([]Group, error) {
	var out []Group
	return out, c.Get(ctx, "/2.0/contact_group", opts.values(), &out)
}

func (c *Client) SearchGroups(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Group, error) {
	var out []Group
	return out, c.Do(ctx, http.MethodPost, "/2.0/contact_group/search", opts.values(), criteria, &out)
}

func (c *Client) GetGroup(ctx context.Context, id int) (*Group, error) {
	var out Group
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/contact_group/%d", id), nil, &out)
}

func (c *Client) CreateGroup(ctx context.Context, name string) (*Group, error) {
	var out Group
	return &out, c.Do(ctx, http.MethodPost, "/2.0/contact_group", nil, map[string]any{"name": name}, &out)
}

func (c *Client) UpdateGroup(ctx context.Context, id int, name string) (*Group, error) {
	var out Group
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/contact_group/%d", id), nil, map[string]any{"name": name}, &out)
}

func (c *Client) DeleteGroup(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/contact_group/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete contact group %d: API reported failure", id)
	}
	return nil
}

// Sector is a contact sector (API resource "contact_branch", read-only).
type Sector struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (s *Sector) UnmarshalJSON(data []byte) error {
	type sector Sector
	var v sector
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = Sector(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListSectors(ctx context.Context, opts ListOptions) ([]Sector, error) {
	var out []Sector
	return out, c.Get(ctx, "/2.0/contact_branch", opts.values(), &out)
}

func (c *Client) SearchSectors(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Sector, error) {
	var out []Sector
	return out, c.Do(ctx, http.MethodPost, "/2.0/contact_branch/search", opts.values(), criteria, &out)
}
