package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// This file covers the 4.0 payroll API: employees, their absences, and the
// paystub PDF download. Unlike the 2.0/3.0 resources, payroll ids are UUID
// strings, list responses come wrapped in a {"data": [...]} envelope, and
// the write endpoints answer 204 No Content.

// PayrollDecimal holds a decimal value the payroll API may send either as a
// JSON number or as a string; the original text is kept for display.
type PayrollDecimal string

func (d *PayrollDecimal) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" || s == "" {
		*d = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*d = PayrollDecimal(str)
		return nil
	}
	*d = PayrollDecimal(s)
	return nil
}

func (d PayrollDecimal) String() string { return string(d) }

// payrollDecodeList unwraps the {"data": [...]} envelope of the 4.0 list
// endpoints, tolerating a bare array as well.
func payrollDecodeList[T any](raw json.RawMessage) ([]T, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var out []T
		return out, json.Unmarshal(raw, &out)
	}
	var envelope struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

// EmployeeAddress is the nested address object of a payroll employee. The
// `street` field is deprecated: send street_name + house_number instead.
type EmployeeAddress struct {
	ComplementaryLine string `json:"complementary_line"`
	StreetName        string `json:"street_name"`
	HouseNumber       string `json:"house_number"`
	Postbox           string `json:"postbox"`
	Locality          string `json:"locality"`
	ZipCode           string `json:"zip_code"`
	City              string `json:"city"`
	Country           string `json:"country"`
	Canton            string `json:"canton"`
	MunicipalityID    string `json:"municipality_id"`
}

// Employee is a payroll employee (4.0 API /4.0/payroll/employees). Raw
// preserves the full API object for --output json.
type Employee struct {
	ID                 string          `json:"id"`
	FirstName          string          `json:"first_name"`
	LastName           string          `json:"last_name"`
	DateOfBirth        string          `json:"date_of_birth"`
	AhvNumber          string          `json:"ahv_number"`
	Gender             string          `json:"gender"`
	Nationality        string          `json:"nationality"`
	StayPermitCategory string          `json:"stay_permit_category"`
	Language           string          `json:"language"`
	MaritalStatus      string          `json:"marital_status"`
	Email              string          `json:"email"`
	PhoneNumber        string          `json:"phone_number"`
	HoursPerWeek       PayrollDecimal  `json:"hours_per_week"`
	EmploymentLevel    PayrollDecimal  `json:"employment_level"`
	AnnualVacationDays int             `json:"annual_vacation_days_total"`
	PersonalNumber     string          `json:"personal_number"`
	Iban               string          `json:"iban"`
	Address            EmployeeAddress `json:"address"`

	Raw json.RawMessage `json:"-"`
}

func (e *Employee) UnmarshalJSON(data []byte) error {
	type employee Employee // avoid recursion
	var v employee
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = Employee(v)
	e.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Name renders "first last", falling back to whichever part is present.
func (e Employee) Name() string {
	return strings.TrimSpace(e.FirstName + " " + e.LastName)
}

const employeePath = "/4.0/payroll/employees"

// ListEmployees fetches all active employees (GET /4.0/payroll/employees).
// The endpoint takes no paging parameters and answers a {"data": [...]}
// envelope.
func (c *Client) ListEmployees(ctx context.Context) ([]Employee, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, employeePath, nil, &raw); err != nil {
		return nil, err
	}
	return payrollDecodeList[Employee](raw)
}

// GetEmployee fetches the state of an employee on a date (YYYY-MM-DD). The
// date parameter is required by the API.
func (c *Client) GetEmployee(ctx context.Context, id, date string) (*Employee, error) {
	if date == "" {
		return nil, fmt.Errorf("employee lookup needs a date (YYYY-MM-DD)")
	}
	q := url.Values{}
	q.Set("date", date)
	var out Employee
	return &out, c.Get(ctx, employeePath+"/"+url.PathEscape(id), q, &out)
}

// CreateEmployee creates an employee. fields uses the raw API field names
// (ahv_number is required; address is a nested object).
func (c *Client) CreateEmployee(ctx context.Context, fields map[string]any) (*Employee, error) {
	var out Employee
	return &out, c.Do(ctx, http.MethodPost, employeePath, nil, fields, &out)
}

// UpdateEmployee edits an employee. The payroll API uses PATCH here (not
// POST like the 2.0 resources) and answers 204 No Content, so there is no
// object to return.
func (c *Client) UpdateEmployee(ctx context.Context, id string, fields map[string]any) error {
	return c.Do(ctx, http.MethodPatch, employeePath+"/"+url.PathEscape(id), nil, fields, nil)
}

