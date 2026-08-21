package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lukeisontheroad/bexio-cli/internal/api"
	"github.com/lukeisontheroad/bexio-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() { registerModule(newArticleCmd) }
func init() { registerModule(newStockCmd) }
func init() { registerModule(newStockAreaCmd) }
func init() { registerModule(newDeliveryCmd) }

// newArticleCmd is the command group for items (API resource "article").
func newArticleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "article",
		Aliases: []string{"item"},
		Short:   "List, view, search, and modify items (products & services)",
	}
	cmd.AddCommand(
		newArticleListCmd(),
		newArticleViewCmd(),
		newArticleSearchCmd(),
		newArticleCreateCmd(),
		newArticleUpdateCmd(),
		newArticleDeleteCmd(),
	)
	return cmd
}

func renderArticles(cmd *cobra.Command, articles []api.Article) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(articles))
		for i, a := range articles {
			raws[i] = a.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(articles))
	for i, a := range articles {
		stock := ""
		if a.IsStock {
			stock = strconv.Itoa(a.StockAvailableNr)
		}
		rows[i] = []string{
			strconv.Itoa(a.ID),
			a.InternCode,
			a.TypeName(),
			output.Truncate(a.InternName, 40),
			a.SalePrice,
			a.PurchasePrice,
			stock,
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "code", "type", "name", "sale", "purchase", "stock"}, rows)
	return nil
}

// articleDetailOrder is the field order for the table view of a single item;
// remaining raw fields follow alphabetically.
var articleDetailOrder = []string{
	"id", "article_type_id", "intern_code", "intern_name", "intern_description",
	"sale_price", "purchase_price", "sale_total", "purchase_total",
	"currency_id", "tax_income_id", "tax_expense_id", "unit_id",
	"is_stock", "stock_id", "stock_place_id", "stock_nr", "stock_min_nr",
	"stock_reserved_nr", "stock_available_nr", "stock_picked_nr",
	"stock_disposed_nr", "stock_ordered_nr", "contact_id", "deliverer_code",
	"deliverer_name", "deliverer_description", "delivery_price",
	"article_group_id", "account_id", "expense_account_id", "width", "height",
	"weight", "volume", "remarks", "user_id",
}

func newArticleListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List items",
		Example: `  bexio article list
  bexio article list --order-by intern_name --limit 500
  bexio article list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			articles, err := client.ListArticles(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderArticles(cmd, articles)
		},
	}
	listFlags(cmd, &opts, "id, intern_name")
	return cmd
}

func newArticleViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("article", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			a, err := client.GetArticle(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, a.Raw, articleDetailOrder)
		},
	}
}

func newArticleSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search items",
		Long: `Search items. A bare term matches intern_name partially. --where clauses
