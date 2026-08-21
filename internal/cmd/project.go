package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	registerModule(newProjectCmd)
	registerModule(newTimesheetCmd)
	registerModule(newBusinessActivityCmd)
}

// ---------------------------------------------------------------------------
// pr-project (API resource "pr_project", plus the 3.0 milestones/packages)
// ---------------------------------------------------------------------------

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"pr-project"},
		Short:   "List, view, search, and modify projects",
	}
	cmd.AddCommand(
		newProjectListCmd(),
		newProjectViewCmd(),
		newProjectSearchCmd(),
		newProjectCreateCmd(),
		newProjectUpdateCmd(),
		newProjectDeleteCmd(),
		newProjectArchiveCmd(),
		newProjectReactivateCmd(),
		newProjectStateCmd(),
		newProjectTypeCmd(),
		newProjectMilestoneCmd(),
		newProjectPackageCmd(),
	)
	return cmd
}

var projectDetailOrder = []string{
	"id", "uuid", "nr", "name", "start_date", "end_date", "comment",
	"pr_state_id", "pr_project_type_id", "contact_id", "contact_sub_id",
	"pr_invoice_type_id", "pr_invoice_type_amount", "pr_budget_type_id",
	"pr_budget_type_amount", "user_id",
}

func renderProjects(cmd *cobra.Command, projects []api.Project) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(projects))
		for i, p := range projects {
			raws[i] = p.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(projects))
	for i, p := range projects {
		rows[i] = []string{
			strconv.Itoa(p.ID),
			p.Nr,
			output.Truncate(p.Name, 40),
			strconv.Itoa(p.ContactID),
			strconv.Itoa(p.PrStateID),
			shortDate(p.StartDate),
			shortDate(p.EndDate),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "nr", "name", "contact_id", "state", "start", "end"}, rows)
	return nil
}

func newProjectListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Example: `  bexio project list
  bexio project list --archived
  bexio project list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			projects, err := client.ListProjects(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderProjects(cmd, projects)
		},
	}
	listFlags(cmd, &opts, "id, nr, name, start_date, end_date")
	cmd.Flags().BoolVar(&opts.ShowArchived, "archived", false, "show archived projects only")
	return cmd
}

func newProjectViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetProject(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, projectDetailOrder)
		},
	}
}

func newProjectSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search projects",
		Long: `Search projects. A bare term matches name partially. --where clauses use
