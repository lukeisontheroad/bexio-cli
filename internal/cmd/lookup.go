package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// This file implements the reference-data commands: country, language,
// salutation, title, unit, payment-type, currency, tax, bank-account,
// user, company-profile, and permission.

func init() {
	registerModule(newCountryCmd)
	registerModule(newLanguageCmd)
	registerModule(newSalutationCmd)
	registerModule(newTitleCmd)
	registerModule(newUnitCmd)
	registerModule(newPaymentTypeCmd)
	registerModule(newCurrencyCmd)
	registerModule(newTaxCmd)
	registerModule(newBankAccountCmd)
	registerModule(newUserCmd)
	registerModule(newCompanyProfileCmd)
	registerModule(newPermissionCmd)
}

// lookupFlags30 adds the 3.0 list parameters (limit/offset only — 3.0
// list endpoints have no order_by or show_archived).
func lookupFlags30(cmd *cobra.Command, opts *api.ListOptions) {
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "maximum number of results (API default 500, max 2000)")
	cmd.Flags().IntVar(&opts.Offset, "offset", 0, "skip this many results (pagination)")
}

// renderLookupList renders a list of lookup items as a table (via row) or
// as raw JSON (via raw), following the module-wide output convention.
func renderLookupList[T any](cmd *cobra.Command, items []T, headers []string, raw func(T) json.RawMessage, row func(T) []string) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(items))
		for i, it := range items {
			raws[i] = raw(it)
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = row(it)
	}
	output.Table(cmd.OutOrStdout(), headers, rows)
	return nil
}

// lookupItem is the common shape of the simple id+name lookup resources.
type lookupItem struct {
	ID   int
	Name string
	Raw  json.RawMessage
}

// lookupCRUD wires a simple id+name resource into the standard
// list/search/view/create/update/delete subcommands. Nil funcs omit the
// corresponding subcommand (read-only resources set only list/search).
type lookupCRUD struct {
	use    string // command name (mirrors the API resource)
	kind   string // human-readable singular for messages
	list   func(ctx context.Context, c *api.Client, opts api.ListOptions) ([]lookupItem, error)
	search func(ctx context.Context, c *api.Client, criteria []api.SearchCriterion, opts api.ListOptions) ([]lookupItem, error)
	get    func(ctx context.Context, c *api.Client, id int) (lookupItem, error)
	create func(ctx context.Context, c *api.Client, name string) (lookupItem, error)
	edit   func(ctx context.Context, c *api.Client, id int, name string) (lookupItem, error)
	remove func(ctx context.Context, c *api.Client, id int) error
}

func lookupItemOf(id int, name string, raw json.RawMessage) lookupItem {
	return lookupItem{ID: id, Name: name, Raw: raw}
}

func lookupItemsOf[T any](items []T, conv func(T) lookupItem) []lookupItem {
	out := make([]lookupItem, len(items))
	for i, it := range items {
		out[i] = conv(it)
	}
	return out
}

func renderLookupItems(cmd *cobra.Command, items []lookupItem) error {
	return renderLookupList(cmd, items, []string{"id", "name"},
		func(it lookupItem) json.RawMessage { return it.Raw },
		func(it lookupItem) []string { return []string{strconv.Itoa(it.ID), it.Name} })
}

func (r lookupCRUD) command(short string) *cobra.Command {
	cmd := &cobra.Command{Use: r.use, Short: short}
	cmd.AddCommand(r.listCmd(), r.searchCmd())
	if r.get != nil {
		cmd.AddCommand(r.viewCmd())
	}
	if r.create != nil {
		cmd.AddCommand(r.createCmd())
	}
	if r.edit != nil {
		cmd.AddCommand(r.updateCmd())
	}
	if r.remove != nil {
		cmd.AddCommand(r.deleteCmd())
	}
	return cmd
}

func (r lookupCRUD) listCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List " + r.kind + "s",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			items, err := r.list(cmd.Context(), client, opts)
			if err != nil {
				return err
			}
			return renderLookupItems(cmd, items)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func (r lookupCRUD) searchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search " + r.kind + "s",
		Args:  cobra.MaximumNArgs(1),
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
			items, err := r.search(cmd.Context(), client, criteria, opts)
			if err != nil {
				return err
			}
			return renderLookupItems(cmd, items)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}

