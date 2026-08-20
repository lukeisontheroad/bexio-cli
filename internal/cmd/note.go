package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newNoteCmd) }
func init() { registerModule(newTaskCmd) }
func init() { registerModule(newCommentCmd) }

// noteIDCell renders a nullable id field: empty instead of 0.
func noteIDCell(id int) string {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}

//
// note
//

// newNoteCmd manages notes (API resource "note").
func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "List, view, search, and modify notes",
	}
	cmd.AddCommand(
		newNoteListCmd(),
		newNoteViewCmd(),
		newNoteSearchCmd(),
		newNoteCreateCmd(),
		newNoteUpdateCmd(),
		newNoteDeleteCmd(),
	)
	return cmd
}

var noteDetailOrder = []string{
	"id", "user_id", "subject", "info", "event_start", "contact_id",
	"project_id", "entry_id", "module_id",
}

func renderNotes(cmd *cobra.Command, notes []api.Note) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(notes))
		for i, n := range notes {
			raws[i] = n.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(notes))
	for i, n := range notes {
		rows[i] = []string{
			strconv.Itoa(n.ID),
			output.Truncate(n.Subject, 40),
			noteIDCell(n.ContactID),
			strconv.Itoa(n.UserID),
			shortDate(n.EventStart),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "subject", "contact_id", "user_id", "event_start"}, rows)
	return nil
}

func newNoteListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			notes, err := client.ListNotes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderNotes(cmd, notes)
		},
	}
	listFlags(cmd, &opts, "id, event_start, contact_id, user_id")
	return cmd
}

func newNoteViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("note", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			n, err := client.GetNote(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, n.Raw, noteDetailOrder)
		},
	}
}

func newNoteSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search notes",
		Long:  "Search notes. A bare term matches subject partially; --where clauses use raw API field names (subject, info, contact_id, user_id, event_start, module_id, entry_id).",
		Example: `  bexio note search meeting
  bexio note search --where contact_id=14
  bexio note search --where "event_start>2026-01-01" -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				criteria = append(criteria, api.SearchCriterion{
					Field: "subject", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			notes, err := client.SearchNotes(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderNotes(cmd, notes)
		},
	}
	listFlags(cmd, &opts, "id, event_start, contact_id, user_id")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed)")
	return cmd
}

// noteFieldFlags mirrors the writable API fields of POST /2.0/note.
type noteFieldFlags struct {
	userID                    int
	eventStart, subject, info string
	contactID, prProjectID    int
	entryID, moduleID         int
}

func (f *noteFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.StringVar(&f.eventStart, "event-start", "", `date/time of the note, e.g. "2026-08-21 14:20:00"`)
	fl.StringVar(&f.subject, "subject", "", "subject of the note")
	fl.StringVar(&f.info, "info", "", "note text")
	fl.IntVar(&f.contactID, "contact-id", 0, "linked contact id")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "linked project id (returned as project_id)")
	fl.IntVar(&f.entryID, "entry-id", 0, "linked module entry id")
	fl.IntVar(&f.moduleID, "module-id", 0, "linked module id")
}

func (f *noteFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "event-start", "event_start", f.eventStart)
	setIfChanged(cmd, fields, "subject", "subject", f.subject)
	setIfChanged(cmd, fields, "info", "info", f.info)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", f.prProjectID)
	setIfChanged(cmd, fields, "entry-id", "entry_id", f.entryID)
	setIfChanged(cmd, fields, "module-id", "module_id", f.moduleID)
	return fields
}

func newNoteCreateCmd() *cobra.Command {
	var fields noteFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a note",
		Long: `Create a note. --subject and --event-start are required. user_id is
required by the API and defaults to the authenticated user.`,
		Example: `  bexio note create --subject "Call follow-up" --event-start "2026-08-21 14:00:00"
  bexio note create --subject Offer --event-start "2026-08-21 09:00:00" --contact-id 14 --info "sent draft"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["subject"] == nil {
				return fmt.Errorf("--subject is required")
			}
			if payload["event_start"] == nil {
				return fmt.Errorf("--event-start is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if payload["user_id"] == nil {
				me, err := client.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolve default user_id: %w", err)
				}
				payload["user_id"] = me.ID
			}
			n, err := client.CreateNote(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), n.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created note %d (%s)\n", n.ID, n.Subject)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newNoteUpdateCmd() *cobra.Command {
	var fields noteFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a note",
		Long:  "Update a note. Only the flags you pass are changed.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("note", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			n, err := client.UpdateNote(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), n.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated note %d (%s)\n", n.ID, n.Subject)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newNoteDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("note", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteNote(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted note %d\n", id)
			return nil
		},
	}
}

