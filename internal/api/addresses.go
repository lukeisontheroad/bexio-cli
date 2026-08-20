package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Address is an additional address of a contact.
type Address struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	NameAddition string `json:"name_addition"`
	StreetName   string `json:"street_name"`
	HouseNumber  string `json:"house_number"`
	Postcode     string `json:"postcode"`
	City         string `json:"city"`
	Subject      string `json:"subject"`

	Raw json.RawMessage `json:"-"`
}

func (a *Address) UnmarshalJSON(data []byte) error {
	type address Address
	var v address
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = Address(v)
	a.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func addressPath(contactID int) string {
	return fmt.Sprintf("/2.0/contact/%d/additional_address", contactID)
}

func (c *Client) ListAddresses(ctx context.Context, contactID int, opts ListOptions) ([]Address, error) {
	var out []Address
	return out, c.Get(ctx, addressPath(contactID), opts.values(), &out)
}

func (c *Client) SearchAddresses(ctx context.Context, contactID int, criteria []SearchCriterion, opts ListOptions) ([]Address, error) {
	var out []Address
	return out, c.Do(ctx, http.MethodPost, addressPath(contactID)+"/search", opts.values(), criteria, &out)
}

func (c *Client) GetAddress(ctx context.Context, contactID, id int) (*Address, error) {
	var out Address
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", addressPath(contactID), id), nil, &out)
}

func (c *Client) CreateAddress(ctx context.Context, contactID int, fields map[string]any) (*Address, error) {
	var out Address
	return &out, c.Do(ctx, http.MethodPost, addressPath(contactID), nil, fields, &out)
}

func (c *Client) UpdateAddress(ctx context.Context, contactID, id int, fields map[string]any) (*Address, error) {
	var out Address
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%d", addressPath(contactID), id), nil, fields, &out)
}

func (c *Client) DeleteAddress(ctx context.Context, contactID, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", addressPath(contactID), id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete additional address %d: API reported failure", id)
	}
	return nil
}
