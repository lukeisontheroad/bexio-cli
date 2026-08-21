package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const billJSON = `{"id":"7572f70e-6bf5-47be-9a28-466423d8e3b1","document_no":"NO-1",` +
	`"status":"DRAFT","title":"Hosting","firstname_suffix":"John","lastname_company":"Doe",` +
	`"vendor":"John Doe","vendor_ref":"V-1","created_at":"2026-03-23T09:53:49+0000",` +
	`"currency_code":"CHF","pending_amount":100.23,"net":110,"gross":120.5,` +
	`"bill_date":"2026-02-12","due_date":"2026-03-14","overdue":false,` +
	`"booking_account_ids":[10],"attachment_ids":[]}`

// billDetailJSON is what the single-fetch endpoints return: no "vendor" or
// "gross", but the nested address/line_items/discounts and read-only members
// that must not be echoed back on PUT.
const billDetailJSON = `{"id":"7572f70e-6bf5-47be-9a28-466423d8e3b1","document_no":"NO-1",` +
	`"status":"DRAFT","title":"Hosting","lastname_company":"Doe","created_at":"2026-03-23T09:53:49+0000",` +
	`"supplier_id":17,"contact_partner_id":17,"manual_amount":false,"amount_calc":120.5,` +
	`"pending_amount":120.5,"bill_date":"2026-02-12","due_date":"2026-03-14","currency_code":"CHF",` +
	`"base_currency_code":"CHF","item_net":false,"split_into_line_items":false,"overdue":false,` +
	`"address":{"lastname_company":"bexio AG","type":"COMPANY","city":"Rapperswil"},` +
	`"line_items":[{"id":"8b102a32-5bef-462e-a41b-9c00197c26b9","position":1,"title":"Hosting",` +
	`"tax_id":15,"tax_calc":12.89,"amount":120.5,"booking_account_id":16}],` +
	`"discounts":[],"attachment_ids":[]}`

func TestBillListTableUnwrapsEnvelope(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/purchase/bills" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("status") != "TODO" || q.Get("limit") != "100" {
			t.Errorf("query = %v", q)
		}
		if got := q["fields[]"]; len(got) != 1 || got[0] != "title" {
			t.Errorf("fields[] = %v", got)
		}
		w.Write([]byte(`{"data":[` + billJSON + `],"paging":{"page":1,"page_size":10,"page_count":1,"item_count":1}}`))
	}), "purchase-bill", "list", "--status", "TODO", "--search-term", "host", "--field", "title")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "NO-1") || !strings.Contains(out, "DRAFT") || !strings.Contains(out, "John Doe") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "120.50") {
		t.Fatalf("gross not rendered:\n%s", out)
	}
}

func TestBillViewJSONRaw(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/purchase/bills/7572f70e-6bf5-47be-9a28-466423d8e3b1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(billDetailJSON))
	}), "purchase-bill", "view", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	// -o json must pass the raw API object through, nested members included.
	items, ok := m["line_items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("line_items missing: %v", m)
	}
	if items[0].(map[string]any)["tax_calc"] != 12.89 {
		t.Fatalf("raw passthrough lost fields: %v", items[0])
	}
}

func TestBillCreatePayload(t *testing.T) {
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/4.0/purchase/bills" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(billDetailJSON))
	}),
		"purchase-bill", "create",
		"--supplier-id", "17", "--contact-partner-id", "18",
		"--currency-code", "CHF", "--bill-date", "2026-08-01", "--due-date", "2026-08-31",
		"--address-lastname-company", "bexio AG", "--address-type", "company",
		"--address-city", "Rapperswil",
		"--line-item", "position=1,title=Hosting,amount=120.5,tax_id=15,booking_account_id=16",
	)
	if err != nil {
		t.Fatal(err)
	}
	if body["supplier_id"] != float64(17) || body["contact_partner_id"] != float64(18) {
		t.Fatalf("ids missing: %v", body)
	}
	if body["manual_amount"] != false || body["item_net"] != false {
		t.Fatalf("required booleans missing: %v", body)
	}
	if _, ok := body["discounts"].([]any); !ok {
		t.Fatalf("discounts must be sent even when empty: %v", body["discounts"])
	}
	addr, _ := body["address"].(map[string]any)
	if addr["lastname_company"] != "bexio AG" || addr["type"] != "COMPANY" || addr["city"] != "Rapperswil" {
		t.Fatalf("address = %v", addr)
	}
	items, _ := body["line_items"].([]any)
	if len(items) != 1 {
		t.Fatalf("line_items = %v", body["line_items"])
	}
	li := items[0].(map[string]any)
	if li["position"] != float64(1) || li["amount"] != 120.5 || li["tax_id"] != float64(15) || li["title"] != "Hosting" {
		t.Fatalf("line item = %v", li)
	}
	if !strings.Contains(out, "Created bill 7572f70e-6bf5-47be-9a28-466423d8e3b1") {
		t.Fatalf("output: %s", out)
	}
}

