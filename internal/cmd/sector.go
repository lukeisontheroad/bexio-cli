package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// newSectorCmd lists contact sectors (API resource "contact_branch",
// read-only in the API).
func newSectorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contact-sector",
		Aliases: []string{"sector"},
		Short:   "List contact sectors (read-only)",
	}
	cmd.AddCommand(newSectorListCmd(), newSectorSearchCmd())
	return cmd
}

func renderSectors(cmd *cobra.Command, sectors []api.Sector) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(sectors))
		for i, s := range sectors {
			raws[i] = s.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(sectors))
	for i, s := range sectors {
		rows[i] = []string{strconv.Itoa(s.ID), s.Name}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
	return nil
}

func newSectorListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contact sectors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			sectors, err := client.ListSectors(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderSectors(cmd, sectors)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newSectorSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search contact sectors",
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
			sectors, err := client.SearchSectors(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderSectors(cmd, sectors)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}
