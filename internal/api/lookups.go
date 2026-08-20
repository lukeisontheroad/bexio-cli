package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// This file covers the reference-data ("lookup") resources other commands
// reference by id: countries, languages, salutations, titles, units,
// payment types, currencies, taxes, bank accounts, users, company profiles,
// and permissions.

// lookupValues30 builds the query for 3.0 list endpoints, which only
// support limit/offset (no order_by / show_archived).
func lookupValues30(o ListOptions) url.Values {
	q := url.Values{}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Offset > 0 {
		q.Set("offset", strconv.Itoa(o.Offset))
	}
	return q
}

// Country is a country (API resource "country", 2.0).
type Country struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	NameShort     string `json:"name_short"`
	Iso3166Alpha2 string `json:"iso3166_alpha2"`

	Raw json.RawMessage `json:"-"`
}

func (c *Country) UnmarshalJSON(data []byte) error {
	type country Country
	var v country
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*c = Country(v)
	c.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListCountries(ctx context.Context, opts ListOptions) ([]Country, error) {
	var out []Country
	return out, c.Get(ctx, "/2.0/country", opts.values(), &out)
}

func (c *Client) SearchCountries(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Country, error) {
	var out []Country
	return out, c.Do(ctx, http.MethodPost, "/2.0/country/search", opts.values(), criteria, &out)
}

func (c *Client) GetCountry(ctx context.Context, id int) (*Country, error) {
	var out Country
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/country/%d", id), nil, &out)
}

func (c *Client) CreateCountry(ctx context.Context, fields map[string]any) (*Country, error) {
	var out Country
	return &out, c.Do(ctx, http.MethodPost, "/2.0/country", nil, fields, &out)
}

func (c *Client) UpdateCountry(ctx context.Context, id int, fields map[string]any) (*Country, error) {
	var out Country
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/country/%d", id), nil, fields, &out)
}

func (c *Client) DeleteCountry(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/country/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete country %d: API reported failure", id)
	}
	return nil
}

// Language is a language (API resource "language", 2.0, read-only).
type Language struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Iso6391 string `json:"iso_639_1"`

	Raw json.RawMessage `json:"-"`
}