//
// task
//

// newTaskCmd manages tasks (API resource "task") plus the todo_priority and
// todo_status lookups.
func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "List, view, search, and modify tasks",
	}
	cmd.AddCommand(
		newTaskListCmd(),
		newTaskViewCmd(),
		newTaskSearchCmd(),
		newTaskCreateCmd(),
		newTaskUpdateCmd(),
		newTaskDeleteCmd(),
		newTaskPriorityCmd(),
		newTaskStatusCmd(),
	)
	return cmd
}

var taskDetailOrder = []string{
	"id", "user_id", "subject", "info", "finish_date",
	"todo_status_id", "todo_status", "todo_priority_id", "todo_priority",
	"contact_id", "sub_contact_id", "project_id", "entry_id", "module_id",
	"has_reminder", "remember_type_id", "remember_time_id",
	"communication_kind_id",
}

func renderTasks(cmd *cobra.Command, tasks []api.Task) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(tasks))
		for i, t := range tasks {
			raws[i] = t.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(tasks))
	for i, t := range tasks {
		rows[i] = []string{
			strconv.Itoa(t.ID),
			output.Truncate(t.Subject, 40),
			strconv.Itoa(t.UserID),
			shortDate(t.FinishDate),
			noteIDCell(t.TodoStatusID),
			noteIDCell(t.TodoPriorityID),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "subject", "user_id", "finish_date", "status_id", "priority_id"}, rows)
	return nil
}

func newTaskListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			tasks, err := client.ListTasks(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderTasks(cmd, tasks)
		},
	}
	listFlags(cmd, &opts, "id, finish_date")
	return cmd
}

func newTaskViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a task (status/priority ids resolved to names)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("task", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			t, err := client.GetTask(cmd.Context(), id)
			if err != nil {
				return err
			}
			raw := t.Raw
			if flagOutput != "json" {
				raw = taskEnrichDetail(cmd, client, t)
			}
			return renderDetail(cmd, raw, taskDetailOrder)
		},
	}
}

// taskEnrichDetail adds todo_status / todo_priority name fields to the raw
// task object for the table view. Best effort: lookup failures are ignored.
func taskEnrichDetail(cmd *cobra.Command, client *api.Client, t *api.Task) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(t.Raw, &m); err != nil {
		return t.Raw
	}
	if t.TodoStatusID != 0 {
		if statuses, err := client.ListTodoStatuses(cmd.Context(), api.ListOptions{}); err == nil {
			for _, s := range statuses {
				if s.ID == t.TodoStatusID {
					m["todo_status"] = s.Name
				}
			}
		}
	}
	if t.TodoPriorityID != 0 {
		if prios, err := client.ListTodoPriorities(cmd.Context(), api.ListOptions{}); err == nil {
			for _, p := range prios {
				if p.ID == t.TodoPriorityID {
					m["todo_priority"] = p.Name
				}
			}
		}
	}
	enriched, err := json.Marshal(m)
	if err != nil {
		return t.Raw
	}
	return enriched
}

func newTaskSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search tasks",
		Long:  "Search tasks. A bare term matches subject partially; --where clauses use raw API field names (subject, updated_at, user_id, contact_id, todo_status_id, module_id, entry_id).",
		Example: `  bexio task search offer
  bexio task search --where todo_status_id=1
  bexio task search --where user_id=4 --where "finish_date<2026-09-01"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				criteria = append(criteria, api.SearchCriterion{
					Field: "subject", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			tasks, err := client.SearchTasks(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderTasks(cmd, tasks)
		},
	}
	listFlags(cmd, &opts, "id, finish_date")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed)")
	return cmd
}

// taskFieldFlags mirrors the writable API fields of POST /2.0/task.
type taskFieldFlags struct {
	userID                         int
	finishDate, subject, info      string
	contactID, subContactID        int
	prProjectID, entryID, moduleID int
	todoStatusID, todoPriorityID   int
	haveRemember                   bool
	rememberTypeID, rememberTimeID int
	communicationKindID            int
}

func (f *taskFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.StringVar(&f.finishDate, "finish-date", "", `due date/time, e.g. "2026-09-01 09:00:00"`)
	fl.StringVar(&f.subject, "subject", "", "subject of the task")
	fl.StringVar(&f.info, "info", "", "task description")
	fl.IntVar(&f.contactID, "contact-id", 0, "linked contact id")
	fl.IntVar(&f.subContactID, "sub-contact-id", 0, "linked sub contact (person) id")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "linked project id (returned as project_id)")
	fl.IntVar(&f.entryID, "entry-id", 0, "linked module entry id")
	fl.IntVar(&f.moduleID, "module-id", 0, "linked module id")
	fl.IntVar(&f.todoStatusID, "todo-status-id", 0, "status id (list with: bexio task status list)")
	fl.IntVar(&f.todoPriorityID, "todo-priority-id", 0, "priority id (list with: bexio task priority list)")
	fl.BoolVar(&f.haveRemember, "have-remember", false, "enable a reminder (requires --remember-type-id and --remember-time-id)")
	fl.IntVar(&f.rememberTypeID, "remember-type-id", 0, "reminder type id")
	fl.IntVar(&f.rememberTimeID, "remember-time-id", 0, "reminder time id")
	fl.IntVar(&f.communicationKindID, "communication-kind-id", 0, "communication kind id")
}

func (f *taskFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "finish-date", "finish_date", f.finishDate)
	setIfChanged(cmd, fields, "subject", "subject", f.subject)
	setIfChanged(cmd, fields, "info", "info", f.info)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "sub-contact-id", "sub_contact_id", f.subContactID)
	setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", f.prProjectID)
	setIfChanged(cmd, fields, "entry-id", "entry_id", f.entryID)
	setIfChanged(cmd, fields, "module-id", "module_id", f.moduleID)
	setIfChanged(cmd, fields, "todo-status-id", "todo_status_id", f.todoStatusID)
	setIfChanged(cmd, fields, "todo-priority-id", "todo_priority_id", f.todoPriorityID)
	setIfChanged(cmd, fields, "have-remember", "have_remember", f.haveRemember)
	setIfChanged(cmd, fields, "remember-type-id", "remember_type_id", f.rememberTypeID)
	setIfChanged(cmd, fields, "remember-time-id", "remember_time_id", f.rememberTimeID)
	setIfChanged(cmd, fields, "communication-kind-id", "communication_kind_id", f.communicationKindID)
	return fields
}

func newTaskCreateCmd() *cobra.Command {
	var fields taskFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		Long: `Create a task. --subject is required. user_id is required by the API and
defaults to the authenticated user.`,
		Example: `  bexio task create --subject "Send documents"
  bexio task create --subject "Call back" --contact-id 14 --finish-date "2026-09-01 09:00:00" --todo-priority-id 2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["subject"] == nil {
				return fmt.Errorf("--subject is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if payload["user_id"] == nil {
				me, err := client.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolve default user_id: %w", err)
				}
				payload["user_id"] = me.ID
			}
			t, err := client.CreateTask(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), t.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created task %d (%s)\n", t.ID, t.Subject)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newTaskUpdateCmd() *cobra.Command {
	var fields taskFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Long:  "Update a task. Only the flags you pass are changed.",
		Example: `  bexio task update 5 --todo-status-id 3
  bexio task update 5 --finish-date "2026-09-15 12:00:00"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("task", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			t, err := client.UpdateTask(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), t.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated task %d (%s)\n", t.ID, t.Subject)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("task", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteTask(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted task %d\n", id)
			return nil
		},
	}
}

func newTaskPriorityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "priority",
		Short: "Task priority lookup values",
	}
	var opts api.ListOptions
	list := &cobra.Command{
		Use:   "list",
		Short: "List task priorities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			prios, err := client.ListTodoPriorities(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				raws := make([]json.RawMessage, len(prios))
				for i, p := range prios {
					raws[i] = p.Raw
				}
				return output.JSON(cmd.OutOrStdout(), raws)
			}
			rows := make([][]string, len(prios))
			for i, p := range prios {
				rows[i] = []string{strconv.Itoa(p.ID), p.Name}
			}
			output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
			return nil
		},
	}
	listFlags(list, &opts, "id, name")
	cmd.AddCommand(list)
	return cmd
}

func newTaskStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Task status lookup values",
	}
	var opts api.ListOptions
	list := &cobra.Command{
		Use:   "list",
		Short: "List task statuses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			statuses, err := client.ListTodoStatuses(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				raws := make([]json.RawMessage, len(statuses))
				for i, s := range statuses {
					raws[i] = s.Raw
				}
				return output.JSON(cmd.OutOrStdout(), raws)
			}
			rows := make([][]string, len(statuses))
			for i, s := range statuses {
				rows[i] = []string{strconv.Itoa(s.ID), s.Name}
			}
			output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
			return nil
		},
	}
	listFlags(list, &opts, "id, name")
	cmd.AddCommand(list)
	return cmd
}

//
// comment
//

// newCommentCmd manages comments on kb documents
// (/2.0/{kb_document_type}/{document_id}/comment).
func newCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Comments on quotes, orders, and invoices (kb documents)",
		Long: `Comments on kb documents. The first argument is always the document type
