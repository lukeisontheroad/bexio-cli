package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func newContactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contact",
		Short: "List, view, search, and modify contacts",
	}
	cmd.AddCommand(
		newContactListCmd(),
		newContactViewCmd(),
		newContactSearchCmd(),
		newContactCreateCmd(),
		newContactUpdateCmd(),
		newContactDeleteCmd(),
		newContactRestoreCmd(),
		newAddressCmd(),
	)
	return cmd
}

func renderContacts(cmd *cobra.Command, contacts []api.Contact) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(contacts))
		for i, c := range contacts {
			raws[i] = c.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(contacts))
	for i, c := range contacts {
		rows[i] = []string{
			strconv.Itoa(c.ID),
			c.Nr,
			c.TypeName(),
			output.Truncate(c.Name(), 40),
			c.Mail,
			c.City,
			shortDate(c.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "nr", "type", "name", "mail", "city", "updated"}, rows)
	return nil
}

// contactDetailOrder is the field order for the table view of a single
// contact; remaining raw fields follow alphabetically.
var contactDetailOrder = []string{
	"id", "nr", "contact_type_id", "name_1", "name_2", "salutation_id",
	"title_id", "birthday", "street_name", "house_number", "address_addition",
	"postcode", "city", "country_id", "mail", "mail_second", "phone_fixed",
	"phone_fixed_second", "phone_mobile", "fax", "url", "skype_name",
	"remarks", "language_id", "is_lead", "contact_group_ids",
	"contact_branch_ids", "user_id", "owner_id", "updated_at",
}