func (l *Language) UnmarshalJSON(data []byte) error {
	type language Language
	var v language
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*l = Language(v)
	l.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListLanguages(ctx context.Context, opts ListOptions) ([]Language, error) {
	var out []Language
	return out, c.Get(ctx, "/2.0/language", opts.values(), &out)
}

func (c *Client) SearchLanguages(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Language, error) {
	var out []Language
	return out, c.Do(ctx, http.MethodPost, "/2.0/language/search", opts.values(), criteria, &out)
}

// Salutation is a salutation (API resource "salutation", 2.0).
type Salutation struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (s *Salutation) UnmarshalJSON(data []byte) error {
	type salutation Salutation
	var v salutation
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = Salutation(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListSalutations(ctx context.Context, opts ListOptions) ([]Salutation, error) {
	var out []Salutation
	return out, c.Get(ctx, "/2.0/salutation", opts.values(), &out)
}

func (c *Client) SearchSalutations(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Salutation, error) {
	var out []Salutation
	return out, c.Do(ctx, http.MethodPost, "/2.0/salutation/search", opts.values(), criteria, &out)
}

func (c *Client) GetSalutation(ctx context.Context, id int) (*Salutation, error) {
	var out Salutation
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/salutation/%d", id), nil, &out)
}

func (c *Client) CreateSalutation(ctx context.Context, name string) (*Salutation, error) {
	var out Salutation
	return &out, c.Do(ctx, http.MethodPost, "/2.0/salutation", nil, map[string]any{"name": name}, &out)
}

func (c *Client) UpdateSalutation(ctx context.Context, id int, name string) (*Salutation, error) {
	var out Salutation
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/salutation/%d", id), nil, map[string]any{"name": name}, &out)
}

func (c *Client) DeleteSalutation(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/salutation/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete salutation %d: API reported failure", id)
	}
	return nil
}

// Title is a personal title (API resource "title", 2.0).
type Title struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (t *Title) UnmarshalJSON(data []byte) error {
	type title Title
	var v title
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Title(v)
	t.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListTitles(ctx context.Context, opts ListOptions) ([]Title, error) {
	var out []Title
	return out, c.Get(ctx, "/2.0/title", opts.values(), &out)
}

func (c *Client) SearchTitles(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Title, error) {
	var out []Title
	return out, c.Do(ctx, http.MethodPost, "/2.0/title/search", opts.values(), criteria, &out)
}

func (c *Client) GetTitle(ctx context.Context, id int) (*Title, error) {
	var out Title
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/title/%d", id), nil, &out)
}

func (c *Client) CreateTitle(ctx context.Context, name string) (*Title, error) {
	var out Title
	return &out, c.Do(ctx, http.MethodPost, "/2.0/title", nil, map[string]any{"name": name}, &out)
}

func (c *Client) UpdateTitle(ctx context.Context, id int, name string) (*Title, error) {
	var out Title
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/title/%d", id), nil, map[string]any{"name": name}, &out)
}

func (c *Client) DeleteTitle(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/title/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete title %d: API reported failure", id)
	}
	return nil
}

// Unit is an article/position unit (API resource "unit", 2.0).
type Unit struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (u *Unit) UnmarshalJSON(data []byte) error {
	type unit Unit
	var v unit
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*u = Unit(v)
	u.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListUnits(ctx context.Context, opts ListOptions) ([]Unit, error) {
	var out []Unit
	return out, c.Get(ctx, "/2.0/unit", opts.values(), &out)
}

func (c *Client) SearchUnits(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Unit, error) {
	var out []Unit
	return out, c.Do(ctx, http.MethodPost, "/2.0/unit/search", opts.values(), criteria, &out)
}

func (c *Client) GetUnit(ctx context.Context, id int) (*Unit, error) {
	var out Unit
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/unit/%d", id), nil, &out)
}

func (c *Client) CreateUnit(ctx context.Context, name string) (*Unit, error) {
	var out Unit
	return &out, c.Do(ctx, http.MethodPost, "/2.0/unit", nil, map[string]any{"name": name}, &out)
}

func (c *Client) UpdateUnit(ctx context.Context, id int, name string) (*Unit, error) {
	var out Unit
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/unit/%d", id), nil, map[string]any{"name": name}, &out)
}

func (c *Client) DeleteUnit(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/unit/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete unit %d: API reported failure", id)
	}
	return nil
}

// PaymentType is a payment type (API resource "payment_type", 2.0, read-only).
type PaymentType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (p *PaymentType) UnmarshalJSON(data []byte) error {
	type paymentType PaymentType
	var v paymentType
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = PaymentType(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListPaymentTypes(ctx context.Context, opts ListOptions) ([]PaymentType, error) {
	var out []PaymentType
	return out, c.Get(ctx, "/2.0/payment_type", opts.values(), &out)
}

func (c *Client) SearchPaymentTypes(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]PaymentType, error) {
	var out []PaymentType
	return out, c.Do(ctx, http.MethodPost, "/2.0/payment_type/search", opts.values(), criteria, &out)
}

// Currency is a currency (API resource "currencies", 3.0).
type Currency struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	RoundFactor float64 `json:"round_factor"`

	Raw json.RawMessage `json:"-"`
}

func (cu *Currency) UnmarshalJSON(data []byte) error {
	type currency Currency
	var v currency
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*cu = Currency(v)
	cu.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListCurrencies(ctx context.Context, opts ListOptions) ([]Currency, error) {
	var out []Currency
	return out, c.Get(ctx, "/3.0/currencies", lookupValues30(opts), &out)
}

func (c *Client) GetCurrency(ctx context.Context, id int) (*Currency, error) {
	var out Currency
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/currencies/%d", id), nil, &out)
}

// CurrencyCodes returns the raw response of GET /3.0/currencies/codes
// (an array of ISO 4217 codes).
func (c *Client) CurrencyCodes(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Get(ctx, "/3.0/currencies/codes", nil, &out)
}

// CurrencyExchangeRates returns the raw exchange rates of a currency,
// optionally at a given date (YYYY-MM-DD).
func (c *Client) CurrencyExchangeRates(ctx context.Context, id int, date string) (json.RawMessage, error) {
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	var out json.RawMessage
	return out, c.Get(ctx, fmt.Sprintf("/3.0/currencies/%d/exchange_rates", id), q, &out)
}

// Tax is a tax rate (API resource "taxes", 3.0).
type Tax struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Type        string  `json:"type"`
	Value       float64 `json:"value"`
	DisplayName string  `json:"display_name"`
	IsActive    bool    `json:"is_active"`

	Raw json.RawMessage `json:"-"`
}

func (t *Tax) UnmarshalJSON(data []byte) error {
	type tax Tax
	var v tax
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Tax(v)
	t.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// TaxListOptions are the query parameters of GET /3.0/taxes. Types is
// "sales_tax" or "pre_tax"; Scope is "active" or "inactive"; Date
// (YYYY-MM-DD) filters taxes active at that date.
type TaxListOptions struct {
	ListOptions
	Types string
	Scope string
	Date  string
}

func (c *Client) ListTaxes(ctx context.Context, opts TaxListOptions) ([]Tax, error) {
	q := lookupValues30(opts.ListOptions)
	if opts.Types != "" {
		q.Set("types", opts.Types)
	}
	if opts.Scope != "" {
		q.Set("scope", opts.Scope)
	}
	if opts.Date != "" {
		q.Set("date", opts.Date)
	}
	var out []Tax
	return out, c.Get(ctx, "/3.0/taxes", q, &out)
}

func (c *Client) GetTax(ctx context.Context, id int) (*Tax, error) {
	var out Tax
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/taxes/%d", id), nil, &out)
}

// BankAccount is a bank account (API resource "banking/accounts", 3.0,
// read-only).
type BankAccount struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	BankName   string `json:"bank_name"`
	IbanNr     string `json:"iban_nr"`
	CurrencyID int    `json:"currency_id"`
	Type       string `json:"type"`

	Raw json.RawMessage `json:"-"`
}

func (b *BankAccount) UnmarshalJSON(data []byte) error {
	type bankAccount BankAccount
	var v bankAccount
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = BankAccount(v)
	b.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListBankAccounts(ctx context.Context, opts ListOptions) ([]BankAccount, error) {
	var out []BankAccount
	return out, c.Get(ctx, "/3.0/banking/accounts", lookupValues30(opts), &out)
}

func (c *Client) GetBankAccount(ctx context.Context, id int) (*BankAccount, error) {
	var out BankAccount
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/banking/accounts/%d", id), nil, &out)
}

// ListUsers lists the users of the company (/3.0/users).
func (c *Client) ListUsers(ctx context.Context, opts ListOptions) ([]User, error) {
	var out []User
	return out, c.Get(ctx, "/3.0/users", lookupValues30(opts), &out)
}

// GetUser fetches a single user (/3.0/users/{id}).
func (c *Client) GetUser(ctx context.Context, id int) (*User, error) {
	var out User
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/users/%d", id), nil, &out)
}

// CompanyProfileDetail is a full company profile (API resource
// "company_profile", 2.0, read-only). The trimmed CompanyProfile in
// users.go serves auth status; this one keeps the raw response.
type CompanyProfileDetail struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (p *CompanyProfileDetail) UnmarshalJSON(data []byte) error {
	type profile CompanyProfileDetail
	var v profile
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = CompanyProfileDetail(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListCompanyProfiles(ctx context.Context) ([]CompanyProfileDetail, error) {
	var out []CompanyProfileDetail
	return out, c.Get(ctx, "/2.0/company_profile", nil, &out)
}

func (c *Client) GetCompanyProfile(ctx context.Context, id int) (*CompanyProfileDetail, error) {
	var out CompanyProfileDetail
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/company_profile/%d", id), nil, &out)
}

// GetPermissions returns the raw access information of the authenticated
// user (GET /3.0/permissions: components + per-component permissions).
func (c *Client) GetPermissions(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, c.Get(ctx, "/3.0/permissions", nil, &out)
}
