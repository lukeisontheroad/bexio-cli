package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Project is a bexio project (2.0 API resource "pr_project"). Raw preserves
// the full API object for --output json.
type Project struct {
	ID              int    `json:"id"`
	Nr              string `json:"nr"`
	Name            string `json:"name"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	PrStateID       int    `json:"pr_state_id"`
	PrProjectTypeID int    `json:"pr_project_type_id"`
	ContactID       int    `json:"contact_id"`
	// pr_invoice_type_amount is deliberately not decoded: the docs type it
	// as string but the live API returns a number; Raw keeps it for -o json.
	UserID int `json:"user_id"`

	Raw json.RawMessage `json:"-"`
}

func (p *Project) UnmarshalJSON(data []byte) error {
	type project Project // avoid recursion
	var v project
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = Project(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListProjects fetches /2.0/pr_project.
func (c *Client) ListProjects(ctx context.Context, opts ListOptions) ([]Project, error) {
	var out []Project
	return out, c.Get(ctx, "/2.0/pr_project", opts.values(), &out)
}

// SearchProjects posts to /2.0/pr_project/search.
func (c *Client) SearchProjects(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Project, error) {
	var out []Project
	return out, c.Do(ctx, http.MethodPost, "/2.0/pr_project/search", opts.values(), criteria, &out)
}

// GetProject fetches a single project.
func (c *Client) GetProject(ctx context.Context, id int) (*Project, error) {
	var out Project
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/pr_project/%d", id), nil, &out)
}

// CreateProject creates a project. fields uses the raw API field names.
func (c *Client) CreateProject(ctx context.Context, fields map[string]any) (*Project, error) {
	var out Project
	return &out, c.Do(ctx, http.MethodPost, "/2.0/pr_project", nil, fields, &out)
}

// UpdateProject edits a project (only the provided fields change).
func (c *Client) UpdateProject(ctx context.Context, id int, fields map[string]any) (*Project, error) {
	var out Project
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/pr_project/%d", id), nil, fields, &out)
}

// DeleteProject permanently deletes a project (use ArchiveProject to keep it).
func (c *Client) DeleteProject(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/pr_project/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete project %d: API reported failure", id)
	}
	return nil
}

// ArchiveProject archives a project (reversible via ReactivateProject).
func (c *Client) ArchiveProject(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/pr_project/%d/archive", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("archive project %d: API reported failure", id)
	}
	return nil
}

// ReactivateProject un-archives a project.
func (c *Client) ReactivateProject(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/pr_project/%d/reactivate", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("reactivate project %d: API reported failure", id)
	}
	return nil
}

// ProjectLookupItem is an id/name entry of the read-only lookup resources
// pr_project_state, pr_project_type, and timesheet_status.
type ProjectLookupItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (l *ProjectLookupItem) UnmarshalJSON(data []byte) error {
	type item ProjectLookupItem
	var v item
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*l = ProjectLookupItem(v)
	l.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListProjectStates fetches /2.0/pr_project_state.
func (c *Client) ListProjectStates(ctx context.Context) ([]ProjectLookupItem, error) {
	var out []ProjectLookupItem
	return out, c.Get(ctx, "/2.0/pr_project_state", nil, &out)
}

// ListProjectTypes fetches /2.0/pr_project_type.
func (c *Client) ListProjectTypes(ctx context.Context) ([]ProjectLookupItem, error) {
	var out []ProjectLookupItem
	return out, c.Get(ctx, "/2.0/pr_project_type", nil, &out)
}

// ListTimesheetStatuses fetches /2.0/timesheet_status.
func (c *Client) ListTimesheetStatuses(ctx context.Context) ([]ProjectLookupItem, error) {
	var out []ProjectLookupItem
	return out, c.Get(ctx, "/2.0/timesheet_status", nil, &out)
}

// Milestone is a project milestone (3.0 API, nested under a project).
type Milestone struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	EndDate             string `json:"end_date"`
	Comment             string `json:"comment"`
	PrParentMilestoneID int    `json:"pr_parent_milestone_id"`

	Raw json.RawMessage `json:"-"`
}

func (m *Milestone) UnmarshalJSON(data []byte) error {
	type milestone Milestone
	var v milestone
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = Milestone(v)
	m.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func milestonePath(projectID int) string {
	return fmt.Sprintf("/3.0/projects/%d/milestones", projectID)
}

// pagingValues builds the limit/offset query of the 3.0 list endpoints
// (order_by and show_archived are 2.0-only).
func pagingValues(limit, offset int) url.Values {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return q
}

// ListMilestones fetches /3.0/projects/{id}/milestones.
func (c *Client) ListMilestones(ctx context.Context, projectID, limit, offset int) ([]Milestone, error) {
	var out []Milestone
	return out, c.Get(ctx, milestonePath(projectID), pagingValues(limit, offset), &out)
}

// GetMilestone fetches a single milestone of a project.
func (c *Client) GetMilestone(ctx context.Context, projectID, id int) (*Milestone, error) {
	var out Milestone
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", milestonePath(projectID), id), nil, &out)
}

// CreateMilestone creates a milestone under a project.
func (c *Client) CreateMilestone(ctx context.Context, projectID int, fields map[string]any) (*Milestone, error) {
	var out Milestone
	return &out, c.Do(ctx, http.MethodPost, milestonePath(projectID), nil, fields, &out)
}

// UpdateMilestone edits a milestone (POST, only the provided fields change).
func (c *Client) UpdateMilestone(ctx context.Context, projectID, id int, fields map[string]any) (*Milestone, error) {
	var out Milestone
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%d", milestonePath(projectID), id), nil, fields, &out)
}

// DeleteMilestone deletes a milestone.
func (c *Client) DeleteMilestone(ctx context.Context, projectID, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", milestonePath(projectID), id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete milestone %d: API reported failure", id)
	}
	return nil
}

// ProjectPackage is a project work package (3.0 API, nested under a project).
type ProjectPackage struct {
	ID                   int     `json:"id"`
	Name                 string  `json:"name"`
	SpentTimeInHours     float64 `json:"spent_time_in_hours"`
	EstimatedTimeInHours float64 `json:"estimated_time_in_hours"`
	Comment              string  `json:"comment"`
	PrMilestoneID        int     `json:"pr_milestone_id"`

	Raw json.RawMessage `json:"-"`
}

func (p *ProjectPackage) UnmarshalJSON(data []byte) error {
	type pkg ProjectPackage
	var v pkg
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = ProjectPackage(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func projectPackagePath(projectID int) string {
	return fmt.Sprintf("/3.0/projects/%d/packages", projectID)
}

// ListProjectPackages fetches /3.0/projects/{id}/packages.
func (c *Client) ListProjectPackages(ctx context.Context, projectID, limit, offset int) ([]ProjectPackage, error) {
	var out []ProjectPackage
	return out, c.Get(ctx, projectPackagePath(projectID), pagingValues(limit, offset), &out)
}

// GetProjectPackage fetches a single work package of a project.
func (c *Client) GetProjectPackage(ctx context.Context, projectID, id int) (*ProjectPackage, error) {
	var out ProjectPackage
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", projectPackagePath(projectID), id), nil, &out)
}

// CreateProjectPackage creates a work package under a project.
func (c *Client) CreateProjectPackage(ctx context.Context, projectID int, fields map[string]any) (*ProjectPackage, error) {
	var out ProjectPackage
	return &out, c.Do(ctx, http.MethodPost, projectPackagePath(projectID), nil, fields, &out)
}

// UpdateProjectPackage edits a work package (PATCH — unlike milestones,
// which use POST).
func (c *Client) UpdateProjectPackage(ctx context.Context, projectID, id int, fields map[string]any) (*ProjectPackage, error) {
	var out ProjectPackage
	return &out, c.Do(ctx, http.MethodPatch, fmt.Sprintf("%s/%d", projectPackagePath(projectID), id), nil, fields, &out)
}

// DeleteProjectPackage deletes a work package.
func (c *Client) DeleteProjectPackage(ctx context.Context, projectID, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", projectPackagePath(projectID), id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete work package %d: API reported failure", id)
	}
	return nil
}

// Timesheet is a time tracking entry (2.0 API resource "timesheet").
type Timesheet struct {
	ID              int    `json:"id"`
	UserID          int    `json:"user_id"`
	StatusID        int    `json:"status_id"`
	ClientServiceID int    `json:"client_service_id"`
	Text            string `json:"text"`
	AllowableBill   bool   `json:"allowable_bill"`
	ContactID       int    `json:"contact_id"`
	PrProjectID     int    `json:"pr_project_id"`
	Date            string `json:"date"`
	Duration        string `json:"duration"`
	Running         bool   `json:"running"`

	Raw json.RawMessage `json:"-"`
}

func (t *Timesheet) UnmarshalJSON(data []byte) error {
	type timesheet Timesheet
	var v timesheet
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Timesheet(v)
	t.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListTimesheets fetches /2.0/timesheet.
func (c *Client) ListTimesheets(ctx context.Context, opts ListOptions) ([]Timesheet, error) {
	var out []Timesheet
	return out, c.Get(ctx, "/2.0/timesheet", opts.values(), &out)
}

// SearchTimesheets posts to /2.0/timesheet/search.
func (c *Client) SearchTimesheets(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Timesheet, error) {
	var out []Timesheet
	return out, c.Do(ctx, http.MethodPost, "/2.0/timesheet/search", opts.values(), criteria, &out)
}

// GetTimesheet fetches a single timesheet.
func (c *Client) GetTimesheet(ctx context.Context, id int) (*Timesheet, error) {
	var out Timesheet
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/timesheet/%d", id), nil, &out)
}

// CreateTimesheet creates a timesheet. fields uses the raw API field names,
// including the nested "tracking" object ({type: duration|range, ...}).
func (c *Client) CreateTimesheet(ctx context.Context, fields map[string]any) (*Timesheet, error) {
	var out Timesheet
	return &out, c.Do(ctx, http.MethodPost, "/2.0/timesheet", nil, fields, &out)
}

// UpdateTimesheet edits a timesheet (only the provided fields change).
func (c *Client) UpdateTimesheet(ctx context.Context, id int, fields map[string]any) (*Timesheet, error) {
	var out Timesheet
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/timesheet/%d", id), nil, fields, &out)
}

// DeleteTimesheet deletes a timesheet.
func (c *Client) DeleteTimesheet(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/timesheet/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete timesheet %d: API reported failure", id)
	}
	return nil
}

// BusinessActivity is a business activity (2.0 API resource
// "client_service"), referenced by timesheets via client_service_id.
type BusinessActivity struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	DefaultIsBillable bool   `json:"default_is_billable"`
	// The docs type default_price_per_hour as number, the live API returns
	// a decimal string.
	DefaultPricePerHour string `json:"default_price_per_hour"`
	AccountID           int    `json:"account_id"`

	Raw json.RawMessage `json:"-"`
}

func (b *BusinessActivity) UnmarshalJSON(data []byte) error {
	type activity BusinessActivity
	var v activity
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = BusinessActivity(v)
	b.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListBusinessActivities fetches /2.0/client_service.
func (c *Client) ListBusinessActivities(ctx context.Context, opts ListOptions) ([]BusinessActivity, error) {
	var out []BusinessActivity
	return out, c.Get(ctx, "/2.0/client_service", opts.values(), &out)
}

// SearchBusinessActivities posts to /2.0/client_service/search.
func (c *Client) SearchBusinessActivities(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]BusinessActivity, error) {
	var out []BusinessActivity
	return out, c.Do(ctx, http.MethodPost, "/2.0/client_service/search", opts.values(), criteria, &out)
}

// CreateBusinessActivity creates a business activity.
func (c *Client) CreateBusinessActivity(ctx context.Context, fields map[string]any) (*BusinessActivity, error) {
	var out BusinessActivity
	return &out, c.Do(ctx, http.MethodPost, "/2.0/client_service", nil, fields, &out)
}