func (r lookupCRUD) viewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a " + r.kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID(r.kind, args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			it, err := r.get(cmd.Context(), client, id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, it.Raw, []string{"id", "name"})
		},
	}
}

func (r lookupCRUD) createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a " + r.kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			it, err := r.create(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), it.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s %d (%s)\n", r.kind, it.ID, it.Name)
			return nil
		},
	}
}

func (r lookupCRUD) updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id> <name>",
		Short: "Rename a " + r.kind,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID(r.kind, args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			it, err := r.edit(cmd.Context(), client, id, args[1])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), it.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s %d (%s)\n", r.kind, it.ID, it.Name)
			return nil
		},
	}
}

func (r lookupCRUD) deleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a " + r.kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(r.kind, args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := r.remove(cmd.Context(), client, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s %d\n", r.kind, id)
			return nil
		},
	}
}

// --- salutation / title / unit (id+name CRUD) ---------------------------

func newSalutationCmd() *cobra.Command {
	return lookupCRUD{
		use:  "salutation",
		kind: "salutation",
		list: func(ctx context.Context, c *api.Client, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.ListSalutations(ctx, opts)
			return lookupItemsOf(items, func(s api.Salutation) lookupItem { return lookupItemOf(s.ID, s.Name, s.Raw) }), err
		},
		search: func(ctx context.Context, c *api.Client, criteria []api.SearchCriterion, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.SearchSalutations(ctx, criteria, opts)
			return lookupItemsOf(items, func(s api.Salutation) lookupItem { return lookupItemOf(s.ID, s.Name, s.Raw) }), err
		},
		get: func(ctx context.Context, c *api.Client, id int) (lookupItem, error) {
			s, err := c.GetSalutation(ctx, id)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(s.ID, s.Name, s.Raw), nil
		},
		create: func(ctx context.Context, c *api.Client, name string) (lookupItem, error) {
			s, err := c.CreateSalutation(ctx, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(s.ID, s.Name, s.Raw), nil
		},
		edit: func(ctx context.Context, c *api.Client, id int, name string) (lookupItem, error) {
			s, err := c.UpdateSalutation(ctx, id, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(s.ID, s.Name, s.Raw), nil
		},
		remove: func(ctx context.Context, c *api.Client, id int) error { return c.DeleteSalutation(ctx, id) },
	}.command("List, view, and modify salutations")
}

func newTitleCmd() *cobra.Command {
	return lookupCRUD{
		use:  "title",
		kind: "title",
		list: func(ctx context.Context, c *api.Client, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.ListTitles(ctx, opts)
			return lookupItemsOf(items, func(t api.Title) lookupItem { return lookupItemOf(t.ID, t.Name, t.Raw) }), err
		},
		search: func(ctx context.Context, c *api.Client, criteria []api.SearchCriterion, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.SearchTitles(ctx, criteria, opts)
			return lookupItemsOf(items, func(t api.Title) lookupItem { return lookupItemOf(t.ID, t.Name, t.Raw) }), err
		},
		get: func(ctx context.Context, c *api.Client, id int) (lookupItem, error) {
			t, err := c.GetTitle(ctx, id)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(t.ID, t.Name, t.Raw), nil
		},
		create: func(ctx context.Context, c *api.Client, name string) (lookupItem, error) {
			t, err := c.CreateTitle(ctx, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(t.ID, t.Name, t.Raw), nil
		},
		edit: func(ctx context.Context, c *api.Client, id int, name string) (lookupItem, error) {
			t, err := c.UpdateTitle(ctx, id, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(t.ID, t.Name, t.Raw), nil
		},
		remove: func(ctx context.Context, c *api.Client, id int) error { return c.DeleteTitle(ctx, id) },
	}.command("List, view, and modify titles")
}

func newUnitCmd() *cobra.Command {
	return lookupCRUD{
		use:  "unit",
		kind: "unit",
		list: func(ctx context.Context, c *api.Client, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.ListUnits(ctx, opts)
			return lookupItemsOf(items, func(u api.Unit) lookupItem { return lookupItemOf(u.ID, u.Name, u.Raw) }), err
		},
		search: func(ctx context.Context, c *api.Client, criteria []api.SearchCriterion, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.SearchUnits(ctx, criteria, opts)
			return lookupItemsOf(items, func(u api.Unit) lookupItem { return lookupItemOf(u.ID, u.Name, u.Raw) }), err
		},
		get: func(ctx context.Context, c *api.Client, id int) (lookupItem, error) {
			u, err := c.GetUnit(ctx, id)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(u.ID, u.Name, u.Raw), nil
		},
		create: func(ctx context.Context, c *api.Client, name string) (lookupItem, error) {
			u, err := c.CreateUnit(ctx, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(u.ID, u.Name, u.Raw), nil
		},
		edit: func(ctx context.Context, c *api.Client, id int, name string) (lookupItem, error) {
			u, err := c.UpdateUnit(ctx, id, name)
			if err != nil {
				return lookupItem{}, err
			}
			return lookupItemOf(u.ID, u.Name, u.Raw), nil
		},
		remove: func(ctx context.Context, c *api.Client, id int) error { return c.DeleteUnit(ctx, id) },
	}.command("List, view, and modify units")
}

// --- payment-type (read-only) -------------------------------------------

func newPaymentTypeCmd() *cobra.Command {
	return lookupCRUD{
		use:  "payment-type",
		kind: "payment type",
		list: func(ctx context.Context, c *api.Client, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.ListPaymentTypes(ctx, opts)
			return lookupItemsOf(items, func(p api.PaymentType) lookupItem { return lookupItemOf(p.ID, p.Name, p.Raw) }), err
		},
		search: func(ctx context.Context, c *api.Client, criteria []api.SearchCriterion, opts api.ListOptions) ([]lookupItem, error) {
			items, err := c.SearchPaymentTypes(ctx, criteria, opts)
			return lookupItemsOf(items, func(p api.PaymentType) lookupItem { return lookupItemOf(p.ID, p.Name, p.Raw) }), err
		},
	}.command("List payment types (read-only)")
}

// --- country ------------------------------------------------------------

func newCountryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "country",
		Short: "List, view, and modify countries",
	}
	cmd.AddCommand(
		newCountryListCmd(),
		newCountrySearchCmd(),
		newCountryViewCmd(),
		newCountryCreateCmd(),
		newCountryUpdateCmd(),
		newCountryDeleteCmd(),
	)
	return cmd
}

func renderCountries(cmd *cobra.Command, countries []api.Country) error {
	return renderLookupList(cmd, countries, []string{"id", "name", "name_short", "iso3166_alpha2"},
		func(c api.Country) json.RawMessage { return c.Raw },
		func(c api.Country) []string {
			return []string{strconv.Itoa(c.ID), c.Name, c.NameShort, c.Iso3166Alpha2}
		})
}

func newCountryListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List countries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			countries, err := client.ListCountries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderCountries(cmd, countries)
		},
	}
	listFlags(cmd, &opts, "id, name, name_short, iso3166_alpha2")
	return cmd
}

func newCountrySearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:     "search [term]",
		Short:   "Search countries",
		Example: `  bexio country search Switzerland`,
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
			countries, err := client.SearchCountries(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderCountries(cmd, countries)
		},
	}
	listFlags(cmd, &opts, "id, name, name_short, iso3166_alpha2")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on name, name_short, or iso3166_alpha2 (repeatable, ANDed)")
	return cmd
}

func newCountryViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a country",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("country", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.GetCountry(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, c.Raw, []string{"id", "name", "name_short", "iso3166_alpha2"})
		},
	}
}

func newCountryCreateCmd() *cobra.Command {
	var nameShort, iso string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a country",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			fields := map[string]any{"name": args[0]}
			setIfChanged(cmd, fields, "name-short", "name_short", nameShort)
			setIfChanged(cmd, fields, "iso3166-alpha2", "iso3166_alpha2", iso)
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.CreateCountry(cmd.Context(), fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), c.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created country %d (%s)\n", c.ID, c.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&nameShort, "name-short", "", "short name of the country")
	cmd.Flags().StringVar(&iso, "iso3166-alpha2", "", "ISO 3166 alpha-2 code (e.g. CH)")
	return cmd
}

func newCountryUpdateCmd() *cobra.Command {
	var name, nameShort, iso string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a country (only given flags are sent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("country", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{}
			setIfChanged(cmd, fields, "name", "name", name)
			setIfChanged(cmd, fields, "name-short", "name_short", nameShort)
			setIfChanged(cmd, fields, "iso3166-alpha2", "iso3166_alpha2", iso)
			if len(fields) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.UpdateCountry(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), c.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated country %d (%s)\n", c.ID, c.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "name of the country")
	cmd.Flags().StringVar(&nameShort, "name-short", "", "short name of the country")
	cmd.Flags().StringVar(&iso, "iso3166-alpha2", "", "ISO 3166 alpha-2 code (e.g. CH)")
	return cmd
}

func newCountryDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a country",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("country", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteCountry(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted country %d\n", id)
			return nil
		},
	}
}

