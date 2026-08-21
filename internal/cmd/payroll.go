package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// This file implements the payroll commands (employees, absences, paystub
// PDFs) plus the leftover resources: communication types, document
// settings, document templates, and bulk contact creation.

func init() {
	registerModule(newEmployeeCmd)
	registerModule(newCommunicationKindCmd)
	registerModule(newDocumentSettingCmd)
	registerModule(newDocumentTemplateCmd)
	registerModule(newContactBulkCreateCmd)
}

// ---------------------------------------------------------------------------
// payroll-employee (4.0 API /4.0/payroll/employees)
// ---------------------------------------------------------------------------

func newEmployeeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "payroll-employee",
		Aliases: []string{"employee"},
		Short:   "List, view, and modify payroll employees (4.0 API)",
		Long: `Manage payroll employees. Unlike the other bexio resources, payroll ids
are UUID strings, the update endpoint is a PATCH, and "view" returns the
state of the employee on a given date.`,
	}
	cmd.AddCommand(
		newEmployeeListCmd(),
		newEmployeeViewCmd(),
		newEmployeeCreateCmd(),
		newEmployeeUpdateCmd(),
		newEmployeeAbsenceCmd(),
		newPaystubCmd(),
	)
	return cmd
}

// parseEmployeeUUID validates a payroll id argument. Payroll ids are UUID
// strings, so unlike parseID there is no number to parse.
func parseEmployeeUUID(kind, arg string) (string, error) {
	id := strings.TrimSpace(arg)
	if id == "" {
		return "", fmt.Errorf("invalid %s id %q (expected a UUID)", kind, arg)
	}
	return id, nil
}

var employeeDetailOrder = []string{
	"id", "personal_number", "first_name", "last_name", "date_of_birth",
	"gender", "ahv_number", "nationality", "stay_permit_category", "language",
	"marital_status", "email", "phone_number", "iban", "hours_per_week",
	"effective_working_hours_per_week", "employment_level",
	"annual_vacation_days_total", "annual_vacation_days_used",
	"annual_vacation_days_left", "address",
}

func renderEmployees(cmd *cobra.Command, employees []api.Employee) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(employees))
		for i, e := range employees {
			raws[i] = e.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(employees))
	for i, e := range employees {
		rows[i] = []string{
			e.ID,
			e.PersonalNumber,
			output.Truncate(e.Name(), 30),
			e.Email,
			e.EmploymentLevel.String(),
			strconv.Itoa(e.AnnualVacationDays),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "personal_nr", "name", "email", "employment_level", "vacation_days"}, rows)
	return nil
}

func newEmployeeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active employees",
		Long: `List all active employees. The endpoint takes no paging parameters and
always returns every active employee.`,
		Example: `  bexio payroll-employee list
  bexio payroll-employee list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			employees, err := client.ListEmployees(cmd.Context())
			if err != nil {
				return err
			}
			return renderEmployees(cmd, employees)
		},
	}
}

func newEmployeeViewCmd() *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:   "view <employee-id>",
		Short: "Show the state of an employee on a date",
		Long: `Show a single employee. The API returns the employee's state on a