use the raw API field names and add AND conditions (see "bexio contact
search --help" for the operator syntax).

Searchable fields include: id, intern_name, intern_code, intern_description,
article_type_id, contact_id, deliverer_code, deliverer_name, is_stock,
stock_id, stock_nr, currency_id, unit_id, article_group_id.`,
		Example: `  bexio article search Schraube
  bexio article search --where intern_code=A-1001
  bexio article search --where article_type_id=1 --where "stock_nr<10" -o json`,
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
					Field: "intern_name", Value: "%" + args[0] + "%", Criteria: "like",
				})
			}
			if len(criteria) == 0 {
				return fmt.Errorf("nothing to search: give a term or at least one --where clause")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			articles, err := client.SearchArticles(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderArticles(cmd, articles)
		},
	}
	listFlags(cmd, &opts, "id, intern_name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause (repeatable, ANDed); see long help")
	return cmd
}

// articleFieldFlags mirrors the writable API payload fields of
// POST /2.0/article.
type articleFieldFlags struct {
	articleTypeID                                 int
	internCode, internName, internDescription     string
	purchasePrice, salePrice                      string
	purchaseTotal, saleTotal                      float64
	currencyID, taxIncomeID, taxExpenseID, unitID int
	isStock                                       bool
	stockID, stockPlaceID, stockNr, stockMinNr    int
	contactID                                     int
	delivererCode, delivererName, delivererDescr  string
	deliveryPrice                                 float64
	articleGroupID, accountID, expenseAccountID   int
	width, height, weight, volume                 int
	htmlText, remarks                             string
	userID                                        int
}

// register adds one flag per API field, named exactly after the field
// (underscores as hyphens).
func (f *articleFieldFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.IntVar(&f.articleTypeID, "article-type-id", 0, "item type: 1 physical product, 2 service")
	fl.StringVar(&f.internCode, "intern-code", "", "internal item code / number")
	fl.StringVar(&f.internName, "intern-name", "", "item name")
	fl.StringVar(&f.internDescription, "intern-description", "", "item description")
	fl.StringVar(&f.purchasePrice, "purchase-price", "", "purchase price (decimal string)")
	fl.StringVar(&f.salePrice, "sale-price", "", "sale price (decimal string)")
	fl.Float64Var(&f.purchaseTotal, "purchase-total", 0, "purchase total")
	fl.Float64Var(&f.saleTotal, "sale-total", 0, "sale total")
	fl.IntVar(&f.currencyID, "currency-id", 0, "currency id (list with: bexio api GET /3.0/currencies)")
	fl.IntVar(&f.taxIncomeID, "tax-income-id", 0, "income tax id (list with: bexio api GET /3.0/taxes)")
	fl.IntVar(&f.taxExpenseID, "tax-expense-id", 0, "expense tax id")
	fl.IntVar(&f.unitID, "unit-id", 0, "unit id (list with: bexio api GET /2.0/unit)")
	fl.BoolVar(&f.isStock, "is-stock", false, "enable stock management (needs stock_edit scope)")
	fl.IntVar(&f.stockID, "stock-id", 0, "stock location id (see: bexio stock list)")
	fl.IntVar(&f.stockPlaceID, "stock-place-id", 0, "stock area id (see: bexio stock-area list)")
	fl.IntVar(&f.stockNr, "stock-nr", 0, "stock quantity (only settable before any bookings exist)")
	fl.IntVar(&f.stockMinNr, "stock-min-nr", 0, "minimum stock quantity")
	fl.IntVar(&f.contactID, "contact-id", 0, "supplier contact id")
	fl.StringVar(&f.delivererCode, "deliverer-code", "", "deliverer item code")
	fl.StringVar(&f.delivererName, "deliverer-name", "", "deliverer item name")
	fl.StringVar(&f.delivererDescr, "deliverer-description", "", "deliverer item description")
	fl.Float64Var(&f.deliveryPrice, "delivery-price", 0, "delivery price")
	fl.IntVar(&f.articleGroupID, "article-group-id", 0, "item group id")
	fl.IntVar(&f.accountID, "account-id", 0, "income account id")
	fl.IntVar(&f.expenseAccountID, "expense-account-id", 0, "expense account id")
	fl.IntVar(&f.width, "width", 0, "width")
	fl.IntVar(&f.height, "height", 0, "height")
	fl.IntVar(&f.weight, "weight", 0, "weight")
	fl.IntVar(&f.volume, "volume", 0, "volume")
	fl.StringVar(&f.htmlText, "html-text", "", "HTML text")
	fl.StringVar(&f.remarks, "remarks", "", "remarks")
	fl.IntVar(&f.userID, "user-id", 0, "user id (defaults to the authenticated user)")
}

// payload collects the fields the user actually set.
func (f *articleFieldFlags) payload(cmd *cobra.Command) map[string]any {
	fields := map[string]any{}
	setIfChanged(cmd, fields, "article-type-id", "article_type_id", f.articleTypeID)
	setIfChanged(cmd, fields, "intern-code", "intern_code", f.internCode)
	setIfChanged(cmd, fields, "intern-name", "intern_name", f.internName)
	setIfChanged(cmd, fields, "intern-description", "intern_description", f.internDescription)
	setIfChanged(cmd, fields, "purchase-price", "purchase_price", f.purchasePrice)
	setIfChanged(cmd, fields, "sale-price", "sale_price", f.salePrice)
	setIfChanged(cmd, fields, "purchase-total", "purchase_total", f.purchaseTotal)
	setIfChanged(cmd, fields, "sale-total", "sale_total", f.saleTotal)
	setIfChanged(cmd, fields, "currency-id", "currency_id", f.currencyID)
	setIfChanged(cmd, fields, "tax-income-id", "tax_income_id", f.taxIncomeID)
	setIfChanged(cmd, fields, "tax-expense-id", "tax_expense_id", f.taxExpenseID)
	setIfChanged(cmd, fields, "unit-id", "unit_id", f.unitID)
	setIfChanged(cmd, fields, "is-stock", "is_stock", f.isStock)
	setIfChanged(cmd, fields, "stock-id", "stock_id", f.stockID)
	setIfChanged(cmd, fields, "stock-place-id", "stock_place_id", f.stockPlaceID)
	setIfChanged(cmd, fields, "stock-nr", "stock_nr", f.stockNr)
	setIfChanged(cmd, fields, "stock-min-nr", "stock_min_nr", f.stockMinNr)
	setIfChanged(cmd, fields, "contact-id", "contact_id", f.contactID)
	setIfChanged(cmd, fields, "deliverer-code", "deliverer_code", f.delivererCode)
	setIfChanged(cmd, fields, "deliverer-name", "deliverer_name", f.delivererName)
	setIfChanged(cmd, fields, "deliverer-description", "deliverer_description", f.delivererDescr)
	setIfChanged(cmd, fields, "delivery-price", "delivery_price", f.deliveryPrice)
	setIfChanged(cmd, fields, "article-group-id", "article_group_id", f.articleGroupID)
	setIfChanged(cmd, fields, "account-id", "account_id", f.accountID)
	setIfChanged(cmd, fields, "expense-account-id", "expense_account_id", f.expenseAccountID)
	setIfChanged(cmd, fields, "width", "width", f.width)
	setIfChanged(cmd, fields, "height", "height", f.height)
	setIfChanged(cmd, fields, "weight", "weight", f.weight)
	setIfChanged(cmd, fields, "volume", "volume", f.volume)
	setIfChanged(cmd, fields, "html-text", "html_text", f.htmlText)
	setIfChanged(cmd, fields, "remarks", "remarks", f.remarks)
	setIfChanged(cmd, fields, "user-id", "user_id", f.userID)
	return fields
}

func newArticleCreateCmd() *cobra.Command {
	var fields articleFieldFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an item",
		Long: `Create an item. --intern-name is required; --article-type-id defaults to
1 (physical product), use 2 for services. user_id defaults to the
authenticated user. Stock fields (--is-stock, --stock-id, ...) require the
stock_edit scope.`,
		Example: `  bexio article create --intern-name "Schraube M4" --intern-code A-1001 --sale-price 0.25
  bexio article create --intern-name Beratung --article-type-id 2 --sale-price 180`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			payload := fields.payload(cmd)
			if payload["intern_name"] == nil {
				return fmt.Errorf("--intern-name is required")
			}
			if payload["article_type_id"] == nil {
				payload["article_type_id"] = 1
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
			a, err := client.CreateArticle(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created article %d (%s)\n", a.ID, a.InternName)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newArticleUpdateCmd() *cobra.Command {
	var fields articleFieldFlags
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update fields of an item",
		Long:  "Update an item. Only the flags you pass are changed.",
		Example: `  bexio article update 4 --sale-price 0.30
  bexio article update 4 --stock-min-nr 50 --stock-place-id 2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("article", args[0])
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
			a, err := client.UpdateArticle(cmd.Context(), id, payload)
			if err != nil {
				return err
			}
			if flagOutput == "json" {
				return output.JSON(cmd.OutOrStdout(), a.Raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated article %d (%s)\n", a.ID, a.InternName)
			return nil
		},
	}
	fields.register(cmd)
	return cmd
}

func newArticleDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("article", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.DeleteArticle(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted article %d\n", id)
			return nil
		},
	}
}

// newStockCmd lists stock locations (API resource "stock", read-only in the
// API; locations are managed in the bexio web UI).
func newStockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stock",
		Short: "List stock locations (read-only)",
	}
	cmd.AddCommand(newStockListCmd(), newStockSearchCmd())
	return cmd
}

func renderStocks(cmd *cobra.Command, raws []json.RawMessage, ids []int, names []string) error {
	if flagOutput == "json" {
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(ids))
	for i := range ids {
		rows[i] = []string{strconv.Itoa(ids[i]), names[i]}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "name"}, rows)
	return nil
}

func renderStockList(cmd *cobra.Command, stocks []api.Stock) error {
	raws := make([]json.RawMessage, len(stocks))
	ids := make([]int, len(stocks))
	names := make([]string, len(stocks))
	for i, s := range stocks {
		raws[i], ids[i], names[i] = s.Raw, s.ID, s.Name
	}
	return renderStocks(cmd, raws, ids, names)
}

func renderStockAreaList(cmd *cobra.Command, areas []api.StockArea) error {
	raws := make([]json.RawMessage, len(areas))
	ids := make([]int, len(areas))
	names := make([]string, len(areas))
	for i, s := range areas {
		raws[i], ids[i], names[i] = s.Raw, s.ID, s.Name
	}
	return renderStocks(cmd, raws, ids, names)
}

func newStockListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stock locations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			stocks, err := client.ListStocks(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderStockList(cmd, stocks)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newStockSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search stock locations",
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
			stocks, err := client.SearchStocks(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderStockList(cmd, stocks)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}

// newStockAreaCmd lists stock areas (API resource "stock_place", read-only
// in the API; areas are managed in the bexio web UI).
func newStockAreaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stock-area",
		Short: "List stock areas (read-only)",
	}
	cmd.AddCommand(newStockAreaListCmd(), newStockAreaSearchCmd())
	return cmd
}

func newStockAreaListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stock areas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			areas, err := client.ListStockAreas(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderStockAreaList(cmd, areas)
		},
	}
	listFlags(cmd, &opts, "id, name")
	return cmd
}

func newStockAreaSearchCmd() *cobra.Command {
	var opts api.ListOptions
	var where []string
	cmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search stock areas",
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
			areas, err := client.SearchStockAreas(cmd.Context(), criteria, opts)
			if err != nil {
				return err
			}
			return renderStockAreaList(cmd, areas)
		},
	}
	listFlags(cmd, &opts, "id, name")
	cmd.Flags().StringArrayVarP(&where, "where", "w", nil, "filter clause on id or name (repeatable, ANDed)")
	return cmd
}

// newDeliveryCmd is the command group for deliveries (API resource
// "kb_delivery").
func newDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delivery",
		Aliases: []string{"kb-delivery"},
		Short:   "List, view, and issue deliveries",
		Long: `List, view, and issue deliveries (delivery notes). The API cannot create
deliveries directly: create them from an order with "bexio order
delivery <order-id>", then issue them here to book the stock movements.

Status ids: 10 draft, 18 done, 20 canceled.`,
	}
	cmd.AddCommand(
		newDeliveryListCmd(),
		newDeliveryViewCmd(),
		newDeliveryIssueCmd(),
	)
	return cmd
}

func renderDeliveries(cmd *cobra.Command, deliveries []api.Delivery) error {
	if flagOutput == "json" {
		raws := make([]json.RawMessage, len(deliveries))
		for i, d := range deliveries {
			raws[i] = d.Raw
		}
		return output.JSON(cmd.OutOrStdout(), raws)
	}
	rows := make([][]string, len(deliveries))
	for i, d := range deliveries {
		rows[i] = []string{
			strconv.Itoa(d.ID),
			d.DocumentNr,
			d.StatusName(),
			output.Truncate(d.Title, 40),
			strconv.Itoa(d.ContactID),
			d.Total,
			shortDate(d.UpdatedAt),
		}
	}
	output.Table(cmd.OutOrStdout(), []string{"id", "nr", "status", "title", "contact", "total", "updated"}, rows)
	return nil
}

// deliveryDetailOrder is the field order for the table view of a single
// delivery; remaining raw fields follow alphabetically.
var deliveryDetailOrder = []string{
	"id", "document_nr", "kb_item_status_id", "title", "contact_id",
	"contact_sub_id", "contact_address", "delivery_address_type",
	"delivery_address", "is_valid_from", "total_net", "total_taxes",
	"total_gross", "total", "mwst_type", "mwst_is_net", "currency_id",
	"language_id", "bank_account_id", "logopaper_id", "header", "footer",
	"api_reference", "user_id", "updated_at",
}

func newDeliveryListCmd() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deliveries",
		Example: `  bexio delivery list
  bexio delivery list --order-by updated_at_desc -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			deliveries, err := client.ListDeliveries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return renderDeliveries(cmd, deliveries)
		},
	}
	listFlags(cmd, &opts, "id, total, total_net, total_gross, updated_at")
	return cmd
}

func newDeliveryViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show a single delivery",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			id, err := parseID("delivery", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			d, err := client.GetDelivery(cmd.Context(), id)
			if err != nil {
				return err
			}
			return renderDetail(cmd, d.Raw, deliveryDetailOrder)
		},
	}
}

func newDeliveryIssueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue <id>",
		Short: "Issue a draft delivery",
		Long: `Issue a draft delivery (status 10). Issuing books the stock movements and
sets the delivery to done (status 18).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("delivery", args[0])
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			if err := client.IssueDelivery(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Issued delivery %d\n", id)
			return nil
		},
	}
}
