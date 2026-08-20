package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// newAddressCmd manages the additional addresses of one contact
// (/2.0/contact/{id}/additional_address).
func newAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Manage the additional addresses of a contact",
	}
	cmd.AddCommand(
		newAddressListCmd(),
		newAddressViewCmd(),
		newAddressCreateCmd(),
		newAddressUpdateCmd(),
		newAddressDeleteCmd(),
	)
	return cmd
}

var addressDetailOrder = []string{
	"id", "name", "name_addition", "street_name", "house_number",
	"address_addition", "postcode", "city", "country_id", "subject",
	"description",
}

func renderAddresses(cmd *cobra.Command, addrs []api.Address) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(addrs))
		for i, a := range addrs {
			raws[i] = a.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(addrs))
	for i, a := range addrs {
		street := a.StreetName
		if a.HouseNumber != "" {
			street += " " + a.HouseNumber
		}
		rows[i] = []string{
			strconv.Itoa(a.ID),
			output.Truncate(a.Name, 30),
			street,
			a.Postcode,
			a.City,
			output.Truncate(a.Subject, 30),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name", "street", "postcode", "city", "subject"}, rows)
	return nil
}

type addressFieldFlags struct {
	name, nameAddition                       string
	streetName, houseNumber, addressAddition string
	postcode, city                           string
	countryID                                int
	subject, description                     string
}

func (f *addressFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "address name")
	fl.StringVar(&f.nameAddition, "name-addition", "", "name addition")
	fl.StringVar(&f.streetName, "street-name", "", "street name")
	fl.StringVar(&f.houseNumber, "house-number", "", "house number (requires --street-name)")
	fl.StringVar(&f.addressAddition, "address-addition", "", "address addition (requires --street-name)")
	fl.StringVar(&f.postcode, "postcode", "", "postcode")
	fl.StringVar(&f.city, "city", "", "city")
	fl.IntVar(&f.countryID, "country-id", 0, "country id")
	fl.StringVar(&f.subject, "subject", "", "subject")
	fl.StringVar(&f.description, "description", "", "description")
}

func (f *addressFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "name", "name", f.name)
	setIfChanged(cmd, fields, "name-addition", "name_addition", f.nameAddition)
	setIfChanged(cmd, fields, "street-name", "street_name", f.streetName)
	setIfChanged(cmd, fields, "house-number", "house_number", f.houseNumber)
	setIfChanged(cmd, fields, "address-addition", "address_addition", f.addressAddition)
	setIfChanged(cmd, fields, "postcode", "postcode", f.postcode)
	setIfChanged(cmd, fields, "city", "city", f.city)
	setIfChanged(cmd, fields, "country-id", "country_id", f.countryID)
	setIfChanged(cmd, fields, "subject", "subject", f.subject)
	setIfChanged(cmd, fields, "description", "description", f.description)
	return fields
}

func newAddressListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:     "list <contact-id>",
		Short:   "List the additional addresses of a contact",
		Example: `  bexio contact address list 17`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			contactID, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			addrs, err := client.ListAddresses(cmd.Context(), contactID, opts)
			if err != nil {
				return err
			}
			return renderAddresses(cmd, addrs)
		},
	}
	listFlags(cmd, &opts, "id, name, postcode, country_id")
	return cmd
}

func newAddressViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <contact-id> <address-id>",
		Short: "Show an additional address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			contactID, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("address", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			a, err := client.GetAddress(cmd.Context(), contactID, id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, a.Raw, addressDetailOrder)
		},
	}
}

func newAddressCreateCmd() *cobra.Command {
	var fields addressFieldFlags
	cmd := &cobra.Command{
		Use:   "create <contact-id>",
		Short: "Create an additional address",
		Example: `  bexio contact address create 17 --name "Warehouse" \
      --street-name "Lagerstrasse" --house-number 1 --postcode 8005 --city Zürich`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			contactID, err := parseID("contact", args[0])
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
			a, err := client.CreateAddress(cmd.Context(), contactID, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created additional address %d for contact %d\n", a.ID, contactID)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newAddressUpdateCmd() *cobra.Command {
	var fields addressFieldFlags
	cmd := &cobra.Command{
		Use:   "update <contact-id> <address-id>",
		Short: "Update an additional address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			contactID, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("address", args[1])
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
			a, err := client.UpdateAddress(cmd.Context(), contactID, id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated additional address %d\n", a.ID)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newAddressDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <contact-id> <address-id>",
		Short: "Delete an additional address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			contactID, err := parseID("contact", args[0])
			if err != nil {
				return err
			}
			id, err := parseID("address", args[1])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteAddress(cmd.Context(), contactID, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted additional address %d\n", id)
			return nil
		},
	}
}
