package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// The purchase resources live on two different API generations:
//
//   - Bills (/4.0/purchase/bills), expenses (/4.0/expenses) and outgoing
//     payments (/4.0/purchase/outgoing-payments) are 4.0 endpoints: ids are
//     UUID strings, updates use PUT and replace the whole object, lists are
//     paginated with limit/page (not limit/offset) and wrapped in a
//     {"data": [...], "paging": {...}} envelope, and deletes answer 204 with
//     an empty body (no {"success": true}).
//   - Purchase orders (/3.0/purchase_orders) are a 3.0 kb-style document:
//     integer ids, limit/offset lists returning a bare array, PUT for
//     updates, and a {"success": bool} delete response.

// PurchaseListOptions are the paging and sorting parameters of the 4.0
// purchase lists. Filters carries the endpoint-specific filter parameters
// (bill_date_start, supplier_id, ...) verbatim.
type PurchaseListOptions struct {
	Limit   int
	Page    int
	Order   string
	Sort    string
	Filters url.Values
}

func (o PurchaseListOptions) values() url.Values {
	q := url.Values{}
	for k, vs := range o.Filters {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Page > 0 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.Order != "" {
		q.Set("order", o.Order)
	}
	if o.Sort != "" {
		q.Set("sort", o.Sort)
	}
	return q
}

// PurchasePaging is the "paging" member of a 4.0 list envelope.
type PurchasePaging struct {
	Page      int `json:"page"`
	PageSize  int `json:"page_size"`
	PageCount int `json:"page_count"`
	ItemCount int `json:"item_count"`
}

// DocumentNumberCheck is the answer of the purchase document number
// validation endpoints.
type DocumentNumberCheck struct {
	Valid           bool   `json:"valid"`
	NextAvailableNo string `json:"next_available_no"`

	Raw json.RawMessage `json:"-"`
}

func (d *DocumentNumberCheck) UnmarshalJSON(data []byte) error {
	type documentNumberCheck DocumentNumberCheck
	var v documentNumberCheck
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*d = DocumentNumberCheck(v)
	d.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Bill is a supplier bill (4.0 resource "purchase/bills"). Raw preserves the
// full API object (line_items, discounts, address, payment on single
// fetches).
type Bill struct {
	ID               string  `json:"id"`
	DocumentNo       string  `json:"document_no"`
	Title            string  `json:"title"`
	Status           string  `json:"status"`
	FirstnameSuffix  string  `json:"firstname_suffix"`
	LastnameCompany  string  `json:"lastname_company"`
	Vendor           string  `json:"vendor"`
	VendorRef        string  `json:"vendor_ref"`
	CreatedAt        string  `json:"created_at"`
	SupplierID       int     `json:"supplier_id"`
	ContactPartnerID int     `json:"contact_partner_id"`
	PurchaseOrderID  int     `json:"purchase_order_id"`
	CurrencyCode     string  `json:"currency_code"`
	PendingAmount    float64 `json:"pending_amount"`
	AmountMan        float64 `json:"amount_man"`
	AmountCalc       float64 `json:"amount_calc"`
	ManualAmount     bool    `json:"manual_amount"`
	Net              float64 `json:"net"`
	Gross            float64 `json:"gross"`
	BillDate         string  `json:"bill_date"`
	DueDate          string  `json:"due_date"`
	Overdue          bool    `json:"overdue"`

	Raw json.RawMessage `json:"-"`
}

func (b *Bill) UnmarshalJSON(data []byte) error {
	type bill Bill
	var v bill
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = Bill(v)
	b.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// VendorName returns the vendor of a bill, falling back to the name parts
// (the list endpoint sends "vendor", the detail endpoints do not).
func (b Bill) VendorName() string {
	if b.Vendor != "" {
		return b.Vendor
	}
	if b.FirstnameSuffix != "" && b.LastnameCompany != "" {
		return b.FirstnameSuffix + " " + b.LastnameCompany
	}
	return b.LastnameCompany
}

// Amount returns the effective bill amount: amount_man when manual_amount is
// set, amount_calc otherwise.
func (b Bill) Amount() float64 {
	if b.ManualAmount {
		return b.AmountMan
	}
	return b.AmountCalc
}

// BillStatuses are the values of the read-only Bill "status" field.
var BillStatuses = []string{
	"DRAFT", "BOOKED", "PARTIALLY_CREATED", "CREATED", "PARTIALLY_SENT",
	"SENT", "PARTIALLY_DOWNLOADED", "DOWNLOADED", "PARTIALLY_PAID", "PAID",
	"PARTIALLY_FAILED", "FAILED",
}

// BillBookingStatuses are the statuses accepted by
// PUT /4.0/purchase/bills/{id}/bookings/{status}.
var BillBookingStatuses = []string{"DRAFT", "BOOKED"}

// BillListStatuses are the status buckets of the bill list filter.
var BillListStatuses = []string{"DRAFTS", "TODO", "PAID", "OVERDUE"}

// BillSearchFields are the fields the bill list search_term may target.
var BillSearchFields = []string{
	"firstname_suffix", "lastname_company", "vendor_ref", "currency_code",
	"document_no", "title",
}

// PurchaseActions are the actions accepted by the bill and expense actions
// endpoints.
var PurchaseActions = []string{"DUPLICATE"}

const billsPath = "/4.0/purchase/bills"

func billPath(id string) string { return billsPath + "/" + url.PathEscape(id) }

type billListResponse struct {
	Data   []Bill         `json:"data"`
	Paging PurchasePaging `json:"paging"`
}

// ListBills fetches GET /4.0/purchase/bills and unwraps the "data" envelope.
func (c *Client) ListBills(ctx context.Context, opts PurchaseListOptions) ([]Bill, error) {
	var out billListResponse
	return out.Data, c.Get(ctx, billsPath, opts.values(), &out)
}

func (c *Client) GetBill(ctx context.Context, id string) (*Bill, error) {
	var out Bill
	return &out, c.Get(ctx, billPath(id), nil, &out)
}

// CreateBill posts a new bill. fields uses the raw API field names.
func (c *Client) CreateBill(ctx context.Context, fields map[string]any) (*Bill, error) {
	var out Bill
	return &out, c.Do(ctx, http.MethodPost, billsPath, nil, fields, &out)
}

// UpdateBill replaces a bill (the 4.0 API updates with PUT and validates the
// full object, so fields must carry every required member).
func (c *Client) UpdateBill(ctx context.Context, id string, fields map[string]any) (*Bill, error) {
	var out Bill
	return &out, c.Do(ctx, http.MethodPut, billPath(id), nil, fields, &out)
}

// DeleteBill deletes a bill; the API answers 204 with an empty body.
func (c *Client) DeleteBill(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, billPath(id), nil, nil, nil)
}

// UpdateBillBookingStatus moves a bill between DRAFT and BOOKED.
func (c *Client) UpdateBillBookingStatus(ctx context.Context, id, status string) (*Bill, error) {
	var out Bill
	return &out, c.Do(ctx, http.MethodPut, billPath(id)+"/bookings/"+url.PathEscape(status), nil, nil, &out)
}

// BillAction executes an action (DUPLICATE) and returns the resulting bill.
func (c *Client) BillAction(ctx context.Context, id, action string) (*Bill, error) {
	var out Bill
	return &out, c.Do(ctx, http.MethodPost, billPath(id)+"/actions", nil, map[string]any{"action": action}, &out)
}

// CheckBillDocumentNumber validates a bill document number and reports the
// next free one.
func (c *Client) CheckBillDocumentNumber(ctx context.Context, documentNo string) (*DocumentNumberCheck, error) {
	q := url.Values{"document_no": {documentNo}}
	var out DocumentNumberCheck
	return &out, c.Get(ctx, "/4.0/purchase/documentnumbers/bills", q, &out)
}

// Expense is a purchase expense (4.0 resource "expenses").
type Expense struct {
	ID                  string  `json:"id"`
	DocumentNo          string  `json:"document_no"`
	Title               string  `json:"title"`
	Status              string  `json:"status"`
	FirstnameSuffix     string  `json:"firstname_suffix"`
	LastnameCompany     string  `json:"lastname_company"`
	Vendor              string  `json:"vendor"`
	CreatedAt           string  `json:"created_at"`
	PaidOn              string  `json:"paid_on"`
	SupplierID          int     `json:"supplier_id"`
	BankAccountID       int     `json:"bank_account_id"`
	BookingAccountID    int     `json:"booking_account_id"`
	CurrencyCode        string  `json:"currency_code"`
	BaseCurrencyCode    string  `json:"base_currency_code"`
	Amount              float64 `json:"amount"`
	Net                 float64 `json:"net"`
	Gross               float64 `json:"gross"`
	TaxID               int     `json:"tax_id"`
	ProjectID           string  `json:"project_id"`
	ChargeableContactID int     `json:"chargeable_contact_id"`
	TransactionID       string  `json:"transaction_id"`
	InvoiceID           string  `json:"invoice_id"`

	Raw json.RawMessage `json:"-"`
}

func (e *Expense) UnmarshalJSON(data []byte) error {
	type expense Expense
	var v expense
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = Expense(v)
	e.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// VendorName returns the vendor of an expense, falling back to the name
// parts (only the list endpoint sends "vendor").
func (e Expense) VendorName() string {
	if e.Vendor != "" {
		return e.Vendor
	}
	if e.FirstnameSuffix != "" && e.LastnameCompany != "" {
		return e.FirstnameSuffix + " " + e.LastnameCompany
	}
	return e.LastnameCompany
}

// ExpenseStatuses are the values of the read-only Expense "status" field;
// they double as the statuses accepted by the bookings endpoint.
var ExpenseStatuses = []string{"DRAFT", "DONE"}

const expensesPath = "/4.0/expenses"

func expensePath(id string) string { return expensesPath + "/" + url.PathEscape(id) }

type expenseListResponse struct {
	Data   []Expense      `json:"data"`
	Paging PurchasePaging `json:"paging"`
}

// ListExpenses fetches GET /4.0/expenses and unwraps the "data" envelope.
func (c *Client) ListExpenses(ctx context.Context, opts PurchaseListOptions) ([]Expense, error) {
	var out expenseListResponse
	return out.Data, c.Get(ctx, expensesPath, opts.values(), &out)
}

func (c *Client) GetExpense(ctx context.Context, id string) (*Expense, error) {
	var out Expense
	return &out, c.Get(ctx, expensePath(id), nil, &out)
}

func (c *Client) CreateExpense(ctx context.Context, fields map[string]any) (*Expense, error) {
	var out Expense
	return &out, c.Do(ctx, http.MethodPost, expensesPath, nil, fields, &out)
}

// UpdateExpense replaces an expense (PUT, full object).
func (c *Client) UpdateExpense(ctx context.Context, id string, fields map[string]any) (*Expense, error) {
	var out Expense
	return &out, c.Do(ctx, http.MethodPut, expensePath(id), nil, fields, &out)
}

// DeleteExpense deletes an expense; the API answers 204 with an empty body.
func (c *Client) DeleteExpense(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, expensePath(id), nil, nil, nil)
}

// UpdateExpenseBookingStatus moves an expense between DRAFT and DONE.
func (c *Client) UpdateExpenseBookingStatus(ctx context.Context, id, status string) (*Expense, error) {
	var out Expense
	return &out, c.Do(ctx, http.MethodPut, expensePath(id)+"/bookings/"+url.PathEscape(status), nil, nil, &out)
}

// ExpenseAction executes an action (DUPLICATE) and returns the resulting
// expense.
func (c *Client) ExpenseAction(ctx context.Context, id, action string) (*Expense, error) {
	var out Expense
	return &out, c.Do(ctx, http.MethodPost, expensePath(id)+"/actions", nil, map[string]any{"action": action}, &out)
}

// CheckExpenseDocumentNumber validates an expense document number and
// reports the next free one.
func (c *Client) CheckExpenseDocumentNumber(ctx context.Context, documentNo string) (*DocumentNumberCheck, error) {
	q := url.Values{"document_no": {documentNo}}
	var out DocumentNumberCheck
	return &out, c.Get(ctx, expensesPath+"/documentnumbers", q, &out)
}

// PurchaseOrder is a purchase order (3.0 resource "purchase_orders").
type PurchaseOrder struct {
	ID             int    `json:"id"`
	DocumentNr     string `json:"document_nr"`
	Title          string `json:"title"`
	ContactID      int    `json:"contact_id"`
	ContactSubID   int    `json:"contact_sub_id"`
	UserID         int    `json:"user_id"`
	ProjectID      int    `json:"project_id"`
	CurrencyID     int    `json:"currency_id"`
	LanguageID     int    `json:"language_id"`
	KbItemStatusID int    `json:"kb_item_status_id"`
	IsValidFrom    string `json:"is_valid_from"`
	IsValidTo      string `json:"is_valid_to"`
	Reference      string `json:"reference"`
	APIReference   string `json:"api_reference"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`

	Raw json.RawMessage `json:"-"`
}

func (p *PurchaseOrder) UnmarshalJSON(data []byte) error {
	type purchaseOrder PurchaseOrder
	var v purchaseOrder
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = PurchaseOrder(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// StatusName maps the read-only kb_item_status_id of a purchase order to its
// meaning. Purchase order status ids differ from every other kb document.
func (p PurchaseOrder) StatusName() string {
	switch p.KbItemStatusID {
	case 22:
		return "draft"
	case 23:
		return "open"
	case 24:
		return "partly"
	case 25:
		return "done"
	case 26:
		return "canceled"
	default:
		return fmt.Sprintf("status %d", p.KbItemStatusID)
	}
}

const purchaseOrdersPath = "/3.0/purchase_orders"

func purchaseOrderPath(id int) string { return fmt.Sprintf("%s/%d", purchaseOrdersPath, id) }

// ListPurchaseOrders fetches GET /3.0/purchase_orders (a bare array).
func (c *Client) ListPurchaseOrders(ctx context.Context, opts ListOptions) ([]PurchaseOrder, error) {
	var out []PurchaseOrder
	return out, c.Get(ctx, purchaseOrdersPath, opts.values(), &out)
}

func (c *Client) GetPurchaseOrder(ctx context.Context, id int) (*PurchaseOrder, error) {
	var out PurchaseOrder
	return &out, c.Get(ctx, purchaseOrderPath(id), nil, &out)
}

func (c *Client) CreatePurchaseOrder(ctx context.Context, fields map[string]any) (*PurchaseOrder, error) {
	var out PurchaseOrder
	return &out, c.Do(ctx, http.MethodPost, purchaseOrdersPath, nil, fields, &out)
}

// UpdatePurchaseOrder replaces a purchase order. Unlike the 2.0 kb documents
// this is a PUT, not a POST.
func (c *Client) UpdatePurchaseOrder(ctx context.Context, id int, fields map[string]any) (*PurchaseOrder, error) {
	var out PurchaseOrder
	return &out, c.Do(ctx, http.MethodPut, purchaseOrderPath(id), nil, fields, &out)
}

// DeletePurchaseOrder permanently deletes a purchase order.
func (c *Client) DeletePurchaseOrder(ctx context.Context, id int) error {
	var out Success
	if err := c.Do(ctx, http.MethodDelete, purchaseOrderPath(id), nil, nil, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("delete purchase order %d: API reported failure", id)
	}
	return nil
}

// OutgoingPayment is a payment on a bill (4.0 resource
// "purchase/outgoing-payments").
type OutgoingPayment struct {
	ID                  string  `json:"id"`
	BillID              string  `json:"bill_id"`
	Status              string  `json:"status"`
	PaymentType         string  `json:"payment_type"`
	CreatedAt           string  `json:"created_at"`
	ExecutionDate       string  `json:"execution_date"`
	Amount              float64 `json:"amount"`
	CurrencyCode        string  `json:"currency_code"`
	ExchangeRate        float64 `json:"exchange_rate"`
	Note                string  `json:"note"`
	SenderBankAccountID int     `json:"sender_bank_account_id"`
	ReceiverAccountNo   string  `json:"receiver_account_no"`
	ReceiverIBAN        string  `json:"receiver_iban"`
	ReceiverName        string  `json:"receiver_name"`
	FeeType             string  `json:"fee_type"`
	IsSalaryPayment     bool    `json:"is_salary_payment"`
	ReferenceNo         string  `json:"reference_no"`
	Message             string  `json:"message"`
	BankingPaymentID    string  `json:"banking_payment_id"`
	TransactionID       string  `json:"transaction_id"`

	Raw json.RawMessage `json:"-"`
}

func (p *OutgoingPayment) UnmarshalJSON(data []byte) error {
	type outgoingPayment OutgoingPayment
	var v outgoingPayment
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = OutgoingPayment(v)
	p.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// OutgoingPaymentStatuses are the values of the read-only status field.
var OutgoingPaymentStatuses = []string{
	"PENDING", "TRANSFERRED", "DOWNLOADED", "ERROR", "PAID", "DISCOUNTED",
}

// OutgoingPaymentTypes are the payment types accepted on create (the
// read-only RECONCILED type is only ever returned).
var OutgoingPaymentTypes = []string{"IBAN", "MANUAL", "CASH_DISCOUNT", "QR"}

// OutgoingPaymentFeeTypes are the values of fee_type.
var OutgoingPaymentFeeTypes = []string{"BY_SENDER", "BY_RECEIVER", "BREAKDOWN", "NO_FEE"}

const outgoingPaymentsPath = "/4.0/purchase/outgoing-payments"

func outgoingPaymentPath(id string) string {
	return outgoingPaymentsPath + "/" + url.PathEscape(id)
}

type outgoingPaymentListResponse struct {
	Data   []OutgoingPayment `json:"data"`
	Paging PurchasePaging    `json:"paging"`
}

// ListOutgoingPayments fetches the payments of one bill (bill_id is a
// required query parameter) and unwraps the "data" envelope.
func (c *Client) ListOutgoingPayments(ctx context.Context, billID string, opts PurchaseListOptions) ([]OutgoingPayment, error) {
	q := opts.values()
	q.Set("bill_id", billID)
	var out outgoingPaymentListResponse
	return out.Data, c.Get(ctx, outgoingPaymentsPath, q, &out)
}

func (c *Client) GetOutgoingPayment(ctx context.Context, id string) (*OutgoingPayment, error) {
	var out OutgoingPayment
	return &out, c.Get(ctx, outgoingPaymentPath(id), nil, &out)
}

func (c *Client) CreateOutgoingPayment(ctx context.Context, fields map[string]any) (*OutgoingPayment, error) {
	var out OutgoingPayment
	return &out, c.Do(ctx, http.MethodPost, outgoingPaymentsPath, nil, fields, &out)
}

// UpdateOutgoingPayment replaces an outgoing payment. Quirk: the PUT goes to
// the collection path — the payment is addressed by "payment_id" inside the
// body, not by a path segment.
func (c *Client) UpdateOutgoingPayment(ctx context.Context, fields map[string]any) (*OutgoingPayment, error) {
	var out OutgoingPayment
	return &out, c.Do(ctx, http.MethodPut, outgoingPaymentsPath, nil, fields, &out)
}

// DeleteOutgoingPayment deletes a payment; the API answers 204 with an empty
// body.
func (c *Client) DeleteOutgoingPayment(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, outgoingPaymentPath(id), nil, nil, nil)
}