// renderDetail prints a raw API object as "field  value" rows: known fields
// first in a stable order, any extras alphabetically. Null values are
// skipped.
func renderDetail(cmd *cobra.Command, raw json.RawMessage, order []string) error {
	if flagOutput == "json" {
		return output.JSON(cmd.OutOrStdout(), raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	var rows [][]string
	add := func(k string) {
		v, ok := m[k]
		if !ok || v == nil {
			return
		}
		delete(m, k)
		s := ""
		switch t := v.(type) {
		case string:
			s = t
		case float64:
			s = strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			s = strconv.FormatBool(t)
		default:
			b, _ := json.Marshal(t)
			s = string(b)
		}
		if s == "" {
			return
		}
		rows = append(rows, []string{k, s})
	}
	for _, k := range order {
		add(k)
	}
	rest := make([]string, 0, len(m))
	for k := range m {
		rest = append(rest, k)
	}
	sort.Strings(rest)
	for _, k := range rest {
		add(k)
	}
	output.Table(cmd.OutOrStdout(), []string{"field", "value"}, rows)
	return nil
}

func newContactListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts",
		Example: `  bexio contact list
  bexio contact list --limit 500 --order-by name_1
  bexio contact list --archived
  bexio contact list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			contacts, err := client.ListContacts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderContacts(cmd, contacts)
		},
	}
	listFlags(cmd, &opts, "id, nr, name_1, updated_at")
	cmd.Flags().BoolVar(&opts.ShowArchived, "archived", false, "show archived (deleted) contacts only")
	return cmd
}

func newContactViewCmd() *cobra.Command {
	var archived bool
	cmd := &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single contact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.GetContact(cmd.Context(), id, archived)
			if err != nil {
				return err
			}
			return renderDetail(cmd, c.Raw, contactDetailOrder)
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "look up an archived (deleted) contact")
	return cmd
}

func newContactSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search contacts",
		Long: `Search contacts. A bare term matches name_1 partially. --where clauses
use the raw API field names and add AND conditions:

  field=value    exact          field~value    partial (like)
  field!=value   not equal      field!~value   not like
  field>value    greater        field>=value   greater or equal
  field<value    less           field<=value   less or equal

Searchable fields include: id, nr, name_1, name_2, mail, mail_second,
postcode, city, country_id, contact_group_ids, contact_type_id, updated_at,
user_id, phone_fixed, phone_mobile, fax.`,
		Example: `  bexio contact search Meyer
  bexio contact search --where mail~@acme.ch
  bexio contact search --where contact_type_id=1 --where city=Zürich
  bexio contact search --where "updated_at>2026-01-01" -o json`,
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
					Field: "name_1", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			contacts, err := client.SearchContacts(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderContacts(cmd, contacts)
		},
	}
	listFlags(cmd, &opts, "id, nr, name_1, updated_at")
	cmd.Flags().BoolVar(&opts.ShowArchived, "archived", false, "search archived (deleted) contacts only")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

// contactFieldFlags mirrors the API payload fields of POST /2.0/contact.
type contactFieldFlags struct {
	nr, name1, name2                          string
	salutationID, titleID                     int
	birthday                                  string
	streetName, houseNumber, addressAddition  string
	postcode, city                            string
	countryID                                 int
	mail, mailSecond                          string
	phoneFixed, phoneFixedSecond, phoneMobile string
	fax, url, skypeName, remarks              string
	languageID                                int
	isLead                                    bool
	contactGroupIDs, contactBranchIDs         string
	userID, ownerID                           int
}

// register adds one flag per API field, named exactly after the field
// (underscores as hyphens), so the CLI follows the docs.bexio.com scheme.
func (f *contactFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.nr, "nr", "", "contact number (auto-assigned if omitted)")
	fl.StringVar(&f.name1, "name-1", "", "company name / person last name")
	fl.StringVar(&f.name2, "name-2", "", "company name addition / person first name")
	fl.IntVar(&f.salutationID, "salutation-id", 0, "salutation id")
	fl.IntVar(&f.titleID, "title-id", 0, "title id")
	fl.StringVar(&f.birthday, "birthday", "", "birthday (YYYY-MM-DD)")
	fl.StringVar(&f.streetName, "street-name", "", "street name")
	fl.StringVar(&f.houseNumber, "house-number", "", "house number (requires --street-name)")
	fl.StringVar(&f.addressAddition, "address-addition", "", "address addition (requires --street-name)")
	fl.StringVar(&f.postcode, "postcode", "", "postcode")
	fl.StringVar(&f.city, "city", "", "city")
	fl.IntVar(&f.countryID, "country-id", 0, "country id (list with: bexio api GET /2.0/country)")
	fl.StringVar(&f.mail, "mail", "", "e-mail address")
	fl.StringVar(&f.mailSecond, "mail-second", "", "second e-mail address")
	fl.StringVar(&f.phoneFixed, "phone-fixed", "", "phone (fixed)")
	fl.StringVar(&f.phoneFixedSecond, "phone-fixed-second", "", "second phone (fixed)")
	fl.StringVar(&f.phoneMobile, "phone-mobile", "", "phone (mobile)")
	fl.StringVar(&f.fax, "fax", "", "fax number")
	fl.StringVar(&f.url, "url", "", "website URL")
	fl.StringVar(&f.skypeName, "skype-name", "", "skype name")
	fl.StringVar(&f.remarks, "remarks", "", "remarks")
	fl.IntVar(&f.languageID, "language-id", 0, "language id (list with: bexio api GET /2.0/language)")
	fl.BoolVar(&f.isLead, "is-lead", false, "mark the contact as a lead")
	fl.StringVar(&f.contactGroupIDs, "contact-group-ids", "", `contact group ids, comma-separated (e.g. "2,5")`)
	fl.StringVar(&f.contactBranchIDs, "contact-branch-ids", "", `contact sector ids, comma-separated`)
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user)")
	fl.IntVar(&f.ownerID, "owner-id", 0, "owner id (defaults to the authenticated user)")
}

// payload collects the fields the user actually set.
func (f *contactFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "nr", "nr", f.nr)
	setIfChanged(cmd, fields, "name-1", "name_1", f.name1)
	setIfChanged(cmd, fields, "name-2", "name_2", f.name2)
	setIfChanged(cmd, fields, "salutation-id", "salutation_id", f.salutationID)
	setIfChanged(cmd, fields, "title-id", "title_id", f.titleID)
	setIfChanged(cmd, fields, "birthday", "birthday", f.birthday)
	setIfChanged(cmd, fields, "street-name", "street_name", f.streetName)
	setIfChanged(cmd, fields, "house-number", "house_number", f.houseNumber)
	setIfChanged(cmd, fields, "address-addition", "address_addition", f.addressAddition)
	setIfChanged(cmd, fields, "postcode", "postcode", f.postcode)
	setIfChanged(cmd, fields, "city", "city", f.city)
	setIfChanged(cmd, fields, "country-id", "country_id", f.countryID)
	setIfChanged(cmd, fields, "mail", "mail", f.mail)
	setIfChanged(cmd, fields, "mail-second", "mail_second", f.mailSecond)
	setIfChanged(cmd, fields, "phone-fixed", "phone_fixed", f.phoneFixed)
	setIfChanged(cmd, fields, "phone-fixed-second", "phone_fixed_second", f.phoneFixedSecond)
	setIfChanged(cmd, fields, "phone-mobile", "phone_mobile", f.phoneMobile)
	setIfChanged(cmd, fields, "fax", "fax", f.fax)
	setIfChanged(cmd, fields, "url", "url", f.url)
	setIfChanged(cmd, fields, "skype-name", "skype_name", f.skypeName)
	setIfChanged(cmd, fields, "remarks", "remarks", f.remarks)
	setIfChanged(cmd, fields, "language-id", "language_id", f.languageID)
	setIfChanged(cmd, fields, "is-lead", "is_lead", f.isLead)
	setIfChanged(cmd, fields, "contact-group-ids", "contact_group_ids", f.contactGroupIDs)
	setIfChanged(cmd, fields, "contact-branch-ids", "contact_branch_ids", f.contactBranchIDs)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	setIfChanged(cmd, fields, "owner-id", "owner_id", f.ownerID)
	return fields
}

func newContactCreateCmd() *cobra.Command {
	var fields contactFieldFlags
	var contactType string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a contact",
		Long: `Create a contact. --type and --name-1 are required. For persons, name_1
is the last name and name_2 the first name. user_id and owner_id are
required by the API and default to the authenticated user.`,
		Example: `  bexio contact create --type company --name-1 "ACME AG" --mail info@acme.ch
  bexio contact create --type person --name-1 Meyer --name-2 Anna \
      --mail anna@example.com --postcode 8000 --city Zürich --country-id 1`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			switch contactType {
			case "company":
				payload["contact_type_id"] = 1
			case "person":
				payload["contact_type_id"] = 2
			default:
				return fmt.Errorf(`--type must be "company" or "person"`)
			}
			if payload["name_1"] == nil {
				return fmt.Errorf("--name-1 is required")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if payload["user_id"] == nil || payload["owner_id"] == nil {
				me, err := client.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolve default user_id/owner_id: %w", err)
				}
				if payload["user_id"] == nil {
					payload["user_id"] = me.ID
				}
				if payload["owner_id"] == nil {
					payload["owner_id"] = me.ID
				}
			}
			c, err := client.CreateContact(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), c.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created contact %d (%s)\n", c.ID, c.Name())
			return nil
		},
	}
	cmd.Flags().StringVar(&contactType, "type", "", `contact type: "company" or "person" (required)`)
	fields.register(cmd)
	return cmd
}

func newContactUpdateCmd() *cobra.Command {
	var fields contactFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update fields of a contact",
		Long:  "Update a contact. Only the flags you pass are changed.",
		Example: `  bexio contact update 17 --mail new@acme.ch
  bexio contact update 17 --street-name "Neue Strasse" --house-number 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact", args[0])
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
			c, err := client.UpdateContact(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), c.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated contact %d (%s)\n", c.ID, c.Name())
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newContactDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete (archive) a contact",
		Long:  "Delete a contact. Deleted contacts can be brought back with `bexio contact restore`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteContact(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted contact %d (restore with `bexio contact restore %d`)\n", id, id)
			return nil
		},
	}
}

func newContactRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a deleted contact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.RestoreContact(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored contact %d\n", id)
			return nil
		},
	}
}