(kb_offer, kb_order, or kb_invoice), the second the document id.`,
	}
	cmd.AddCommand(
		newCommentListCmd(),
		newCommentViewCmd(),
		newCommentCreateCmd(),
	)
	return cmd
}

var commentDetailOrder = []string{
	"id", "text", "user_id", "user_name", "user_email", "date", "is_public",
	"image_path",
}

// commentDocType validates the kb_document_type positional argument.
func commentDocType(arg string) (string, error) {
	for _, t := range api.KbDocumentTypes {
		if arg == t {
			return arg, nil
		}
	}
	return "", fmt.Errorf("invalid document type %q (want kb_offer, kb_order, or kb_invoice)", arg)
}

func renderComments(cmd *cobra.Command, comments []api.Comment) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(comments))
		for i, c := range comments {
			raws[i] = c.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(comments))
	for i, c := range comments {
		rows[i] = []string{
			strconv.Itoa(c.ID),
			c.UserName,
			output.Truncate(c.Text, 60),
			strconv.FormatBool(c.IsPublic),
			shortDate(c.Date),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "user", "text", "public", "date"}, rows)
	return nil
}

func newCommentListCmd() *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list <kb_offer|kb_order|kb_invoice> <document-id>",
		Short: "List comments of a kb document",
		Example: `  bexio comment list kb_invoice 17
  bexio comment list kb_order 5 -o json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			docType, err := commentDocType(args[0])
			if err != nil {
				return err
			}
			docID, err := parseID("document", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			comments, err := client.ListComments(cmd.Context(), docType, docID, limit, offset)
			if err != nil {
				return err
			}
			return renderComments(cmd, comments)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum number of results (API max 2000)")
	cmd.Flags().IntVar(&offset, "offset", 0, "skip this many results (pagination)")
	return cmd
}

func newCommentViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <kb_offer|kb_order|kb_invoice> <document-id> <comment-id>",
		Short: "Show a comment of a kb document",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			docType, err := commentDocType(args[0])
			if err != nil {
				return err
			}
			docID, err := parseID("document", args[1])
			if err != nil {
				return err
			}
			commentID, err := parseID("comment", args[2])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.GetComment(cmd.Context(), docType, docID, commentID)
			if err != nil {
				return err
			}
			return renderDetail(cmd, c.Raw, commentDetailOrder)
		},
	}
}

func newCommentCreateCmd() *cobra.Command {
	var text string
	var userID int
	var isPublic bool
	cmd := &cobra.Command{
		Use:   "create <kb_offer|kb_order|kb_invoice> <document-id>",
		Short: "Create a comment on a kb document",
		Long: `Create a comment on a kb document. --text is required. user_id defaults
to the authenticated user. --is-public makes the comment visible to the
document recipient.`,
		Example: `  bexio comment create kb_order 5 --text "Shipment delayed by a week"
  bexio comment create kb_invoice 17 --text "Payment reminder sent" --is-public`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			docType, err := commentDocType(args[0])
			if err != nil {
				return err
			}
			docID, err := parseID("document", args[1])
			if err != nil {
				return err
			}
			if text == "" {
				return fmt.Errorf("--text is required")
			}
			fields := map[string]any{"text": text}
			setIfChanged(cmd, fields, "user-id", "user_id", userID)
			setIfChanged(cmd, fields, "is-public", "is_public", isPublic)
			client, err := newClient()
			if err != nil {
				return err
			}
			if fields["user_id"] == nil {
				me, err := client.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolve default user_id: %w", err)
				}
				fields["user_id"] = me.ID
			}
			c, err := client.CreateComment(cmd.Context(), docType, docID, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), c.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created comment %d on %s %d\n", c.ID, docType, docID)
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "comment text (required)")
	cmd.Flags().IntVar(&userID, "user-id", 0, "user id (defaults to the authenticated user)")
	cmd.Flags().BoolVar(&isPublic, "is-public", false, "make the comment visible to the document recipient")
	return cmd
}