func TestBillCreateRequiresLineItem(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}),
		"purchase-bill", "create",
		"--supplier-id", "17", "--contact-partner-id", "18",
		"--currency-code", "CHF", "--bill-date", "2026-08-01", "--due-date", "2026-08-31",
		"--address-lastname-company", "bexio AG", "--address-type", "COMPANY",
	)
	if err == nil || !strings.Contains(err.Error(), "--line-item") {
		t.Fatalf("err = %v", err)
	}
}

// TestBillUpdatePutsMergedObject covers the 4.0 update flow: read the bill,
// re-send the writable members with the change applied, via PUT.
func TestBillUpdatePutsMergedObject(t *testing.T) {
	var method, path string
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(billDetailJSON))
			return
		}
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(billDetailJSON))
	}), "purchase-bill", "update", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "--title", "Hosting 2026")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/4.0/purchase/bills/7572f70e-6bf5-47be-9a28-466423d8e3b1" {
		t.Fatalf("%s %s", method, path)
	}
	if body["title"] != "Hosting 2026" {
		t.Fatalf("title not applied: %v", body["title"])
	}
	// Untouched members are re-sent...
	if body["supplier_id"] != float64(17) || body["bill_date"] != "2026-02-12" {
		t.Fatalf("merge lost fields: %v", body)
	}
	if body["manual_amount"] != false || body["item_net"] != false {
		t.Fatalf("required booleans lost: %v", body)
	}
	// ...but the read-only ones the PUT schema rejects are dropped.
	for _, k := range []string{"status", "created_at", "pending_amount", "base_currency_code", "overdue", "id"} {
		if _, ok := body[k]; ok {
			t.Fatalf("read-only field %q was echoed back: %v", k, body)
		}
	}
	items, _ := body["line_items"].([]any)
	if len(items) != 1 {
		t.Fatalf("line_items = %v", body["line_items"])
	}
	if _, ok := items[0].(map[string]any)["tax_calc"]; ok {
		t.Fatalf("read-only tax_calc was echoed back: %v", items[0])
	}
	if !strings.Contains(out, "Updated bill") {
		t.Fatalf("output: %s", out)
	}
}

func TestBillUpdateNothing(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "purchase-bill", "update", "7572f70e-6bf5-47be-9a28-466423d8e3b1")
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("err = %v", err)
	}
}

// TestBillBookTransition covers PUT .../bookings/{status}.
func TestBillBookTransition(t *testing.T) {
	var method, path string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Write([]byte(strings.Replace(billDetailJSON, `"status":"DRAFT"`, `"status":"BOOKED"`, 1)))
	}), "purchase-bill", "book", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "booked")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s", method)
	}
	if path != "/4.0/purchase/bills/7572f70e-6bf5-47be-9a28-466423d8e3b1/bookings/BOOKED" {
		t.Fatalf("path = %q", path)
	}
	if !strings.Contains(out, "is now BOOKED") {
		t.Fatalf("output: %s", out)
	}
}

func TestBillBookRejectsUnknownStatus(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "purchase-bill", "book", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "PAID")
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("err = %v", err)
	}
}

func TestBillActionPostsBody(t *testing.T) {
	var body map[string]any
	var path string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(billDetailJSON))
	}), "purchase-bill", "action", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "duplicate")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/4.0/purchase/bills/7572f70e-6bf5-47be-9a28-466423d8e3b1/actions" {
		t.Fatalf("path = %q", path)
	}
	if body["action"] != "DUPLICATE" {
		t.Fatalf("body = %v", body)
	}
}

func TestBillDocumentNumberCheck(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/purchase/documentnumbers/bills" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("document_no") != "AB-1234" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Write([]byte(`{"valid":false,"next_available_no":"AB-1235"}`))
	}), "purchase-bill", "document-number", "check", "AB-1234")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "false") || !strings.Contains(out, "AB-1235") {
		t.Fatalf("output: %s", out)
	}
}

const expenseJSON = `{"id":"1355499f-aa07-4382-887e-acaf0323e6f6","document_no":"EX-1",` +
	`"status":"DRAFT","title":"Office supplies","firstname_suffix":"John","lastname_company":"Doe",` +
	`"vendor":"John Doe","created_at":"2026-03-23T09:53:49+0000","paid_on":"2026-03-20",` +
	`"booking_account_id":4,"currency_code":"CHF","net":26.65,"gross":29.43,"attachment_ids":[]}`

func TestExpenseListTableUnwrapsEnvelope(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/expenses" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("paid_on_start") != "2026-01-01" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Write([]byte(`{"data":[` + expenseJSON + `],"paging":{"page":1,"page_size":10,"page_count":1,"item_count":1}}`))
	}), "expense", "list", "--paid-on-start", "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "EX-1") || !strings.Contains(out, "29.43") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestExpenseCreatePayload(t *testing.T) {
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/4.0/expenses" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(expenseJSON))
	}), "expense", "create", "--paid-on", "2026-08-01", "--currency-code", "CHF",
		"--amount", "30.90", "--booking-account-id", "4", "--title", "Office supplies")
	if err != nil {
		t.Fatal(err)
	}
	if body["paid_on"] != "2026-08-01" || body["amount"] != 30.9 || body["booking_account_id"] != float64(4) {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["attachment_ids"].([]any); !ok {
		t.Fatalf("attachment_ids must be sent even when empty: %v", body["attachment_ids"])
	}
}

