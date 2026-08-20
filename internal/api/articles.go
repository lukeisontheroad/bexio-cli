package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Article is an item (API resource "article"). Raw preserves the full API
// object for --output json.
type Article struct {
	ID               int    `json:"id"`
	ArticleTypeID    int    `json:"article_type_id"`
	InternCode       string `json:"intern_code"`
	InternName       string `json:"intern_name"`
	SalePrice        string `json:"sale_price"`
	PurchasePrice    string `json:"purchase_price"`
	IsStock          bool   `json:"is_stock"`
	StockID          int    `json:"stock_id"`
	StockNr          int    `json:"stock_nr"`
	StockAvailableNr int    `json:"stock_available_nr"`

	Raw json.RawMessage `json:"-"`
}

func (a *Article) UnmarshalJSON(data []byte) error {
	type article Article // avoid recursion
	var v article
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = Article(v)
	a.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// TypeName maps article_type_id to its meaning (1 physical, 2 service).
func (a Article) TypeName() string {
	switch a.ArticleTypeID {
	case 1:
		return "physical"
	case 2:
		return "service"
	default:
		return fmt.Sprintf("type %d", a.ArticleTypeID)
	}
}

// ListArticles fetches /2.0/article.
func (c *Client) ListArticles(ctx context.Context, opts ListOptions) ([]Article, error) {
	var out []Article
	return out, c.Get(ctx, "/2.0/article", opts.values(), &out)
}

// SearchArticles posts to /2.0/article/search.
func (c *Client) SearchArticles(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Article, error) {
	var out []Article
	return out, c.Do(ctx, http.MethodPost, "/2.0/article/search", opts.values(), criteria, &out)
}

// GetArticle fetches a single item.
func (c *Client) GetArticle(ctx context.Context, id int) (*Article, error) {
	var out Article
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/article/%d", id), nil, &out)
}

// CreateArticle creates an item. fields uses the raw API field names.
func (c *Client) CreateArticle(ctx context.Context, fields map[string]any) (*Article, error) {
	var out Article
	return &out, c.Do(ctx, http.MethodPost, "/2.0/article", nil, fields, &out)
}

// UpdateArticle edits an item (only the provided fields change).
func (c *Client) UpdateArticle(ctx context.Context, id int, fields map[string]any) (*Article, error) {
	var out Article
	return &out, c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/article/%d", id), nil, fields, &out)
}

// DeleteArticle deletes an item.
func (c *Client) DeleteArticle(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/2.0/article/%d", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete article %d: API reported failure", id)
	}
	return nil
}

// Stock is a stock location (API resource "stock", read-only).
type Stock struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (s *Stock) UnmarshalJSON(data []byte) error {
	type stock Stock
	var v stock
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = Stock(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListStocks fetches /2.0/stock.
func (c *Client) ListStocks(ctx context.Context, opts ListOptions) ([]Stock, error) {
	var out []Stock
	return out, c.Get(ctx, "/2.0/stock", opts.values(), &out)
}

// SearchStocks posts to /2.0/stock/search.
func (c *Client) SearchStocks(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]Stock, error) {
	var out []Stock
	return out, c.Do(ctx, http.MethodPost, "/2.0/stock/search", opts.values(), criteria, &out)
}

// StockArea is a stock area (API resource "stock_place", read-only).
type StockArea struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Raw json.RawMessage `json:"-"`
}

func (s *StockArea) UnmarshalJSON(data []byte) error {
	type stockArea StockArea
	var v stockArea
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = StockArea(v)
	s.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ListStockAreas fetches /2.0/stock_place.
func (c *Client) ListStockAreas(ctx context.Context, opts ListOptions) ([]StockArea, error) {
	var out []StockArea
	return out, c.Get(ctx, "/2.0/stock_place", opts.values(), &out)
}

// SearchStockAreas posts to /2.0/stock_place/search.
func (c *Client) SearchStockAreas(ctx context.Context, criteria []SearchCriterion, opts ListOptions) ([]StockArea, error) {
	var out []StockArea
	return out, c.Do(ctx, http.MethodPost, "/2.0/stock_place/search", opts.values(), criteria, &out)
}

// Delivery is a delivery note (API resource "kb_delivery"). Deliveries are
// created from orders (POST /2.0/kb_order/{id}/delivery); the kb_delivery
// endpoints only list, view, and issue them.
type Delivery struct {
	ID             int    `json:"id"`
	DocumentNr     string `json:"document_nr"`
	Title          string `json:"title"`
	ContactID      int    `json:"contact_id"`
	Total          string `json:"total"`
	KbItemStatusID int    `json:"kb_item_status_id"`
	IsValidFrom    string `json:"is_valid_from"`
	UpdatedAt      string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (d *Delivery) UnmarshalJSON(data []byte) error {
	type delivery Delivery
	var v delivery
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*d = Delivery(v)
	d.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// StatusName maps kb_item_status_id to its meaning for deliveries
// (10 draft, 18 done, 20 canceled).
func (d Delivery) StatusName() string {
	switch d.KbItemStatusID {
	case 10:
		return "draft"
	case 18:
		return "done"
	case 20:
		return "canceled"
	default:
		return fmt.Sprintf("status %d", d.KbItemStatusID)
	}
}

// ListDeliveries fetches /2.0/kb_delivery (no search endpoint exists).
func (c *Client) ListDeliveries(ctx context.Context, opts ListOptions) ([]Delivery, error) {
	var out []Delivery
	return out, c.Get(ctx, "/2.0/kb_delivery", opts.values(), &out)
}

// GetDelivery fetches a single delivery.
func (c *Client) GetDelivery(ctx context.Context, id int) (*Delivery, error) {
	var out Delivery
	return &out, c.Get(ctx, fmt.Sprintf("/2.0/kb_delivery/%d", id), nil, &out)
}

// IssueDelivery issues a draft delivery (POST /2.0/kb_delivery/{id}/issue),
// which books the stock movements.
func (c *Client) IssueDelivery(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/2.0/kb_delivery/%d/issue", id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("issue delivery %d: API reported failure", id)
	}
	return nil
}
