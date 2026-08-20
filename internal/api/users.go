package api

import (
	"context"
	"encoding/json"
	"strings"
)

// User is a bexio user (/3.0/users).
type User struct {
	ID        int    `json:"id"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Email     string `json:"email"`

	Raw json.RawMessage `json:"-"`
}

func (u *User) UnmarshalJSON(data []byte) error {
	type user User
	var v user
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*u = User(v)
	u.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (u User) DisplayName() string {
	return strings.TrimSpace(u.Firstname + " " + u.Lastname)
}

// Me returns the authenticated user (also serves as the login token check).
func (c *Client) Me(ctx context.Context) (*User, error) {
	var out User
	return &out, c.Get(ctx, "/3.0/users/me", nil, &out)
}

// CompanyProfile is the company the token belongs to (/2.0/company_profile).
type CompanyProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CompanyName returns the company name, or "" if the endpoint is not
// accessible with the token's scopes.
func (c *Client) CompanyName(ctx context.Context) string {
	var out []CompanyProfile
	if err := c.Get(ctx, "/2.0/company_profile", nil, &out); err != nil || len(out) == 0 {
		return ""
	}
	return out[0].Name
}
