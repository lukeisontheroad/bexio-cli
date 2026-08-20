package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// newGroupCmd manages contact groups (API resource "contact_group").
func newGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contact-group",
		Aliases: []string{"group"},
		Short:   "List, view, and modify contact groups",
	}
	cmd.AddCommand(
		newGroupListCmd(),
		newGroupViewCmd(),
		newGroupSearchCmd(),
		newGroupCreateCmd(),
		newGroupUpdateCmd(),
		newGroupDeleteCmd(),
	)
	return cmd
}

func renderGroups(cmd *cobra.Command, groups []api.Group) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(groups))
		for i, g := range groups {
			raws[i] = g.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(groups))
	for i, g := range groups {
		rows[i] = []string{strconv.Itoa(g.ID), g.Name}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
	return nil
}

func newGroupListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contact groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			groups, err := client.ListGroups(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderGroups(cmd, groups)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newGroupViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a contact group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact group", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			g, err := client.GetGroup(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, g.Raw, []string{"id", "name"})
		},
	}
}

func newGroupSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:     "search [term]",
		Short:   "Search contact groups",
		Example: `  bexio contact-group search Newsletter`,
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
			groups, err := client.SearchGroups(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderGroups(cmd, groups)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}

func newGroupCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a contact group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			g, err := client.CreateGroup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), g.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created contact group %d (%s)\n", g.ID, g.Name)
			return nil
		},
	}
}

func newGroupUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id> <name>",
		Short: "Rename a contact group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact group", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			g, err := client.UpdateGroup(cmd.Context(), id, args[1])
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), g.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated contact group %d (%s)\n", g.ID, g.Name)
			return nil
		},
	}
}

func newGroupDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a contact group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("contact group", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteGroup(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted contact group %d\n", id)
			return nil
		},
	}
}