the raw API field names and add AND conditions (see "bexio contact search
--help" for operators). Searchable fields include: name, contact_id,
pr_state_id.`,
		Example: `  bexio project search Website
  bexio project search --where contact_id=17
  bexio project search --where pr_state_id=1 -o json`,
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
					Field: "name", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			projects, err := client.SearchProjects(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderProjects(cmd, projects)
		},
	}
	listFlags(cmd, &opts, "id, nr, name, start_date, end_date")
	cmd.Flags().BoolVar(&opts.ShowArchived, "archived", false, "search archived projects only")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

// projectFieldFlags mirrors the API payload fields of POST /2.0/pr_project.
type projectFieldFlags struct {
	name, documentNr            string
	startDate, endDate, comment string
	prStateID, prProjectTypeID  int
	contactID, contactSubID     int
	prInvoiceTypeID             int
	prInvoiceTypeAmount         string
	prBudgetTypeID              int
	prBudgetTypeAmount          string
	userID                      int
}

func (f *projectFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "project name")
	fl.StringVar(&f.documentNr, "document-nr", "", "project number (only if automatic numbering is deactivated)")
	fl.StringVar(&f.startDate, "start-date", "", "start date (YYYY-MM-DD)")
	fl.StringVar(&f.endDate, "end-date", "", "end date (YYYY-MM-DD)")
	fl.StringVar(&f.comment, "comment", "", "comment")
	fl.IntVar(&f.prStateID, "pr-state-id", 0, "project state id (list with: bexio project state list)")
	fl.IntVar(&f.prProjectTypeID, "pr-project-type-id", 0, "project type id (list with: bexio project type list)")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id")
	fl.IntVar(&f.contactSubID, "contact-sub-id", 0, "sub contact id")
	fl.IntVar(&f.prInvoiceTypeID, "pr-invoice-type-id", 0, "invoice type id (1 hourly/service, 2 hourly/employee, 3 hourly/project, 4 fix)")
	fl.StringVar(&f.prInvoiceTypeAmount, "pr-invoice-type-amount", "", `invoice type amount (e.g. "230.00"; invoice types 3 and 4 only)`)
	fl.IntVar(&f.prBudgetTypeID, "pr-budget-type-id", 0, "budget type id (1 costs, 2 hours, 3 per service, 4 per employee)")
	fl.StringVar(&f.prBudgetTypeAmount, "pr-budget-type-amount", "", `budget type amount (e.g. "200.00"; budget types 1 and 2 only)`)
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
}

func (f *projectFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "name", "name", f.name)
	setIfChanged(cmd, fields, "document-nr", "document_nr", f.documentNr)
	setIfChanged(cmd, fields, "start-date", "start_date", f.startDate)
	setIfChanged(cmd, fields, "end-date", "end_date", f.endDate)
	setIfChanged(cmd, fields, "comment", "comment", f.comment)
	setIfChanged(cmd, fields, "pr-state-id", "pr_state_id", f.prStateID)
	setIfChanged(cmd, fields, "pr-project-type-id", "pr_project_type_id", f.prProjectTypeID)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", f.contactSubID)
	setIfChanged(cmd, fields, "pr-invoice-type-id", "pr_invoice_type_id", f.prInvoiceTypeID)
	setIfChanged(cmd, fields, "pr-invoice-type-amount", "pr_invoice_type_amount", f.prInvoiceTypeAmount)
	setIfChanged(cmd, fields, "pr-budget-type-id", "pr_budget_type_id", f.prBudgetTypeID)
	setIfChanged(cmd, fields, "pr-budget-type-amount", "pr_budget_type_amount", f.prBudgetTypeAmount)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	return fields
}

func newProjectCreateCmd() *cobra.Command {
	var fields projectFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Long: `Create a project. --name, --contact-id, --pr-state-id, and
--pr-project-type-id are required. user_id is required by the API and
defaults to the authenticated user.`,
		Example: `  bexio project create --name "Website Relaunch" --contact-id 17 \
      --pr-state-id 1 --pr-project-type-id 1`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			for flag, field := range map[string]string{
				"name": "name", "contact-id": "contact_id",
				"pr-state-id": "pr_state_id", "pr-project-type-id": "pr_project_type_id",
			} {
				if payload[field] == nil {
					return fmt.Errorf("--%s is required", flag)
				}
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
			p, err := client.CreateProject(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created project %d (%s)\n", p.ID, p.Name)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newProjectUpdateCmd() *cobra.Command {
	var fields projectFieldFlags
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update fields of a project",
		Long:    "Update a project. Only the flags you pass are changed.",
		Example: `  bexio project update 3 --pr-state-id 3 --end-date 2026-12-31`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("project", args[0])
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
			p, err := client.UpdateProject(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated project %d (%s)\n", p.ID, p.Name)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newProjectDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a project permanently",
		Long:  "Delete a project permanently. Use `bexio project archive` to keep it recoverable.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteProject(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted project %d\n", id)
			return nil
		},
	}
}

func newProjectArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.ArchiveProject(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived project %d (undo with `bexio project reactivate %d`)\n", id, id)
			return nil
		},
	}
}

func newProjectReactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reactivate <id>",
		Short: "Un-archive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.ReactivateProject(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reactivated project %d\n", id)
			return nil
		},
	}
}

func renderProjectLookup(cmd *cobra.Command, items []api.ProjectLookupItem) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(items))
		for i, it := range items {
			raws[i] = it.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{strconv.Itoa(it.ID), it.Name}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
	return nil
}

// projectLookupListCmd builds a list-only subcommand for one of the
// read-only lookup resources (pr_project_state, pr_project_type,
// timesheet_status).
func projectLookupListCmd(short string, fetch func(*cobra.Command) ([]api.ProjectLookupItem, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			items, err := fetch(cmd)
			if err != nil {
				return err
			}
			return renderProjectLookup(cmd, items)
		},
	}
}

func newProjectStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Project states (API resource pr_project_state, read-only)",
	}
	cmd.AddCommand(projectLookupListCmd("List project states", func(cmd *cobra.Command) ([]api.ProjectLookupItem, error) {
		client, err := newClient()
		if err != nil {
			return nil, err
		}
		return client.ListProjectStates(cmd.Context())
	}))
	return cmd
}

func newProjectTypeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "type",
		Short: "Project types (API resource pr_project_type, read-only)",
	}
	cmd.AddCommand(projectLookupListCmd("List project types", func(cmd *cobra.Command) ([]api.ProjectLookupItem, error) {
		client, err := newClient()
		if err != nil {
			return nil, err
		}
		return client.ListProjectTypes(cmd.Context())
	}))
	return cmd
}

// ---------------------------------------------------------------------------
// pr-project milestone (3.0 API /3.0/projects/{id}/milestones)
// ---------------------------------------------------------------------------

func newProjectMilestoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Manage the milestones of a project (3.0 API)",
	}
	cmd.AddCommand(
		newMilestoneListCmd(),
		newMilestoneViewCmd(),
		newMilestoneCreateCmd(),
		newMilestoneUpdateCmd(),
		newMilestoneDeleteCmd(),
	)
	return cmd
}

var milestoneDetailOrder = []string{"id", "name", "end_date", "comment", "pr_parent_milestone_id"}

func renderMilestones(cmd *cobra.Command, milestones []api.Milestone) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(milestones))
		for i, m := range milestones {
			raws[i] = m.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(milestones))
	for i, m := range milestones {
		rows[i] = []string{
			strconv.Itoa(m.ID),
			output.Truncate(m.Name, 40),
			m.EndDate,
			output.Truncate(m.Comment, 40),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name", "end_date", "comment"}, rows)
	return nil
}

type milestoneFieldFlags struct {
	name, endDate, comment string
	prParentMilestoneID    int
}

func (f *milestoneFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "milestone name")
	fl.StringVar(&f.endDate, "end-date", "", "end date (YYYY-MM-DD)")
	fl.StringVar(&f.comment, "comment", "", "description")
	fl.IntVar(&f.prParentMilestoneID, "pr-parent-milestone-id", 0, "higher level milestone id")
}

func (f *milestoneFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "name", "name", f.name)
	setIfChanged(cmd, fields, "end-date", "end_date", f.endDate)
	setIfChanged(cmd, fields, "comment", "comment", f.comment)
	setIfChanged(cmd, fields, "pr-parent-milestone-id", "pr_parent_milestone_id", f.prParentMilestoneID)
	return fields
}

// paging30Flags adds the limit/offset flags of the 3.0 list endpoints
// (which support no order_by).
func paging30Flags(cmd *cobra.Command, limit, offset *int) {
	cmd.Flags().IntVar(limit, "limit", 100, "maximum number of results (API max 2000)")
	cmd.Flags().IntVar(offset, "offset", 0, "skip this many results (pagination)")
}

func newMilestoneListCmd() *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:     "list <project-id>",
		Short:   "List the milestones of a project",
		Example: `  bexio project milestone list 3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			milestones, err := client.ListMilestones(cmd.Context(), projectID, limit, offset)
			if err != nil {
				return err
			}
			return renderMilestones(cmd, milestones)
		},
	}
	paging30Flags(cmd, &limit, &offset)
	return cmd
}

func newMilestoneViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <project-id> <milestone-id>",
		Short: "Show a milestone",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("milestone", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			m, err := client.GetMilestone(cmd.Context(), projectID, id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, m.Raw, milestoneDetailOrder)
		},
	}
}

func newMilestoneCreateCmd() *cobra.Command {
	var fields milestoneFieldFlags
	cmd := &cobra.Command{
		Use:     "create <project-id>",
		Short:   "Create a milestone",
		Example: `  bexio project milestone create 3 --name "Go live" --end-date 2026-12-01`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["name"] == nil {
				return fmt.Errorf("--name is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			m, err := client.CreateMilestone(cmd.Context(), projectID, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), m.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created milestone %d (%s) for project %d\n", m.ID, m.Name, projectID)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newMilestoneUpdateCmd() *cobra.Command {
	var fields milestoneFieldFlags
	cmd := &cobra.Command{
		Use:   "update <project-id> <milestone-id>",
		Short: "Update a milestone",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("milestone", args[1])
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
			m, err := client.UpdateMilestone(cmd.Context(), projectID, id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), m.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated milestone %d (%s)\n", m.ID, m.Name)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newMilestoneDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project-id> <milestone-id>",
		Short: "Delete a milestone",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("milestone", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteMilestone(cmd.Context(), projectID, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted milestone %d\n", id)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// pr-project package (3.0 API /3.0/projects/{id}/packages)
// ---------------------------------------------------------------------------

func newProjectPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Manage the work packages of a project (3.0 API)",
	}
	cmd.AddCommand(
		newProjectPackageListCmd(),
		newProjectPackageViewCmd(),
		newProjectPackageCreateCmd(),
		newProjectPackageUpdateCmd(),
		newProjectPackageDeleteCmd(),
	)
	return cmd
}

var projectPackageDetailOrder = []string{
	"id", "name", "spent_time_in_hours", "estimated_time_in_hours",
	"comment", "pr_milestone_id",
}

func renderProjectPackages(cmd *cobra.Command, pkgs []api.ProjectPackage) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(pkgs))
		for i, p := range pkgs {
			raws[i] = p.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(pkgs))
	for i, p := range pkgs {
		rows[i] = []string{
			strconv.Itoa(p.ID),
			output.Truncate(p.Name, 40),
			strconv.FormatFloat(p.SpentTimeInHours, 'f', -1, 64),
			strconv.FormatFloat(p.EstimatedTimeInHours, 'f', -1, 64),
			strconv.Itoa(p.PrMilestoneID),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name", "spent_h", "estimated_h", "milestone"}, rows)
	return nil
}

type projectPackageFieldFlags struct {
	name, comment              string
	spentHours, estimatedHours float64
	prMilestoneID              int
}

func (f *projectPackageFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "work package name")
	fl.Float64Var(&f.spentHours, "spent-time-in-hours", 0, "time spent, in hours (e.g. 0.5)")
	fl.Float64Var(&f.estimatedHours, "estimated-time-in-hours", 0, "estimated time, in hours (e.g. 1.75)")
	fl.StringVar(&f.comment, "comment", "", "description")
	fl.IntVar(&f.prMilestoneID, "pr-milestone-id", 0, "milestone id")
}

func (f *projectPackageFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "name", "name", f.name)
	setIfChanged(cmd, fields, "spent-time-in-hours", "spent_time_in_hours", f.spentHours)
	setIfChanged(cmd, fields, "estimated-time-in-hours", "estimated_time_in_hours", f.estimatedHours)
	setIfChanged(cmd, fields, "comment", "comment", f.comment)
	setIfChanged(cmd, fields, "pr-milestone-id", "pr_milestone_id", f.prMilestoneID)
	return fields
}

func newProjectPackageListCmd() *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:     "list <project-id>",
		Short:   "List the work packages of a project",
		Example: `  bexio project package list 3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			pkgs, err := client.ListProjectPackages(cmd.Context(), projectID, limit, offset)
			if err != nil {
				return err
			}
			return renderProjectPackages(cmd, pkgs)
		},
	}
	paging30Flags(cmd, &limit, &offset)
	return cmd
}

func newProjectPackageViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <project-id> <package-id>",
		Short: "Show a work package",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("work package", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetProjectPackage(cmd.Context(), projectID, id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, p.Raw, projectPackageDetailOrder)
		},
	}
}

func newProjectPackageCreateCmd() *cobra.Command {
	var fields projectPackageFieldFlags
	cmd := &cobra.Command{
		Use:     "create <project-id>",
		Short:   "Create a work package",
		Example: `  bexio project package create 3 --name Documentation --estimated-time-in-hours 8`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["name"] == nil {
				return fmt.Errorf("--name is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.CreateProjectPackage(cmd.Context(), projectID, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created work package %d (%s) for project %d\n", p.ID, p.Name, projectID)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newProjectPackageUpdateCmd() *cobra.Command {
	var fields projectPackageFieldFlags
	cmd := &cobra.Command{
		Use:   "update <project-id> <package-id>",
		Short: "Update a work package",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("work package", args[1])
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
			p, err := client.UpdateProjectPackage(cmd.Context(), projectID, id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), p.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated work package %d (%s)\n", p.ID, p.Name)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newProjectPackageDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project-id> <package-id>",
		Short: "Delete a work package",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := parseID("project", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("work package", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteProjectPackage(cmd.Context(), projectID, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted work package %d\n", id)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// timesheet (API resource "timesheet")
// ---------------------------------------------------------------------------

func newTimesheetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timesheet",
		Short: "List, view, search, and modify timesheets (time tracking)",
	}
	cmd.AddCommand(
		newTimesheetListCmd(),
		newTimesheetViewCmd(),
		newTimesheetSearchCmd(),
		newTimesheetCreateCmd(),
		newTimesheetUpdateCmd(),
		newTimesheetDeleteCmd(),
		newTimesheetStatusCmd(),
	)
	return cmd
}

var timesheetDetailOrder = []string{
	"id", "user_id", "status_id", "client_service_id", "text",
	"allowable_bill", "charge", "contact_id", "sub_contact_id",
	"pr_project_id", "pr_package_id", "pr_milestone_id", "estimated_time",
	"date", "duration", "running", "tracking", "travel_time",
	"travel_charge", "travel_distance",
}

func renderTimesheets(cmd *cobra.Command, sheets []api.Timesheet) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(sheets))
		for i, t := range sheets {
			raws[i] = t.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(sheets))
	for i, t := range sheets {
		rows[i] = []string{
			strconv.Itoa(t.ID),
			t.Date,
			t.Duration,
			strconv.Itoa(t.UserID),
			strconv.Itoa(t.ClientServiceID),
			strconv.Itoa(t.PrProjectID),
			strconv.FormatBool(t.AllowableBill),
			output.Truncate(t.Text, 40),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "date", "duration", "user", "service", "project", "billable", "text"}, rows)
	return nil
}

func newTimesheetListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List timesheets",
		Example: `  bexio timesheet list
  bexio timesheet list --order-by date_desc -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			sheets, err := client.ListTimesheets(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderTimesheets(cmd, sheets)
		},
	}
	listFlags(cmd, &opts, "id, date, duration, user_id")
	return cmd
}

func newTimesheetViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single timesheet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("timesheet", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			t, err := client.GetTimesheet(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, t.Raw, timesheetDetailOrder)
		},
	}
}

func newTimesheetSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search timesheets",
		Long: `Search timesheets with --where clauses on the raw API field names (see
"bexio contact search --help" for operators). Searchable fields include:
id, user_id, status_id, client_service_id, contact_id, pr_project_id, date.`,
		Example: `  bexio timesheet search --where pr_project_id=3
  bexio timesheet search --where user_id=1 --where "date>=2026-08-01"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			criteria, err := parseWhere(where)
			if err != nil {
				return err
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: pass at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			sheets, err := client.SearchTimesheets(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderTimesheets(cmd, sheets)
		},
	}
	listFlags(cmd, &opts, "id, date, duration, user_id")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

// timesheetFieldFlags mirrors the API payload fields of POST /2.0/timesheet.
// The nested tracking object is exposed as --tracking-type plus --date,
// --duration (type duration) and --start, --end (type range).
type timesheetFieldFlags struct {
	userID, statusID, clientServiceID int
	text                              string
	allowableBill                     bool
	contactID, subContactID           int
	prProjectID                       int
	prPackageID, prMilestoneID        int
	estimatedTime                     string

	trackingType   string
	date, duration string
	start, end     string
}

func (f *timesheetFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user on create)")
	fl.IntVar(&f.statusID, "status-id", 0, "timesheet status id (list with: bexio timesheet status list)")
	fl.IntVar(&f.clientServiceID, "client-service-id", 0, "business activity id (list with: bexio client-service list)")
	fl.StringVar(&f.text, "text", "", "description")
	fl.BoolVar(&f.allowableBill, "allowable-bill", false, "the tracked time is billable")
	fl.IntVar(&f.contactID, "contact-id", 0, "contact id")
	fl.IntVar(&f.subContactID, "sub-contact-id", 0, "sub contact id")
	fl.IntVar(&f.prProjectID, "pr-project-id", 0, "project id")
	fl.IntVar(&f.prPackageID, "pr-package-id", 0, "work package id")
	fl.IntVar(&f.prMilestoneID, "pr-milestone-id", 0, "milestone id")
	fl.StringVar(&f.estimatedTime, "estimated-time", "", `estimated time (e.g. "02:30")`)
	fl.StringVar(&f.trackingType, "tracking-type", "", `tracking type: "duration" (with --date and --duration) or "range" (with --start and --end)`)
	fl.StringVar(&f.date, "date", "", "tracking date (YYYY-MM-DD; tracking type duration)")
	fl.StringVar(&f.duration, "duration", "", `tracked duration (e.g. "01:40"; tracking type duration)`)
	fl.StringVar(&f.start, "start", "", `tracking start (e.g. "2026-08-20 14:00:00"; tracking type range)`)
	fl.StringVar(&f.end, "end", "", `tracking end (e.g. "2026-08-20 16:30:00"; tracking type range)`)
}

// payload collects the flat fields plus, if --tracking-type was given, the
// nested tracking object.
func (f *timesheetFieldFlags) payload(cmd *cobra.Command) (map[string]any, error) {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "status-id", "status_id", f.statusID)
	setIfChanged(cmd, fields, "client-service-id", "client_service_id", f.clientServiceID)
	setIfChanged(cmd, fields, "text", "text", f.text)
	setIfChanged(cmd, fields, "allowable-bill", "allowable_bill", f.allowableBill)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "sub-contact-id", "sub_contact_id", f.subContactID)
	setIfChanged(cmd, fields, "pr-project-id", "pr_project_id", f.prProjectID)
	setIfChanged(cmd, fields, "pr-package-id", "pr_package_id", f.prPackageID)
	setIfChanged(cmd, fields, "pr-milestone-id", "pr_milestone_id", f.prMilestoneID)
	setIfChanged(cmd, fields, "estimated-time", "estimated_time", f.estimatedTime)

	switch f.trackingType {
	case "":
		if f.date != "" || f.duration != "" || f.start != "" || f.end != "" {
			return nil, fmt.Errorf("--date/--duration/--start/--end need --tracking-type")
		}
	case "duration":
		if f.date == "" || f.duration == "" {
			return nil, fmt.Errorf("--tracking-type duration needs --date and --duration")
		}
		fields["tracking"] = map[string]any{"type": "duration", "date": f.date, "duration": f.duration}
	case "range":
		if f.start == "" || f.end == "" {
			return nil, fmt.Errorf("--tracking-type range needs --start and --end")
		}
		fields["tracking"] = map[string]any{"type": "range", "start": f.start, "end": f.end}
	default:
		return nil, fmt.Errorf(`--tracking-type must be "duration" or "range"`)
	}
	return fields, nil
}

func newTimesheetCreateCmd() *cobra.Command {
	var fields timesheetFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a timesheet",
		Long: `Create a timesheet. --client-service-id and --tracking-type are
