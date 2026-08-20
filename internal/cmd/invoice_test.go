package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const invoiceJSON = `{"id":4,"document_nr":"RE-00004","title":"Website relaunch",` +
	`"contact_id":17,"user_id":1,"kb_item_status_id":8,"total":"1500.00",` +
	`"total_remaining_payments":"500.00","updated_at":"2026-08-01 10:30:00"}`

func TestInvoiceListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_invoice" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte("[" + invoiceJSON + "]"))
	}), "kb-invoice", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "RE-00004") || !strings.Contains(out, "pending") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "500.00") {
		t.Fatalf("missing remaining amount:\n%s", out)
	}
}

func TestInvoiceViewJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_invoice/4" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(invoiceJSON))
	}), "invoice", "view", "4", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if m["document_nr"] != "RE-00004" {
		t.Fatalf("got %v", m)
	}
}

func TestInvoiceCreateDefaultsUserID(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/kb_invoice":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(invoiceJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "kb-invoice", "create", "--contact-id", "17", "--title", "Website relaunch",
		"--position", "type=custom,text=Consulting,amount=8,unit_price=150")
	if err != nil {
		t.Fatal(err)
	}
	if created["contact_id"] != float64(17) {
		t.Fatalf("contact_id = %v", created["contact_id"])
	}
	if created["user_id"] != float64(4) {
		t.Fatalf("user_id default missing: %v", created)
	}
	positions, ok := created["positions"].([]any)
	if !ok || len(positions) != 1 {
		t.Fatalf("positions = %v", created["positions"])
	}
	pos := positions[0].(map[string]any)
	if pos["type"] != "KbPositionCustom" || pos["text"] != "Consulting" {
		t.Fatalf("position = %v", pos)
	}
	if !strings.Contains(out, "Created invoice 4") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestInvoicePaymentCreate(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_invoice/4/payment" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &created); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"id":9,"date":"2026-08-21","value":"150.00","kb_invoice_id":4}`))
	}), "kb-invoice", "payment", "create", "4", "--value", "150.00", "--date", "2026-08-21", "--bank-account-id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if created["value"] != "150.00" || created["date"] != "2026-08-21" || created["bank_account_id"] != float64(1) {
		t.Fatalf("payload = %v", created)
	}
	if !strings.Contains(out, "Recorded payment 9") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestInvoiceDeleteRequiresForce(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "kb-invoice", "delete", "4")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}

	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_invoice/4" || r.Method != http.MethodDelete {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"success":true}`))
	}), "kb-invoice", "delete", "4", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Deleted invoice 4") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}