specific date (salary and employment data change over time); --date
defaults to today.`,
		Example: `  bexio payroll-employee view 309bf968-ea25-4819-8f2e-ca08aa369690
  bexio payroll-employee view 309bf968-ea25-4819-8f2e-ca08aa369690 --date 2026-01-31`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			e, err := client.GetEmployee(cmd.Context(), id, date)
			if err != nil {
				return err
			}
			return renderDetail(cmd, e.Raw, employeeDetailOrder)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "date of the employee's state (YYYY-MM-DD, default today)")
	return cmd
}

// employeeFieldFlags mirrors the API payload fields of POST/PATCH
// /4.0/payroll/employees. The address is a nested object, so its fields are
// exposed with an --address- prefix.
type employeeFieldFlags struct {
	email, firstName, lastName string
	personalNumber, nationality,
	iban, ahvNumber string
	maritalStatus, gender, dateOfBirth string
	language, phoneNumber              string
	annualVacationDays                 int

	addrComplementaryLine, addrStreetName, addrHouseNumber string
	addrPostbox, addrLocality, addrZipCode, addrCity       string
	addrCountry, addrCanton, addrMunicipalityID            string
}

func (f *employeeFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.firstName, "first-name", "", "first name")
	fl.StringVar(&f.lastName, "last-name", "", "last name")
	fl.StringVar(&f.email, "email", "", "email address")
	fl.StringVar(&f.personalNumber, "personal-number", "", "personal number")
	fl.StringVar(&f.nationality, "nationality", "", `nationality, ISO alpha-2 (e.g. "CH"; "11" unknown, "22" stateless)`)
	fl.StringVar(&f.iban, "iban", "", "IBAN of the salary account")
	fl.StringVar(&f.ahvNumber, "ahv-number", "", "AHV (social security) number, required on create")
	fl.StringVar(&f.maritalStatus, "marital-status", "", "marital status: unknown, single, married, separated, registered_partnership, partnership_dissolved_by_law, partnership_dissolved_by_death, partnership_dissolved_by_declaration_of_lost, widowed, divorced")
	fl.StringVar(&f.gender, "gender", "", "gender: male or female")
	fl.StringVar(&f.dateOfBirth, "date-of-birth", "", "date of birth (YYYY-MM-DD)")
	fl.StringVar(&f.language, "language", "", "correspondence language: de, it, fr, or en")
	fl.StringVar(&f.phoneNumber, "phone-number", "", "phone number")
	fl.IntVar(&f.annualVacationDays, "annual-vacation-days", 0, "annual vacation days")
	fl.StringVar(&f.addrStreetName, "address-street-name", "", "street name (the API's `street` field is deprecated)")
	fl.StringVar(&f.addrHouseNumber, "address-house-number", "", "house number (requires --address-street-name)")
	fl.StringVar(&f.addrComplementaryLine, "address-complementary-line", "", "address addition")
	fl.StringVar(&f.addrPostbox, "address-postbox", "", "post box")
	fl.StringVar(&f.addrLocality, "address-locality", "", "locality")
	fl.StringVar(&f.addrZipCode, "address-zip-code", "", "zip code")
	fl.StringVar(&f.addrCity, "address-city", "", "city")
	fl.StringVar(&f.addrCountry, "address-country", "", `country, ISO alpha-2 (e.g. "CH")`)
	fl.StringVar(&f.addrCanton, "address-canton", "", "canton")
	fl.StringVar(&f.addrMunicipalityID, "address-municipality-id", "", "municipality id")
}

// payload builds the request body. The address is only sent when at least
// one --address- flag was given; the API replaces the whole nested object,
// so pass every address field you want to keep.
func (f *employeeFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "first-name", "first_name", f.firstName)
	setIfChanged(cmd, fields, "last-name", "last_name", f.lastName)
	setIfChanged(cmd, fields, "email", "email", f.email)
	setIfChanged(cmd, fields, "personal-number", "personal_number", f.personalNumber)
	setIfChanged(cmd, fields, "nationality", "nationality", f.nationality)
	setIfChanged(cmd, fields, "iban", "iban", f.iban)
	setIfChanged(cmd, fields, "ahv-number", "ahv_number", f.ahvNumber)
	setIfChanged(cmd, fields, "marital-status", "marital_status", f.maritalStatus)
	setIfChanged(cmd, fields, "gender", "gender", f.gender)
	setIfChanged(cmd, fields, "date-of-birth", "date_of_birth", f.dateOfBirth)
	setIfChanged(cmd, fields, "language", "language", f.language)
	setIfChanged(cmd, fields, "phone-number", "phone_number", f.phoneNumber)
	setIfChanged(cmd, fields, "annual-vacation-days", "annual_vacation_days", f.annualVacationDays)

	address := map[string]any{}
	setIfChanged(cmd, address, "address-street-name", "street_name", f.addrStreetName)
	setIfChanged(cmd, address, "address-house-number", "house_number", f.addrHouseNumber)
	setIfChanged(cmd, address, "address-complementary-line", "complementary_line", f.addrComplementaryLine)
	setIfChanged(cmd, address, "address-postbox", "postbox", f.addrPostbox)
	setIfChanged(cmd, address, "address-locality", "locality", f.addrLocality)
	setIfChanged(cmd, address, "address-zip-code", "zip_code", f.addrZipCode)
	setIfChanged(cmd, address, "address-city", "city", f.addrCity)
	setIfChanged(cmd, address, "address-country", "country", f.addrCountry)
	setIfChanged(cmd, address, "address-canton", "canton", f.addrCanton)
	setIfChanged(cmd, address, "address-municipality-id", "municipality_id", f.addrMunicipalityID)
	if len(address) > 0 {
		fields["address"] = address
	}
	return fields
}

func newEmployeeCreateCmd() *cobra.Command {
	var fields employeeFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an employee",
		Long: `Create a payroll employee. --ahv-number is required by the API. The
