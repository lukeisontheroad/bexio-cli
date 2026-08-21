package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// This file covers the accounting resources: manual entries (3.0, incl.
// their file attachments), accounts and account groups (2.0), business
// years, calendar years and VAT periods (3.0), and the journal report.

// ManualEntryLine is one debit/credit line of a manual entry. The API calls
// this nested object "ManualEntry" as well; the CLI names it a line to keep
// it apart from the booking it belongs to.
type ManualEntryLine struct {
	ID                 int     `json:"id,omitempty"`
	Date               string  `json:"date,omitempty"`
	DebitAccountID     int     `json:"debit_account_id,omitempty"`
	CreditAccountID    int     `json:"credit_account_id,omitempty"`
	TaxID              int     `json:"tax_id,omitempty"`
	TaxAccountID       int     `json:"tax_account_id,omitempty"`
	Description        string  `json:"description,omitempty"`
	Amount             float64 `json:"amount,omitempty"`
	CurrencyID         int     `json:"currency_id,omitempty"`
	CurrencyFactor     float64 `json:"currency_factor,omitempty"`
	BaseCurrencyID     int     `json:"base_currency_id,omitempty"`
	BaseCurrencyAmount float64 `json:"base_currency_amount,omitempty"`
	CreatedByUserID    int     `json:"created_by_user_id,omitempty"`
	EditedByUserID     int     `json:"edited_by_user_id,omitempty"`
}

// ManualEntry is a manual accounting entry (API resource
// "accounting/manual_entries", 3.0).
type ManualEntry struct {
	ID              int               `json:"id"`
	Type            string            `json:"type"`
	Date            string            `json:"date"`
	ReferenceNr     string            `json:"reference_nr"`
	CreatedByUserID int               `json:"created_by_user_id"`
	EditedByUserID  int               `json:"edited_by_user_id"`
	Entries         []ManualEntryLine `json:"entries"`
	IsLocked        bool              `json:"is_locked"`
	LockedInfo      string            `json:"locked_info"`

	Raw json.RawMessage `json:"-"`
}

func (m *ManualEntry) UnmarshalJSON(data []byte) error {
	type manualEntry ManualEntry
	var v manualEntry
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = ManualEntry(v)
	m.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Total sums the line amounts of the entry.
func (m *ManualEntry) Total() float64 {
	var sum float64
	for _, e := range m.Entries {
		sum += e.Amount
	}
	return sum
}

// ManualEntryTypes are the values allowed for a manual entry's type.
var ManualEntryTypes = []string{
	"manual_single_entry", "manual_compound_entry", "manual_group_entry",
}

// ListManualEntries fetches /3.0/accounting/manual_entries (limit/offset
// only — 3.0 list endpoints have no order_by).
func (c *Client) ListManualEntries(ctx context.Context, opts ListOptions) ([]ManualEntry, error) {
	var out []ManualEntry
	return out, c.Get(ctx, "/3.0/accounting/manual_entries", lookupValues30(opts), &out)
}

// manualEntryScanPage / manualEntryScanPages bound the list scan behind
// GetManualEntry.
const (
	manualEntryScanPage  = 2000
	manualEntryScanPages = 10
)

// GetManualEntry returns a single manual entry. The 3.0 API has no
// "fetch one" endpoint for manual entries, so the list endpoint is paged
// through (up to manualEntryScanPage*manualEntryScanPages entries) and
// matched on id.
func (c *Client) GetManualEntry(ctx context.Context, id int) (*ManualEntry, error) {
	for page := 0; page < manualEntryScanPages; page++ {
		entries, err := c.ListManualEntries(ctx, ListOptions{
			Limit:  manualEntryScanPage,
			Offset: page * manualEntryScanPage,
		})
		if err != nil {
			return nil, err
		}
		for i := range entries {
			if entries[i].ID == id {
				return &entries[i], nil
			}
		}
		if len(entries) < manualEntryScanPage {
			break
		}
	}
	return nil, fmt.Errorf("manual entry %d not found", id)
}

// CreateManualEntry posts /3.0/accounting/manual_entries. fields uses the
// raw API field names; type, date and entries are required.
func (c *Client) CreateManualEntry(ctx context.Context, fields map[string]any) (*ManualEntry, error) {
	var out ManualEntry
	return &out, c.Do(ctx, http.MethodPost, "/3.0/accounting/manual_entries", nil, fields, &out)
}

// UpdateManualEntry replaces a manual entry. The 3.0 API uses PUT here (not
// POST like the 2.0 resources) and expects the complete object.
func (c *Client) UpdateManualEntry(ctx context.Context, id int, fields map[string]any) (*ManualEntry, error) {
	var out ManualEntry
	return &out, c.Do(ctx, http.MethodPut, fmt.Sprintf("/3.0/accounting/manual_entries/%d", id), nil, fields, &out)
}

// DeleteManualEntry permanently deletes a manual entry.
func (c *Client) DeleteManualEntry(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/3.0/accounting/manual_entries/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete manual entry %d: API reported failure", id)
	}
	return nil
}