func TestExpenseBookTransition(t *testing.T) {
	var method, path string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Write([]byte(strings.Replace(expenseJSON, `"status":"DRAFT"`, `"status":"DONE"`, 1)))
	}), "expense", "book", "1355499f-aa07-4382-887e-acaf0323e6f6", "DONE")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/4.0/expenses/1355499f-aa07-4382-887e-acaf0323e6f6/bookings/DONE" {
		t.Fatalf("%s %s", method, path)
	}
}

const purchaseOrderJSON = `{"id":4,"document_nr":"BE-00004","title":"Hardware",` +
	`"contact_id":14,"user_id":1,"kb_item_status_id":23,"currency_id":1,` +
	`"is_valid_from":"2026-08-01","is_valid_to":"2026-08-31","total_rounding_difference":0,` +
	`"viewed_by_client_at":null,"updated_at":"2026-08-02 10:30:00"}`

func TestPurchaseOrderListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/purchase_orders" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte("[" + purchaseOrderJSON + "]"))
	}), "purchase-order", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "BE-00004") || !strings.Contains(out, "open") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// TestPurchaseOrderUpdateUsesPut covers the 3.0 quirk: purchase orders are
// updated with PUT, not with the POST used by the 2.0 kb documents.
func TestPurchaseOrderUpdateUsesPut(t *testing.T) {
	var method, path string
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(purchaseOrderJSON))
			return
		}
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(purchaseOrderJSON))
	}), "purchase-order", "update", "4", "--title", "Hardware 2026")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/3.0/purchase_orders/4" {
		t.Fatalf("%s %s", method, path)
	}
	if body["title"] != "Hardware 2026" || body["contact_id"] != float64(14) {
		t.Fatalf("body = %v", body)
	}
	for _, k := range []string{"id", "kb_item_status_id", "total_rounding_difference", "updated_at"} {
		if _, ok := body[k]; ok {
			t.Fatalf("read-only field %q was echoed back: %v", k, body)
		}
	}
	if !strings.Contains(out, "Updated purchase order 4") {
		t.Fatalf("output: %s", out)
	}
}

const outgoingPaymentJSON = `{"id":"46913fdc-802b-49ba-99d7-4ccc13cccfc2",` +
	`"bill_id":"176a1442-d66d-4907-b8c8-6dad090452a8","payment_type":"IBAN",` +
	`"status":"PENDING","created_at":"2026-06-27T10:25:50+0200","execution_date":"2026-10-15",` +
	`"amount":45.98,"currency_code":"CHF","exchange_rate":1,"sender_bank_account_id":4,` +
	`"receiver_iban":"CH121234567812345678900","is_salary_payment":false}`

func TestOutgoingPaymentListRequiresBillID(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/purchase/outgoing-payments" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("bill_id") != "176a1442-d66d-4907-b8c8-6dad090452a8" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Write([]byte(`{"data":[` + outgoingPaymentJSON + `],"paging":{"page":1,"page_size":10,"page_count":1,"item_count":1}}`))
	}), "outgoing-payment", "list", "--bill-id", "176a1442-d66d-4907-b8c8-6dad090452a8")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "PENDING") || !strings.Contains(out, "45.98") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// TestOutgoingPaymentUpdatePutsCollection covers the quirk that the update
// PUT goes to the collection path with payment_id in the body.
func TestOutgoingPaymentUpdatePutsCollection(t *testing.T) {
	var method, path string
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(outgoingPaymentJSON))
			return
		}
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(outgoingPaymentJSON))
	}), "outgoing-payment", "update", "46913fdc-802b-49ba-99d7-4ccc13cccfc2", "--execution-date", "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/4.0/purchase/outgoing-payments" {
		t.Fatalf("%s %s", method, path)
	}
	if body["payment_id"] != "46913fdc-802b-49ba-99d7-4ccc13cccfc2" {
		t.Fatalf("payment_id missing: %v", body)
	}
	if body["execution_date"] != "2026-11-01" || body["amount"] != 45.98 {
		t.Fatalf("body = %v", body)
	}
	if body["is_salary_payment"] != false {
		t.Fatalf("required member lost: %v", body)
	}
}

func TestOutgoingPaymentDeleteNeedsForce(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "outgoing-payment", "delete", "46913fdc-802b-49ba-99d7-4ccc13cccfc2")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v", err)
	}
}

// TestBillDeleteHandlesEmpty204 covers the 4.0 delete response: 204 with an
// empty body instead of the 2.0 {"success": true}.
func TestBillDeleteHandlesEmpty204(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "purchase-bill", "delete", "7572f70e-6bf5-47be-9a28-466423d8e3b1", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Deleted bill") {
		t.Fatalf("output: %s", out)
	}
}
