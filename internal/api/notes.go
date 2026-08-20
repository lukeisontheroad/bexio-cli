package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Note is a bexio note (API resource "note"). project_id is read-only in
// responses; to link a project on create/update send pr_project_id.
type Note struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	EventStart string `json:"event_start"`
	Subject    string `json:"subject"`
	Info       string `json:"info"`
	ContactID  int    `json:"contact_id"`
	ProjectID  int    `json:"project_id"`
	EntryID    int    `json:"entry_id"`
	ModuleID   int    `json:"module_id"`

	Raw json.RawMessage `json:"-"`
}

func (n *Note) UnmarshalJSON(data []byte) error {
	type note Note
	var v note
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*n = Note(v)
	n.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListNotes(ctx context.Context, opts ListOptions) ([]Note, error) {
	var out []Note
	return out, c.Get(ctx, "/2.0/note", opts.values(), &out)
}

func (c *Client) SearchNotes(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Note, error) {
	var out []Note
	return out, c.Do(ctx, http.MethodPost, "/2.0/note/search", opts.values(), criteria, &out)
}

func (c *Client) GetNote(ctx context.Context, id int) (*Note, error) {
	var out Note
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/note/%d", id), nil, &out)
}

func (c *Client) CreateNote(ctx context.Context, fields map[string]any) (*Note, error) {
	var out Note
	return &out, c.Do(ctx, http.MethodPost, "/2.0/note", nil, fields, &out)
}

func (c *Client) UpdateNote(ctx context.Context, id int, fields map[string]any) (*Note, error) {
	var out Note
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/note/%d", id), nil, fields, &out)
}

func (c *Client) DeleteNote(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/note/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete note %d: API reported failure", id)
	}
	return nil
}

// Task is a bexio task (API resource "task"). project_id is read-only in
// responses; to link a project on create/update send pr_project_id. Reminder
// writes go through have_remember + remember_type_id + remember_time_id;
// responses only expose the read-only has_reminder flag.
type Task struct {
	ID             int    `json:"id"`
	UserID         int    `json:"user_id"`
	FinishDate     string `json:"finish_date"`
	Subject        string `json:"subject"`
	Info           string `json:"info"`
	ContactID      int    `json:"contact_id"`
	SubContactID   int    `json:"sub_contact_id"`
	ProjectID      int    `json:"project_id"`
	EntryID        int    `json:"entry_id"`
	ModuleID       int    `json:"module_id"`
	TodoStatusID   int    `json:"todo_status_id"`
	TodoPriorityID int    `json:"todo_priority_id"`
	// has_reminder is deliberately not decoded: the docs type it as bool
	// but the live API returns a string; Raw keeps it for -o json.
	CommunicationKindID int `json:"communication_kind_id"`

	Raw json.RawMessage `json:"-"`
}

func (t *Task) UnmarshalJSON(data []byte) error {
	type task Task
	var v task
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Task(v)
	t.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListTasks(ctx context.Context, opts ListOptions) ([]Task, error) {
	var out []Task
	return out, c.Get(ctx, "/2.0/task", opts.values(), &out)
}

func (c *Client) SearchTasks(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Task, error) {
	var out []Task
	return out, c.Do(ctx, http.MethodPost, "/2.0/task/search", opts.values(), criteria, &out)
}

func (c *Client) GetTask(ctx context.Context, id int) (*Task, error) {
	var out Task
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/task/%d", id), nil, &out)
}

func (c *Client) CreateTask(ctx context.Context, fields map[string]any) (*Task, error) {
	var out Task
	return &out, c.Do(ctx, http.MethodPost, "/2.0/task", nil, fields, &out)
}

func (c *Client) UpdateTask(ctx context.Context, id int, fields map[string]any) (*Task, error) {
	var out Task
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/task/%d", id), nil, fields, &out)
}

func (c *Client) DeleteTask(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/task/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete task %d: API reported failure", id)
	}
	return nil
}

// TodoPriority is a task priority lookup value (/2.0/todo_priority).
type TodoPriority struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (p *TodoPriority) UnmarshalJSON(data []byte) error {
	type prio TodoPriority
	var v prio
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = TodoPriority(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListTodoPriorities(ctx context.Context, opts ListOptions) ([]TodoPriority, error) {
	var out []TodoPriority
	return out, c.Get(ctx, "/2.0/todo_priority", opts.values(), &out)
}

// TodoStatus is a task status lookup value (/2.0/todo_status).
type TodoStatus struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (s *TodoStatus) UnmarshalJSON(data []byte) error {
	type status TodoStatus
	var v status
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = TodoStatus(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) ListTodoStatuses(ctx context.Context, opts ListOptions) ([]TodoStatus, error) {
	var out []TodoStatus
	return out, c.Get(ctx, "/2.0/todo_status", opts.values(), &out)
}

// Comment is a comment on a kb document
// (/2.0/{kb_document_type}/{document_id}/comment).
type Comment struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	UserID    int    `json:"user_id"`
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
	Date      string `json:"date"`
	IsPublic  bool   `json:"is_public"`

	Raw json.RawMessage `json:"-"`
}

func (cm *Comment) UnmarshalJSON(data []byte) error {
	type comment Comment
	var v comment
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*cm = Comment(v)
	cm.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// KbDocumentTypes are the document kinds that support comments.
var KbDocumentTypes = []string{"kb_offer", "kb_order", "kb_invoice"}

func commentPath(docType string, documentID int) string {
	return fmt.Sprintf("/2.0/%s/%d/comment", docType, documentID)
}

func (c *Client) ListComments(ctx context.Context, docType string, documentID int, limit, offset int) ([]Comment, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var out []Comment
	return out, c.Get(ctx, commentPath(docType, documentID), q, &out)
}

func (c *Client) GetComment(ctx context.Context, docType string, documentID, commentID int) (*Comment, error) {
	var out Comment
	return &out, c.Get(ctx, fmt.Sprintf("%s/%d", commentPath(docType, documentID), commentID), nil, &out)
}

func (c *Client) CreateComment(ctx context.Context, docType string, documentID int, fields map[string]any) (*Comment, error) {
	var out Comment
	return &out, c.Do(ctx, http.MethodPost, commentPath(docType, documentID), nil, fields, &out)
}