address flags are sent as one nested address object; house_number requires
street_name (the API's flat "street" field is deprecated).`,
		Example: `  bexio payroll-employee create --ahv-number 756.1234.5678.97 \
      --first-name Anna --last-name Meyer --date-of-birth 1990-04-01 \
      --nationality CH --address-street-name Bahnhofstrasse \
      --address-house-number 1 --address-zip-code 8001 --address-city Zürich`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["ahv_number"] == nil {
				return fmt.Errorf("--ahv-number is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			e, err := client.CreateEmployee(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), e.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created employee %s (%s)\n", e.ID, e.Name())
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newEmployeeUpdateCmd() *cobra.Command {
	var fields employeeFieldFlags
	cmd := &cobra.Command{
		Use:   "update <employee-id>",
		Short: "Update fields of an employee",
		Long: `Update an employee (HTTP PATCH). Only the flags you pass are changed,
except for the address: the --address- flags are sent as one nested object
that replaces the stored address. The API answers 204 No Content, so
nothing is echoed back.`,
		Example: `  bexio payroll-employee update 309bf968-ea25-4819-8f2e-ca08aa369690 \
      --email anna.meyer@example.com --phone-number "+41 44 111 22 33"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseEmployeeUUID("employee", args[0])
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
			if err := client.UpdateEmployee(cmd.Context(), id, payload); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated employee %s\n", id)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

// ---------------------------------------------------------------------------
// payroll-employee absence (4.0 API /4.0/payroll/employees/{id}/absences)
// ---------------------------------------------------------------------------

func newEmployeeAbsenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "absence",
		Short: "Manage the absences of an employee",
		Long: `Manage employee absences. Absence ids are UUID strings and every
subcommand takes the employee id first.`,
	}
	cmd.AddCommand(
		newAbsenceListCmd(),
		newAbsenceViewCmd(),
		newAbsenceCreateCmd(),
		newAbsenceUpdateCmd(),
		newAbsenceDeleteCmd(),
	)
	return cmd
}

var absenceDetailOrder = []string{
	"id", "reason", "start_date", "end_date", "half_day", "continued_pay",
	"disability", "paid_hours",
}

func renderAbsences(cmd *cobra.Command, absences []api.Absence) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(absences))
		for i, a := range absences {
			raws[i] = a.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(absences))
	for i, a := range absences {
		rows[i] = []string{
			a.ID,
			a.Reason,
			a.StartDate,
			a.EndDate,
			strconv.FormatBool(a.HalfDay),
			a.PaidHours.String(),
			a.ContinuedPay.String(),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "reason", "start_date", "end_date", "half_day", "paid_hours", "continued_pay"}, rows)
	return nil
}

// absenceFieldFlags mirrors the API payload of the absence endpoints.
type absenceFieldFlags struct {
	reason, startDate, endDate string
	halfDay                    bool
	continuedPay               float64
	disability                 float64
	paidHours                  float64
}

func (f *absenceFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.reason, "reason", "", "absence reason: Injury, Sickness, MaternityLeave, MilitaryLeave, Vacation, or InterruptionOfWork")
	fl.StringVar(&f.startDate, "start-date", "", "first day of the absence (YYYY-MM-DD)")
	fl.StringVar(&f.endDate, "end-date", "", "last day of the absence (YYYY-MM-DD)")
	fl.BoolVar(&f.halfDay, "half-day", false, "the absence covers half a day only")
	fl.Float64Var(&f.continuedPay, "continued-pay", 0, "continued pay in percent (e.g. 80)")
	fl.Float64Var(&f.disability, "disability", 0, "degree of disability in percent (e.g. 50)")
	fl.Float64Var(&f.paidHours, "paid-hours", 0, "paid hours (e.g. 8.5)")
}

func (f *absenceFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "reason", "reason", f.reason)
	setIfChanged(cmd, fields, "start-date", "start_date", f.startDate)
	setIfChanged(cmd, fields, "end-date", "end_date", f.endDate)
	setIfChanged(cmd, fields, "half-day", "half_day", f.halfDay)
	setIfChanged(cmd, fields, "continued-pay", "continued_pay", f.continuedPay)
	setIfChanged(cmd, fields, "disability", "disability", f.disability)
	setIfChanged(cmd, fields, "paid-hours", "paid_hours", f.paidHours)
	return fields
}

func newAbsenceListCmd() *cobra.Command {
	var year int
	cmd := &cobra.Command{
		Use:   "list <employee-id>",
		Short: "List the absences of an employee for one business year",
		Long: `List absences. The API requires a business year; --year defaults to the
current one.`,
		Example: `  bexio payroll-employee absence list 309bf968-ea25-4819-8f2e-ca08aa369690
  bexio payroll-employee absence list 309bf968-ea25-4819-8f2e-ca08aa369690 --year 2025`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			if year == 0 {
				year = time.Now().Year()
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			absences, err := client.ListAbsences(cmd.Context(), employeeID, year)
			if err != nil {
				return err
			}
			return renderAbsences(cmd, absences)
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "business year of the absences (default: current year)")
	return cmd
}

func newAbsenceViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <employee-id> <absence-id>",
		Short: "Show a single absence",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			id, err := parseEmployeeUUID("absence", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			a, err := client.GetAbsence(cmd.Context(), employeeID, id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, a.Raw, absenceDetailOrder)
		},
	}
}

func newAbsenceCreateCmd() *cobra.Command {
	var fields absenceFieldFlags
	cmd := &cobra.Command{
		Use:   "create <employee-id>",
		Short: "Create an absence for an employee",
		Long:  "Create an absence. --reason and --start-date are required.",
		Example: `  bexio payroll-employee absence create 309bf968-ea25-4819-8f2e-ca08aa369690 \
      --reason Vacation --start-date 2026-07-06 --end-date 2026-07-17`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["reason"] == nil {
				return fmt.Errorf("--reason is required")
			}
			if payload["start_date"] == nil {
				return fmt.Errorf("--start-date is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			a, err := client.CreateAbsence(cmd.Context(), employeeID, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created absence %s (%s, %s)\n", a.ID, a.Reason, a.StartDate)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newAbsenceUpdateCmd() *cobra.Command {
	var fields absenceFieldFlags
	cmd := &cobra.Command{
		Use:   "update <employee-id> <absence-id>",
		Short: "Update an absence",
		Long: `Update an absence. The API endpoint is a PUT that replaces the whole
absence, so the CLI first fetches the current absence and applies your
flags on top of it — only the fields you pass change. The API answers 204
No Content, so nothing is echoed back.`,
		Example: `  bexio payroll-employee absence update 309bf968-ea25-4819-8f2e-ca08aa369690 \
      7c1f0f1a-6f0a-4f2e-9d1a-2a0d5c9a1b23 --end-date 2026-07-24`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			id, err := parseEmployeeUUID("absence", args[1])
			if err != nil {
				return err
			}
			changes := fields.payload(cmd)
			if len(changes) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			current, err := client.GetAbsence(cmd.Context(), employeeID, id)
			if err != nil {
				return fmt.Errorf("read current absence: %w", err)
			}
			payload := absenceReplacement(current)
			for k, v := range changes {
				payload[k] = v
			}
			if err := client.UpdateAbsence(cmd.Context(), employeeID, id, payload); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated absence %s\n", id)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

// absenceReplacement builds the complete body the PUT endpoint requires
// from the absence's current state.
func absenceReplacement(a *api.Absence) map[string]any {
	payload := map[string]any{
		"reason":     a.Reason,
		"start_date": a.StartDate,
		"end_date":   a.EndDate,
		"half_day":   a.HalfDay,
	}
	for field, value := range map[string]api.PayrollDecimal{
		"continued_pay": a.ContinuedPay,
		"disability":    a.Disability,
		"paid_hours":    a.PaidHours,
	} {
		if value == "" {
			continue
		}
		if f, err := strconv.ParseFloat(value.String(), 64); err == nil {
			payload[field] = f
		}
	}
	return payload
}

func newAbsenceDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <employee-id> <absence-id>",
		Short: "Delete an absence",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			id, err := parseEmployeeUUID("absence", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteAbsence(cmd.Context(), employeeID, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted absence %s\n", id)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// payroll-employee paystub (4.0 API paystub-pdf-download)
// ---------------------------------------------------------------------------

func newPaystubCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "paystub <employee-id> <year> <month>",
		Short: "Download the paystub PDF of an employee for one month",
		Long: `Download a paystub as PDF. This uses the paystub-pdf-download endpoint,
which returns the document itself; the older paystub-pdf endpoint (which
answers a download link) is deprecated and not exposed by the CLI.`,
		Example: `  bexio payroll-employee paystub 309bf968-ea25-4819-8f2e-ca08aa369690 2026 7
  bexio payroll-employee paystub 309bf968-ea25-4819-8f2e-ca08aa369690 2026 7 --out july.pdf`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			employeeID, err := parseEmployeeUUID("employee", args[0])
			if err != nil {
				return err
			}
			year, err := strconv.Atoi(args[1])
			if err != nil || year < 1900 {
				return fmt.Errorf("invalid year %q", args[1])
			}
			month, err := strconv.Atoi(args[2])
			if err != nil || month < 1 || month > 12 {
				return fmt.Errorf("invalid month %q (want 1-12)", args[2])
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			doc, err := client.DownloadPaystub(cmd.Context(), employeeID, year, month)
			if err != nil {
				return err
			}
			path := out
			if path == "" {
				path = doc.Name
			}
			if path == "" {
				path = fmt.Sprintf("paystub-%s-%04d-%02d.pdf", employeeID, year, month)
			}
			if err := os.WriteFile(path, doc.Data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d bytes)\n", path, len(doc.Data))
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: name from the API)")
	return cmd
}

// ---------------------------------------------------------------------------
// communication-kind (API resource "communication_kind", read-only)
// ---------------------------------------------------------------------------

func newCommunicationKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "communication-kind",
		Aliases: []string{"communication-type"},
		Short:   "List and search communication types (read-only)",
	}
	cmd.AddCommand(
		newCommunicationKindListCmd(),
		newCommunicationKindSearchCmd(),
	)
	return cmd
}

func renderCommunicationKinds(cmd *cobra.Command, kinds []api.CommunicationKind) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(kinds))
		for i, k := range kinds {
			raws[i] = k.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(kinds))
	for i, k := range kinds {
		rows[i] = []string{strconv.Itoa(k.ID), k.Name}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
	return nil
}

func newCommunicationKindListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List communication types",
		Example: `  bexio communication-kind list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			kinds, err := client.ListCommunicationKinds(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderCommunicationKinds(cmd, kinds)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newCommunicationKindSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search communication types",
		Long: `Search communication types. A bare term matches name partially; the only
searchable field is name.`,
		Example: `  bexio communication-kind search Phone
  bexio communication-kind search --where name=E-Mail`,
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
			kinds, err := client.SearchCommunicationKinds(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderCommunicationKinds(cmd, kinds)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on name (repeatable, ANDed); see long help")
	return cmd
}

// ---------------------------------------------------------------------------
// document-setting (API resource "kb_item_setting", read-only)
// ---------------------------------------------------------------------------

func newDocumentSettingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "document-setting",
		Aliases: []string{"kb-item-setting"},
		Short:   "Show the document settings (numbering and defaults, read-only)",
	}
	cmd.AddCommand(newDocumentSettingViewCmd())
	return cmd
}

func newDocumentSettingViewCmd() *cobra.Command {
	var orderBy string
	cmd := &cobra.Command{
		Use:     "view",
		Aliases: []string{"list"},
		Short:   "Show the document settings, one entry per document type",
		Long: `Show the document settings: the numbering scheme and the defaults
(language, currency, tax mode, payment term) of every document type
(quote, order, invoice, delivery, ...). The endpoint supports order_by
only — no limit/offset. Use -o json for all fields.`,
		Example: `  bexio document-setting view
  bexio document-setting view -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			settings, err := client.ListDocumentSettings(cmd.Context(), orderBy)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				raws := make([]json.RawMessage, len(settings))
				for i, s := range settings {
					raws[i] = s.Raw
				}
				return output.JSON(cmd.OutOrStdout(), raws)
			}
			rows := make([][]string, len(settings))
			for i, s := range settings {
				rows[i] = []string{
					strconv.Itoa(s.ID),
					s.Text,
					s.KbItemClass,
					s.EnumerationFormat,
					strconv.FormatBool(s.UseAutomaticEnumeration),
					strconv.Itoa(s.NextNr),
					strconv.Itoa(s.DefaultTimePeriodInDays),
				}
			}
			output.Table(cmd.OutOrStdout(), []string{"id", "text", "class", "format", "auto_nr", "next_nr", "period_days"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&orderBy, "order-by", "", "sort by id or text, suffix _desc to reverse")
	return cmd
}

// ---------------------------------------------------------------------------
// document-template (API resource "document_templates", 3.0, read-only)
// ---------------------------------------------------------------------------

func newDocumentTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document-template",
		Short: "List the PDF document templates (read-only)",
	}
	cmd.AddCommand(newDocumentTemplateListCmd())
	return cmd
}

func newDocumentTemplateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List document templates",
		Long: `List the PDF document templates. Templates are identified by their slug
(template_slug), not by a numeric id.`,
		Example: `  bexio document-template list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			templates, err := client.ListDocumentTemplates(cmd.Context())
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				raws := make([]json.RawMessage, len(templates))
				for i, t := range templates {
					raws[i] = t.Raw
				}
				return output.JSON(cmd.OutOrStdout(), raws)
			}
			rows := make([][]string, len(templates))
			for i, t := range templates {
				rows[i] = []string{
					t.TemplateSlug,
					output.Truncate(t.Name, 40),
					strconv.FormatBool(t.IsDefault),
					strings.Join(t.DefaultForDocumentTypes, ","),
				}
			}
			output.Table(cmd.OutOrStdout(), []string{"template_slug", "name", "is_default", "default_for"}, rows)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// contact-bulk-create (POST /2.0/contact/_bulk_create)
// ---------------------------------------------------------------------------

func newContactBulkCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "contact-bulk-create",
		Short: "Create many contacts in one request from a JSON file",
		Long: `Create several contacts in one request (POST /2.0/contact/_bulk_create).

--file points at a JSON array of contact objects using the raw API field
names — the same scheme as the flags of "bexio contact create"
(contact_type_id, name_1, name_2, street_name, house_number, postcode,
city, country_id, mail, phone_fixed, contact_group_ids, ...). Pass
--file - to read the array from stdin.

contact_type_id (1 company, 2 person) and name_1 are required per entry;
user_id and owner_id are required by the API and default to the
authenticated user when an entry omits them.`,
		Example: `  bexio contact-bulk-create --file contacts.json
  cat contacts.json | bexio contact-bulk-create --file - -o json

  contacts.json:
  [
    {"contact_type_id": 1, "name_1": "ACME AG", "mail": "info@acme.ch"},
    {"contact_type_id": 2, "name_1": "Meyer", "name_2": "Anna"}
  ]`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if file == "" {
				return fmt.Errorf("--file is required (a JSON array of contact objects, or - for stdin)")
			}
			data, err := readContactBulkFile(cmd, file)
			if err != nil {
				return err
			}
			var contacts []map[string]any
			if err := json.Unmarshal(data, &contacts); err != nil {
				return fmt.Errorf("parse %s: %w (expected a JSON array of contact objects)", file, err)
			}
			if len(contacts) == 0 {
				return fmt.Errorf("%s contains no contacts", file)
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := fillContactBulkDefaults(cmd, client, contacts); err != nil {
				return err
			}
			created, err := client.BulkCreateContacts(cmd.Context(), contacts)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return renderContacts(cmd, created)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %d contacts\n", len(created))
			return renderContacts(cmd, created)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file with an array of contact objects (- for stdin)")
	return cmd
}

func readContactBulkFile(cmd *cobra.Command, file string) ([]byte, error) {
	if file == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	return os.ReadFile(file)
}

// fillContactBulkDefaults fills the API-required user_id/owner_id of every
// entry that omits them with the authenticated user, mirroring
// `bexio contact create`.
func fillContactBulkDefaults(cmd *cobra.Command, client *api.Client, contacts []map[string]any) error {
	needsDefault := false
	for _, c := range contacts {
		if c["user_id"] == nil || c["owner_id"] == nil {
			needsDefault = true
			break
		}
	}
	if !needsDefault {
		return nil
	}
	me, err := client.Me(cmd.Context())
	if err != nil {
		return fmt.Errorf("resolve default user_id/owner_id: %w", err)
	}
	for _, c := range contacts {
		if c["user_id"] == nil {
			c["user_id"] = me.ID
		}
		if c["owner_id"] == nil {
			c["owner_id"] = me.ID
		}
	}
	return nil
}