// Absence is an employee absence (4.0 API, nested under an employee).
type Absence struct {
	ID           string         `json:"id"`
	Reason       string         `json:"reason"`
	StartDate    string         `json:"start_date"`
	EndDate      string         `json:"end_date"`
	HalfDay      bool           `json:"half_day"`
	ContinuedPay PayrollDecimal `json:"continued_pay"`
	Disability   PayrollDecimal `json:"disability"`
	PaidHours    PayrollDecimal `json:"paid_hours"`

	Raw json.RawMessage `json:"-"`
}

func (a *Absence) UnmarshalJSON(data []byte) error {
	type absence Absence
	var v absence
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = Absence(v)
	a.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func absencePath(employeeID string) string {
	return employeePath + "/" + url.PathEscape(employeeID) + "/absences"
}

// ListAbsences fetches the absences of an employee for a business year.
// The businessYear parameter is required by the API.
func (c *Client) ListAbsences(ctx context.Context, employeeID string, businessYear int) ([]Absence, error) {
	q := url.Values{}
	q.Set("businessYear", strconv.Itoa(businessYear))
	var raw json.RawMessage
	if err := c.Get(ctx, absencePath(employeeID), q, &raw); err != nil {
		return nil, err
	}
	return payrollDecodeList[Absence](raw)
}

// GetAbsence fetches a single absence of an employee.
func (c *Client) GetAbsence(ctx context.Context, employeeID, absenceID string) (*Absence, error) {
	var out Absence
	return &out, c.Get(ctx, absencePath(employeeID)+"/"+url.PathEscape(absenceID), nil, &out)
}

// CreateAbsence creates an absence for an employee (reason and start_date
// are required).
func (c *Client) CreateAbsence(ctx context.Context, employeeID string, fields map[string]any) (*Absence, error) {
	var out Absence
	return &out, c.Do(ctx, http.MethodPost, absencePath(employeeID), nil, fields, &out)
}

// UpdateAbsence replaces an absence. The endpoint is a PUT and requires the
// complete object (reason, start_date, end_date, half_day, continued_pay,
// disability, paid_hours); it answers 204 No Content.
func (c *Client) UpdateAbsence(ctx context.Context, employeeID, absenceID string, fields map[string]any) error {
	return c.Do(ctx, http.MethodPut, absencePath(employeeID)+"/"+url.PathEscape(absenceID), nil, fields, nil)
}

// DeleteAbsence deletes an absence (204 No Content, no success envelope).
func (c *Client) DeleteAbsence(ctx context.Context, employeeID, absenceID string) error {
	return c.Do(ctx, http.MethodDelete, absencePath(employeeID)+"/"+url.PathEscape(absenceID), nil, nil, nil)
}

// PaystubDocument is the payload of the paystub download endpoint. Unlike
// the 2.0 document PDF endpoints (base64 in a JSON envelope) this one
// returns the PDF bytes directly.
type PaystubDocument struct {
	Name        string
	ContentType string
	Data        []byte
}

// DownloadPaystub downloads the paystub PDF of an employee for a month
// (GET .../paystub-pdf-download/{year}/{month}).
//
// The older .../paystub-pdf/{year}/{month} endpoint is deprecated: it
// answers {"location": "<uri>"} instead of the document and is therefore
// deliberately not implemented.
func (c *Client) DownloadPaystub(ctx context.Context, employeeID string, year, month int) (*PaystubDocument, error) {
	path := fmt.Sprintf("%s/%s/paystub-pdf-download/%d/%d", employeePath, url.PathEscape(employeeID), year, month)
	return c.paystubRawGet(ctx, path)
}

// paystubRawGet performs a GET that returns arbitrary bytes instead of
// JSON. Auth, verbose logging, and error mapping mirror Client.Do.
func (c *Client) paystubRawGet(ctx context.Context, path string) (*PaystubDocument, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	token, err := c.source.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "bexio-cli")
	req.Header.Set("Accept", "application/pdf")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", http.MethodGet, u.String())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(data))
	}
	if resp.StatusCode >= 400 {
		apiErr := &Error{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr) // best effort; error text may not be JSON
		return nil, apiErr
	}

	return &PaystubDocument{
		Name:        paystubNameFromDisposition(resp.Header.Get("Content-Disposition")),
		ContentType: resp.Header.Get("Content-Type"),
		Data:        data,
	}, nil
}

// paystubNameFromDisposition extracts the filename of a Content-Disposition
// header, returning "" when there is none.
func paystubNameFromDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil || params["filename"] == "" {
		return ""
	}
	return filepath.Base(params["filename"])
}