required; user_id defaults to the authenticated user. The tracked time is
either a duration on a date (--tracking-type duration --date --duration)
or a start/end range (--tracking-type range --start --end). allowable_bill
is required by the API and defaults to false (pass --allowable-bill for
billable time).`,
		Example: `  bexio timesheet create --client-service-id 1 --pr-project-id 3 \
      --tracking-type duration --date 2026-08-20 --duration 01:30 \
      --allowable-bill --text "code review"
  bexio timesheet create --client-service-id 1 --tracking-type range \
      --start "2026-08-20 14:00:00" --end "2026-08-20 16:30:00"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if payload["client_service_id"] == nil {
				return fmt.Errorf("--client-service-id is required")
			}
			if payload["tracking"] == nil {
				return fmt.Errorf("--tracking-type is required")
			}
			// allowable_bill is required by the API; always send it.
			payload["allowable_bill"] = fields.allowableBill
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
			t, err := client.CreateTimesheet(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), t.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created timesheet %d (%s, %s)\n", t.ID, t.Date, t.Duration)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newTimesheetUpdateCmd() *cobra.Command {
	var fields timesheetFieldFlags
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update fields of a timesheet",
		Long:    "Update a timesheet. Only the flags you pass are changed.",
		Example: `  bexio timesheet update 42 --text "pairing session" --allowable-bill=false`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("timesheet", args[0])
			if err != nil {
				return err
			}
			payload, err := fields.payload(cmd)
			if err != nil {
				return err
			}
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			t, err := client.UpdateTimesheet(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), t.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated timesheet %d\n", t.ID)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newTimesheetDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a timesheet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("timesheet", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteTimesheet(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted timesheet %d\n", id)
			return nil
		},
	}
}

func newTimesheetStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Timesheet statuses (API resource timesheet_status, read-only)",
	}
	cmd.AddCommand(projectLookupListCmd("List timesheet statuses", func(cmd *cobra.Command) ([]api.ProjectLookupItem, error) {
		client, err := newClient()
		if err != nil {
			return nil, err
		}
		return client.ListTimesheetStatuses(cmd.Context())
	}))
	return cmd
}

// ---------------------------------------------------------------------------
// client-service (API resource "client_service", business activities)
// ---------------------------------------------------------------------------

func newBusinessActivityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client-service",
		Aliases: []string{"business-activity"},
		Short:   "List, search, and create business activities (client services)",
	}
	cmd.AddCommand(
		newBusinessActivityListCmd(),
		newBusinessActivitySearchCmd(),
		newBusinessActivityCreateCmd(),
	)
	return cmd
}

func renderBusinessActivities(cmd *cobra.Command, activities []api.BusinessActivity) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(activities))
		for i, a := range activities {
			raws[i] = a.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(activities))
	for i, a := range activities {
		rows[i] = []string{
			strconv.Itoa(a.ID),
			output.Truncate(a.Name, 40),
			strconv.FormatBool(a.DefaultIsBillable),
			a.DefaultPricePerHour,
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name", "billable", "price_per_hour"}, rows)
	return nil
}

func newBusinessActivityListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List business activities",
		Example: `  bexio client-service list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			activities, err := client.ListBusinessActivities(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderBusinessActivities(cmd, activities)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newBusinessActivitySearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:     "search [term]",
		Short:   "Search business activities",
		Example: `  bexio client-service search Consulting`,
		Args:    cobra.MaximumNArgs(1),
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
					Field: "name", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			activities, err := client.SearchBusinessActivities(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderBusinessActivities(cmd, activities)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}

func newBusinessActivityCreateCmd() *cobra.Command {
	var name string
	var defaultIsBillable bool
	var defaultPricePerHour string
	var accountID int
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a business activity",
		Example: `  bexio client-service create --name Consulting --default-is-billable --default-price-per-hour 150`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			payload := map[string]any{"name": name}
			setIfChanged(cmd, payload, "default-is-billable", "default_is_billable", defaultIsBillable)
			setIfChanged(cmd, payload, "default-price-per-hour", "default_price_per_hour", defaultPricePerHour)
			setIfChanged(cmd, payload, "account-id", "account_id", accountID)
			client, err := newClient()
			if err != nil {
				return err
			}
			a, err := client.CreateBusinessActivity(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created business activity %d (%s)\n", a.ID, a.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "business activity name (required)")
	cmd.Flags().BoolVar(&defaultIsBillable, "default-is-billable", false, "billable by default")
	cmd.Flags().StringVar(&defaultPricePerHour, "default-price-per-hour", "", `default price per hour (decimal string, e.g. "150.00")`)
	cmd.Flags().IntVar(&accountID, "account-id", 0, "account id")
	return cmd
}