// ManualEntryRefNr is the response of the next_ref_nr endpoint.
type ManualEntryRefNr struct {
	NextRefNr string `json:"next_ref_nr"`

	Raw json.RawMessage `json:"-"`
}

func (r *ManualEntryRefNr) UnmarshalJSON(data []byte) error {
	type refNr ManualEntryRefNr
	var v refNr
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = ManualEntryRefNr(v)
	r.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// NextManualEntryRefNr suggests the reference number for the next manual
// entry (GET /3.0/accounting/manual_entries/next_ref_nr).
func (c *Client) NextManualEntryRefNr(ctx context.Context) (*ManualEntryRefNr, error) {
	var out ManualEntryRefNr
	return &out, c.Get(ctx, "/3.0/accounting/manual_entries/next_ref_nr", nil, &out)
}

// ManualEntryFile is a file attached to a manual entry (or to one of its
// lines). Data is only set by the single-file endpoint (base64 content).
type ManualEntryFile struct {
	ID            int    `json:"id"`
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	SizeInBytes   int64  `json:"size_in_bytes"`
	Extension     string `json:"extension"`
	MimeType      string `json:"mime_type"`
	UploaderEmail string `json:"uploader_email"`
	UserID        int    `json:"user_id"`
	IsArchived    bool   `json:"is_archived"`
	IsReferenced  bool   `json:"is_referenced"`
	SourceType    string `json:"source_type"`
	CreatedAt     string `json:"created_at"`
	Data          string `json:"data"`

	Raw json.RawMessage `json:"-"`
}

func (f *ManualEntryFile) UnmarshalJSON(data []byte) error {
	type manualEntryFile ManualEntryFile
	var v manualEntryFile
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*f = ManualEntryFile(v)
	f.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// manualEntryFilesPath builds the file collection path. entryID > 0 selects
// the per-line endpoint (.../entries/{entry_id}/files), otherwise the files
// of the manual (compound) entry itself.
func manualEntryFilesPath(manualEntryID, entryID int) string {
	if entryID > 0 {
		return fmt.Sprintf("/3.0/accounting/manual_entries/%d/entries/%d/files", manualEntryID, entryID)
	}
	return fmt.Sprintf("/3.0/accounting/manual_entries/%d/files", manualEntryID)
}

// ListManualEntryFiles lists the files of a manual entry (entryID == 0) or
// of one of its lines (entryID > 0).
func (c *Client) ListManualEntryFiles(ctx context.Context, manualEntryID, entryID int, opts ListOptions) ([]ManualEntryFile, error) {
	var out []ManualEntryFile
	return out, c.Get(ctx, manualEntryFilesPath(manualEntryID, entryID), lookupValues30(opts), &out)
}

// GetManualEntryFile fetches a single attached file (including its base64
// content in data).
func (c *Client) GetManualEntryFile(ctx context.Context, manualEntryID, entryID, fileID int) (*ManualEntryFile, error) {
	var out ManualEntryFile
	path := fmt.Sprintf("%s/%d", manualEntryFilesPath(manualEntryID, entryID), fileID)
	return &out, c.Get(ctx, path, nil, &out)
}

// DeleteManualEntryFile removes the connection between a file and a manual
// entry (or one of its lines).
func (c *Client) DeleteManualEntryFile(ctx context.Context, manualEntryID, entryID, fileID int) error {
	var out Success
	path := fmt.Sprintf("%s/%d", manualEntryFilesPath(manualEntryID, entryID), fileID)
	if err := c.Do(ctx, http.MethodDelete, path, nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete file %d of manual entry %d: API reported failure", fileID, manualEntryID)
	}
	return nil
}

// UploadManualEntryFiles attaches local files to a manual entry (entryID ==
// 0) or to one of its lines (entryID > 0). The endpoint takes
// multipart/form-data and requires a distinct form field per file
// (fileName, fileName2, ...); max 12 MB per file.
func (c *Client) UploadManualEntryFiles(ctx context.Context, manualEntryID, entryID int, paths []string) ([]ManualEntryFile, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no file to upload")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // user-supplied upload path is the point
		if err != nil {
			return nil, err
		}
		field := "fileName"
		if i > 0 {
			field = fmt.Sprintf("fileName%d", i+1)
		}
		w, err := mw.CreateFormFile(field, filepath.Base(p))
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	var out []ManualEntryFile
	path := manualEntryFilesPath(manualEntryID, entryID)
	return out, c.accountingPostMultipart(ctx, path, mw.FormDataContentType(), buf.Bytes(), &out)
}

// accountingPostMultipart posts a multipart/form-data body. Do() only
// speaks JSON, so the file upload endpoints get their own request path
// (same auth, verbose logging, and error mapping).
func (c *Client) accountingPostMultipart(ctx context.Context, path, contentType string, body []byte, out any) error {
	if c.ReadOnly {
		return fmt.Errorf("read-only instance: refusing POST %s (log in again without --read-only to allow writes)", path)
	}
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	token, err := c.source.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bexio-cli")
	req.Header.Set("Content-Type", contentType)

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> POST %s (multipart, %d bytes)\n", u.String(), len(body))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(data))
	}
	if resp.StatusCode >= 400 {
		apiErr := &Error{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr) // best effort; error text may not be JSON
		return apiErr
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Account is a ledger account (API resource "accounts", 2.0, read-only).
type Account struct {
	ID                 int    `json:"id"`
	UUID               string `json:"uuid"`
	AccountNo          string `json:"account_no"`
	Name               string `json:"name"`
	AccountType        int    `json:"account_type"`
	TaxID              int    `json:"tax_id"`
	FibuAccountGroupID int    `json:"fibu_account_group_id"`
	IsActive           bool   `json:"is_active"`
	IsLocked           bool   `json:"is_locked"`

	Raw json.RawMessage `json:"-"`
}

func (a *Account) UnmarshalJSON(data []byte) error {
	type account Account
	var v account
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = Account(v)
	a.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListAccounts fetches /2.0/accounts (limit/offset only).
func (c *Client) ListAccounts(ctx context.Context, opts ListOptions) ([]Account, error) {
	var out []Account
	return out, c.Get(ctx, "/2.0/accounts", lookupValues30(opts), &out)
}

// SearchAccounts posts to /2.0/accounts/search.
func (c *Client) SearchAccounts(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Account, error) {
	var out []Account
	return out, c.Do(ctx, http.MethodPost, "/2.0/accounts/search", lookupValues30(opts), criteria, &out)
}

// AccountGroup is a group of ledger accounts (API resource
// "account_groups", 2.0, read-only).
type AccountGroup struct {
	ID                       int    `json:"id"`
	UUID                     string `json:"uuid"`
	AccountNo                string `json:"account_no"`
	Name                     string `json:"name"`
	ParentFibuAccountGroupID int    `json:"parent_fibu_account_group_id"`
	IsActive                 bool   `json:"is_active"`
	IsLocked                 bool   `json:"is_locked"`

	Raw json.RawMessage `json:"-"`
}

func (g *AccountGroup) UnmarshalJSON(data []byte) error {
	type accountGroup AccountGroup
	var v accountGroup
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*g = AccountGroup(v)
	g.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListAccountGroups fetches /2.0/account_groups.
func (c *Client) ListAccountGroups(ctx context.Context, opts ListOptions) ([]AccountGroup, error) {
	var out []AccountGroup
	return out, c.Get(ctx, "/2.0/account_groups", lookupValues30(opts), &out)
}

// BusinessYear is a business year (API resource
// "accounting/business_years", 3.0, read-only).
type BusinessYear struct {
	ID       int    `json:"id"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Status   string `json:"status"`
	ClosedAt string `json:"closed_at"`

	Raw json.RawMessage `json:"-"`
}

func (b *BusinessYear) UnmarshalJSON(data []byte) error {
	type businessYear BusinessYear
	var v businessYear
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = BusinessYear(v)
	b.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListBusinessYears fetches /3.0/accounting/business_years.
func (c *Client) ListBusinessYears(ctx context.Context, opts ListOptions) ([]BusinessYear, error) {
	var out []BusinessYear
	return out, c.Get(ctx, "/3.0/accounting/business_years", lookupValues30(opts), &out)
}

// GetBusinessYear fetches a single business year.
func (c *Client) GetBusinessYear(ctx context.Context, id int) (*BusinessYear, error) {
	var out BusinessYear
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/accounting/business_years/%d", id), nil, &out)
}

// CalendarYear is a calendar year (API resource
// "accounting/calendar_years", 3.0).
type CalendarYear struct {
	ID                  int    `json:"id"`
	Start               string `json:"start"`
	End                 string `json:"end"`
	IsVatSubject        bool   `json:"is_vat_subject"`
	IsAnnualReporting   bool   `json:"is_annual_reporting"`
	VatAccountingMethod string `json:"vat_accounting_method"`
	VatAccountingType   string `json:"vat_accounting_type"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (y *CalendarYear) UnmarshalJSON(data []byte) error {
	type calendarYear CalendarYear
	var v calendarYear
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*y = CalendarYear(v)
	y.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListCalendarYears fetches /3.0/accounting/calendar_years.
func (c *Client) ListCalendarYears(ctx context.Context, opts ListOptions) ([]CalendarYear, error) {
	var out []CalendarYear
	return out, c.Get(ctx, "/3.0/accounting/calendar_years", lookupValues30(opts), &out)
}

// SearchCalendarYears posts to /3.0/accounting/calendar_years/search.
func (c *Client) SearchCalendarYears(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]CalendarYear, error) {
	var out []CalendarYear
	return out, c.Do(ctx, http.MethodPost, "/3.0/accounting/calendar_years/search", lookupValues30(opts), criteria, &out)
}

// GetCalendarYear fetches a single calendar year.
func (c *Client) GetCalendarYear(ctx context.Context, id int) (*CalendarYear, error) {
	var out CalendarYear
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/accounting/calendar_years/%d", id), nil, &out)
}

// CreateCalendarYear creates a calendar year. The response is a list: for a
// future year the API also generates every year in between.
func (c *Client) CreateCalendarYear(ctx context.Context, fields map[string]any) ([]CalendarYear, error) {
	var out []CalendarYear
	return out, c.Do(ctx, http.MethodPost, "/3.0/accounting/calendar_years", nil, fields, &out)
}

// VatPeriod is a VAT period (API resource "accounting/vat_periods", 3.0,
// read-only).
type VatPeriod struct {
	ID       int    `json:"id"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	ClosedAt string `json:"closed_at"`

	Raw json.RawMessage `json:"-"`
}

func (p *VatPeriod) UnmarshalJSON(data []byte) error {
	type vatPeriod VatPeriod
	var v vatPeriod
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = VatPeriod(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListVatPeriods fetches /3.0/accounting/vat_periods.
func (c *Client) ListVatPeriods(ctx context.Context, opts ListOptions) ([]VatPeriod, error) {
	var out []VatPeriod
	return out, c.Get(ctx, "/3.0/accounting/vat_periods", lookupValues30(opts), &out)
}

// GetVatPeriod fetches a single VAT period.
func (c *Client) GetVatPeriod(ctx context.Context, id int) (*VatPeriod, error) {
	var out VatPeriod
	return &out, c.Get(ctx, fmt.Sprintf("/3.0/accounting/vat_periods/%d", id), nil, &out)
}

// JournalEntry is one line of the accounting journal report (GET
// /3.0/accounting/journal).
type JournalEntry struct {
	ID                 int     `json:"id"`
	RefID              int     `json:"ref_id"`
	RefUUID            string  `json:"ref_uuid"`
	RefClass           string  `json:"ref_class"`
	Date               string  `json:"date"`
	DebitAccountID     int     `json:"debit_account_id"`
	CreditAccountID    int     `json:"credit_account_id"`
	Description        string  `json:"description"`
	Amount             float64 `json:"amount"`
	CurrencyID         int     `json:"currency_id"`
	CurrencyFactor     float64 `json:"currency_factor"`
	BaseCurrencyID     int     `json:"base_currency_id"`
	BaseCurrencyAmount float64 `json:"base_currency_amount"`

	Raw json.RawMessage `json:"-"`
}

func (j *JournalEntry) UnmarshalJSON(data []byte) error {
	type journalEntry JournalEntry
	var v journalEntry
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*j = JournalEntry(v)
	j.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// JournalOptions are the query parameters of the journal report. From/To
// are dates (YYYY-MM-DD), AccountUUID filters by account.
type JournalOptions struct {
	ListOptions
	From        string
	To          string
	AccountUUID string
}

func (o JournalOptions) values() url.Values {
	q := lookupValues30(o.ListOptions)
	if o.From != "" {
		q.Set("from", o.From)
	}
	if o.To != "" {
		q.Set("to", o.To)
	}
	if o.AccountUUID != "" {
		q.Set("account_uuid", o.AccountUUID)
	}
	return q
}

// ListJournalEntries fetches the journal report (/3.0/accounting/journal).
func (c *Client) ListJournalEntries(ctx context.Context, opts JournalOptions) ([]JournalEntry, error) {
	var out []JournalEntry
	return out, c.Get(ctx, "/3.0/accounting/journal", opts.values(), &out)
}
