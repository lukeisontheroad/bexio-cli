package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const articleJSON = `{"id":4,"article_type_id":1,"intern_code":"A-1001","intern_name":"Schraube M4",` +
	`"sale_price":"0.250000","purchase_price":"0.100000","is_stock":true,"stock_id":1,` +
	`"stock_nr":100,"stock_available_nr":95}`

func TestArticleListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/article" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		w.Write([]byte("[" + articleJSON + "]"))
	}), "article", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Schraube M4") || !strings.Contains(out, "physical") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "95") { // stock_available_nr column
		t.Fatalf("missing stock column:\n%s", out)
	}
}

func TestArticleSearchBuildsCriteria(t *testing.T) {
	var body string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/article/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`[]`))
	}), "article", "search", "Schraube")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `{"field":"intern_name","value":"%Schraube%","criteria":"like"}`) {
		t.Fatalf("missing term clause: %s", body)
	}
}

func TestArticleCreatePayload(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/article":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(articleJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "article", "create", "--intern-name", "Schraube M4", "--intern-code", "A-1001", "--sale-price", "0.25")
	if err != nil {
		t.Fatal(err)
	}
	if created["intern_name"] != "Schraube M4" || created["intern_code"] != "A-1001" {
		t.Fatalf("payload = %v", created)
	}
	if created["sale_price"] != "0.25" {
		t.Fatalf("sale_price = %v", created["sale_price"])
	}
	if created["article_type_id"] != float64(1) {
		t.Fatalf("article_type_id default missing: %v", created)
	}
	if created["user_id"] != float64(4) {
		t.Fatalf("user_id default missing: %v", created)
	}
	if !strings.Contains(out, "Created article 4") {
		t.Fatalf("output: %s", out)
	}
}

func TestArticleUpdatePartial(t *testing.T) {
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/article/4" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(articleJSON))
	}), "article", "update", "4", "--sale-price", "0.30")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["sale_price"] != "0.30" {
		t.Fatalf("expected only sale_price in payload, got %v", body)
	}
}

func TestStockListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/stock" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":1,"name":"Hauptlager"}]`))
	}), "stock", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hauptlager") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestStockAreaListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/stock_place" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":2,"name":"Regal B"}]`))
	}), "stock-area", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Regal B") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestDeliveryListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_delivery" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":5,"document_nr":"LS-00005","title":"Lieferung ACME",` +
			`"contact_id":17,"total":"250.00","kb_item_status_id":10,"updated_at":"2026-08-01 10:30:00"}]`))
	}), "kb-delivery", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "LS-00005") || !strings.Contains(out, "draft") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestDeliveryIssue(t *testing.T) {
	var hit bool
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_delivery/5/issue" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		hit = true
		w.Write([]byte(`{"success":true}`))
	}), "delivery", "issue", "5")
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("issue endpoint not hit")
	}
	if !strings.Contains(out, "Issued delivery 5") {
		t.Fatalf("output: %s", out)
	}
}