// --- language (read-only) -----------------------------------------------

func newLanguageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "language",
		Short: "List languages (read-only)",
	}
	cmd.AddCommand(newLanguageListCmd(), newLanguageSearchCmd())
	return cmd
}

func renderLanguages(cmd *cobra.Command, languages []api.Language) error {
	return renderLookupList(cmd, languages, []string{"id", "name", "iso_639_1"},
		func(l api.Language) json.RawMessage { return l.Raw },
		func(l api.Language) []string { return []string{strconv.Itoa(l.ID), l.Name, l.Iso6391} })
}

func newLanguageListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List languages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			languages, err := client.ListLanguages(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLanguages(cmd, languages)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newLanguageSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search languages",
		Args:  cobra.MaximumNArgs(1),
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
			languages, err := client.SearchLanguages(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderLanguages(cmd, languages)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on name or iso_639_1 (repeatable, ANDed)")
	return cmd
}

// --- currency (3.0) -----------------------------------------------------

func newCurrencyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "currency",
		Short: "List and view currencies",
	}
	cmd.AddCommand(newCurrencyListCmd(), newCurrencyViewCmd(), newCurrencyCodesCmd())
	return cmd
}

func newCurrencyListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List currencies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			currencies, err := client.ListCurrencies(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, currencies, []string{"id", "name", "round_factor"},
				func(c api.Currency) json.RawMessage { return c.Raw },
				func(c api.Currency) []string {
					return []string{strconv.Itoa(c.ID), c.Name, strconv.FormatFloat(c.RoundFactor, 'f', -1, 64)}
				})
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newCurrencyViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a currency",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("currency", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			c, err := client.GetCurrency(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, c.Raw, []string{"id", "name", "round_factor"})
		},
	}
}

func newCurrencyCodesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "codes",
		Short: "List all possible ISO 4217 currency codes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.CurrencyCodes(cmd.Context())
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), raw)
			}
			// The response nests arrays of code strings; flatten for humans.
			var flat func(v any)
			var codes []string
			flat = func(v any) {
				switch t := v.(type) {
				case string:
					codes = append(codes, t)
				case []any:
					for _, e := range t {
						flat(e)
					}
				}
			}
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			flat(v)
			for _, code := range codes {
				fmt.Fprintln(cmd.OutOrStdout(), code)
			}
			return nil
		},
	}
}

// --- tax (3.0) ----------------------------------------------------------

func newTaxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tax",
		Short: "List and view taxes",
	}
	cmd.AddCommand(newTaxListCmd(), newTaxViewCmd())
	return cmd
}

func newTaxListCmd() *cobra.Command {
	var opts api.TaxListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List taxes (defaults to active sales taxes)",
		Example: `  bexio tax list
  bexio tax list --types pre_tax
  bexio tax list --scope inactive --types ""`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			taxes, err := client.ListTaxes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, taxes, []string{"id", "code", "value", "display_name"},
				func(t api.Tax) json.RawMessage { return t.Raw },
				func(t api.Tax) []string {
					return []string{
						strconv.Itoa(t.ID), t.Code,
						strconv.FormatFloat(t.Value, 'f', -1, 64),
						output.Truncate(t.DisplayName, 50),
					}
				})
		},
	}
	lookupFlags30(cmd, &opts.ListOptions)
	cmd.Flags().StringVar(&opts.Types, "types", "sales_tax", `filter by type: "sales_tax" or "pre_tax" ("" for all)`)
	cmd.Flags().StringVar(&opts.Scope, "scope", "active", `filter by scope: "active" or "inactive" ("" for all)`)
	cmd.Flags().StringVar(&opts.Date, "date", "", "only taxes active at this date (YYYY-MM-DD)")
	return cmd
}

func newTaxViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a tax",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("tax", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			t, err := client.GetTax(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, t.Raw, []string{
				"id", "uuid", "name", "display_name", "code", "digit", "type",
				"value", "account_id", "start_year", "end_year", "is_active",
			})
		},
	}
}

// --- bank-account (3.0, read-only) --------------------------------------

func newBankAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bank-account",
		Short: "List and view bank accounts (read-only)",
	}
	cmd.AddCommand(newBankAccountListCmd(), newBankAccountViewCmd())
	return cmd
}

func newBankAccountListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bank accounts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			accounts, err := client.ListBankAccounts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, accounts, []string{"id", "name", "bank_name", "iban_nr", "currency_id"},
				func(b api.BankAccount) json.RawMessage { return b.Raw },
				func(b api.BankAccount) []string {
					return []string{
						strconv.Itoa(b.ID), output.Truncate(b.Name, 40),
						output.Truncate(b.BankName, 30), b.IbanNr, strconv.Itoa(b.CurrencyID),
					}
				})
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newBankAccountViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a bank account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("bank account", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			b, err := client.GetBankAccount(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, b.Raw, []string{
				"id", "name", "type", "bank_name", "iban_nr", "qr_invoice_iban",
				"invoice_mode", "currency_id", "account_id", "owner",
			})
		},
	}
}

// --- user (3.0, read-only) ----------------------------------------------

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "List and view users (read-only)",
	}
	cmd.AddCommand(newUserListCmd(), newUserViewCmd(), newUserMeCmd())
	return cmd
}

var userDetailOrder = []string{
	"id", "salutation_type", "firstname", "lastname", "email",
	"is_superadmin", "is_accountant",
}

func newUserListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			users, err := client.ListUsers(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderLookupList(cmd, users, []string{"id", "name", "email"},
				func(u api.User) json.RawMessage { return u.Raw },
				func(u api.User) []string { return []string{strconv.Itoa(u.ID), u.DisplayName(), u.Email} })
		},
	}
	lookupFlags30(cmd, &opts)
	return cmd
}

func newUserViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("user", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			u, err := client.GetUser(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, u.Raw, userDetailOrder)
		},
	}
}

func newUserMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the authenticated user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			u, err := client.Me(cmd.Context())
			if err != nil {
				return err
			}
			return renderDetail(cmd, u.Raw, userDetailOrder)
		},
	}
}

// --- company-profile (2.0, read-only) -----------------------------------

func newCompanyProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "company-profile",
		Short: "List and view company profiles (read-only)",
	}
	cmd.AddCommand(newCompanyProfileListCmd(), newCompanyProfileViewCmd())
	return cmd
}

func newCompanyProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List company profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			profiles, err := client.ListCompanyProfiles(cmd.Context())
			if err != nil {
				return err
			}
			return renderLookupList(cmd, profiles, []string{"id", "name"},
				func(p api.CompanyProfileDetail) json.RawMessage { return p.Raw },
				func(p api.CompanyProfileDetail) []string { return []string{strconv.Itoa(p.ID), p.Name} })
		},
	}
}

func newCompanyProfileViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a company profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("company profile", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			p, err := client.GetCompanyProfile(cmd.Context(), id)
			if err != nil {
				return err
			}
			raw := p.Raw
			if flagOutput != "json" {
				// Strip the embedded base64 logo from the table view; it
				// stays in -o json output.
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err == nil {
					delete(m, "logo_base64")
					if b, err := json.Marshal(m); err == nil {
						raw = b
					}
				}
			}
			return renderDetail(cmd, raw, []string{
				"id", "name", "legal_form", "address", "address_nr", "postcode",
				"city", "country_name", "mail", "phone_fixed", "phone_mobile",
				"url", "ust_id_nr", "mwst_nr", "trade_register_nr",
			})
		},
	}
}

// --- permission (3.0, read-only) ----------------------------------------

func newPermissionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Show the access information of the authenticated user",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "view",
		Short: "Print the raw permissions JSON (components + per-component permissions)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			raw, err := client.GetPermissions(cmd.Context())
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), raw)
		},
	})
	return cmd
}
