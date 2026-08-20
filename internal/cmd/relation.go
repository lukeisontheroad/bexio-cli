package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// newRelationCmd manages contact relations (API resource "contact_relation").
func newRelationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contact-relation",
		Aliases: []string{"relation"},
		Short:   "Link contacts to each other (company <-> person)",
	}
	cmd.AddCommand(
		newRelationListCmd(),
		newRelationViewCmd(),
		newRelationSearchCmd(),
		newRelationCreateCmd(),
		newRelationUpdateCmd(),
		newRelationDeleteCmd(),
	)
	return cmd
}

var relationDetailOrder = []string{"id", "contact_id", "contact_sub_id", "description", "updated_at"}

func renderRelations(cmd *cobra.Command, rels []api.Relation) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(rels))
		for i, r := range rels {
			raws[i] = r.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(rels))
	for i, r := range rels {
		rows[i] = []string{
			strconv.Itoa(r.ID),
			strconv.Itoa(r.ContactID),
			strconv.Itoa(r.ContactSubID),
			output.Truncate(r.Description, 40),
			shortDate(r.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "contact_id", "contact_sub_id", "description", "updated"}, rows)
	return nil
}

func newRelationListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contact relations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			rels, err := client.ListRelations(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderRelations(cmd, rels)
		},
	}
	listFlags(cmd, &opts, "id, contact_id, contact_sub_id, updated_at")
	return cmd
}

func newRelationViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a contact relation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact relation", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.GetRelation(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, r.Raw, relationDetailOrder)
		},
	}
}

func newRelationSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search contact relations",
		Example: `  bexio contact-relation search --where contact_id=17
  bexio contact-relation search --where contact_sub_id=42`,
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
				return fmt.Errorf("nothing to search: give at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			rels, err := client.SearchRelations(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderRelations(cmd, rels)
		},
	}
	listFlags(cmd, &opts, "id, contact_id, contact_sub_id, updated_at")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on contact_id, contact_sub_id, or description (repeatable, ANDed)")
	return cmd
}

func newRelationCreateCmd() *cobra.Command {
	var contactID, contactSubID int
	var description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a contact relation",
		Long:  "Create a contact relation. --contact-id is the company side, --contact-sub-id the person side.",
		Example: `  bexio contact-relation create --contact-id 17 --contact-sub-id 42
  bexio contact-relation create --contact-id 17 --contact-sub-id 42 --description CEO`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			if contactID == 0 || contactSubID == 0 {
				return fmt.Errorf("--contact-id and --contact-sub-id are required")
			}
			fields := map[string]any{
				"contact_id":     contactID,
				"contact_sub_id": contactSubID,
			}
			if cmd.Flags().Changed("description") {
				fields["description"] = description
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.CreateRelation(cmd.Context(), fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), r.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created contact relation %d (%d <-> %d)\n", r.ID, r.ContactID, r.ContactSubID)
			return nil
		},
	}
	cmd.Flags().IntVar(&contactID, "contact-id", 0, "id of the company-side contact (required)")
	cmd.Flags().IntVar(&contactSubID, "contact-sub-id", 0, "id of the person-side contact (required)")
	cmd.Flags().StringVar(&description, "description", "", "description of the relation")
	return cmd
}

func newRelationUpdateCmd() *cobra.Command {
	var contactID, contactSubID int
	var description string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a contact relation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("contact relation", args[0])
			if err != nil {
				return err
			}
			fields := map[string]any{}
			setIfChanged(cmd, fields, "contact-id", "contact_id", contactID)
			setIfChanged(cmd, fields, "contact-sub-id", "contact_sub_id", contactSubID)
			setIfChanged(cmd, fields, "description", "description", description)
			if len(fields) == 0 {
				return fmt.Errorf("nothing to update: pass at least one field flag")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			r, err := client.UpdateRelation(cmd.Context(), id, fields)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), r.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated contact relation %d\n", r.ID)
			return nil
		},
	}
	cmd.Flags().IntVar(&contactID, "contact-id", 0, "id of the company-side contact")
	cmd.Flags().IntVar(&contactSubID, "contact-sub-id", 0, "id of the person-side contact")
	cmd.Flags().StringVar(&description, "description", "", "description of the relation")
	return cmd
}

func newRelationDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a contact relation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("contact relation", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteRelation(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted contact relation %d\n", id)
			return nil
		},
	}
}
