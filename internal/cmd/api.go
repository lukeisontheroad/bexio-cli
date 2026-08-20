package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

// newAPICmd is the raw API escape hatch.
func newAPICmd() *cobra.Command {
	var fields []string
	var body string
	cmd := &cobra.Command{
		Use:   "api <method> <path>",
		Short: "Make a raw bexio API request",
		Long: `Make an authenticated request against any bexio API endpoint and print the
JSON response. For GET/DELETE, --field values become query parameters; for
other methods they form a flat JSON object body (string values). --body
sends a raw JSON body instead (use "-" to read it from stdin), which is
needed for endpoints that take a JSON array, like the search endpoints.`,
		Example: `  bexio api GET /2.0/country
  bexio api GET /2.0/contact --field limit=10 --field order_by=name_1
  bexio api POST /2.0/contact/search --body '[{"field":"mail","value":"%acme%","criteria":"like"}]'
  bexio api DELETE /2.0/note/12`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}

			kv := url.Values{}
			for _, f := range fields {
				k, v, ok := strings.Cut(f, "=")
				if !ok {
					return fmt.Errorf("invalid --field %q (want key=value)", f)
				}
				kv.Add(k, v)
			}

			var query url.Values
			var reqBody any
			switch {
			case body != "":
				raw := []byte(body)
				if body == "-" {
					data, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return err
					}
					raw = data
				}
				var v json.RawMessage
				if err := json.Unmarshal(raw, &v); err != nil {
					return fmt.Errorf("--body is not valid JSON: %w", err)
				}
				reqBody = v
				query = kv
			case method == http.MethodGet || method == http.MethodDelete:
				query = kv
			case len(fields) > 0:
				m := map[string]any{}
				for k := range kv {
					m[k] = kv.Get(k)
				}
				reqBody = m
			}

			client, err := newClient()
			if err != nil {
				return err
			}
			var out json.RawMessage
			if err := client.Do(cmd.Context(), method, path, query, reqBody, &out); err != nil {
				return err
			}
			if out == nil {
				return nil
			}
			return output.JSON(cmd.OutOrStdout(), out)
		},
	}
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "key=value pair: query parameter (GET/DELETE) or JSON body field (repeatable)")
	cmd.Flags().StringVar(&body, "body", "", `raw JSON request body ("-" reads stdin)`)
	return cmd
}
