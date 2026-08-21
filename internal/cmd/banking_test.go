package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const bankingPaymentUUID = "0c295adb-91ff-4cd5-8a8c-009ee4330f69"

const bankingPaymentJSON = `{"id":12,"uuid":"` + bankingPaymentUUID + `",` +
	`"sender":{"id":1,"uuid":"7a1c1d3e-0000-4000-8000-000000000001","iban":"CH9300762011623852957"},` +
	`"recipient":{"name":"Bexio AG","iban":"CH3000784295116252003",` +
	`"address":{"street_name":"Föhrenstrasse","house_number":"34","zip":"5003","city":"Zürich","country_code":"CH"}},` +
	`"amount":1250.00,"currency":"CHF","execution_date":"2026-09-01","allowance":"fee_split",` +
	`"is_salary":false,"status":"open","type":"iban","created_at":"2026-08-21T10:30:00+02:00"}`

func TestBankingPaymentListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.URL.Query().Get("filter-by"); got != "status:open;currency:CHF" {
			t.Errorf("filter-by = %q", got)
		}
		if got := r.URL.Query().Get("per-page"); got != "20" {
			t.Errorf("per-page = %q", got)
		}
		w.Write([]byte("[" + bankingPaymentJSON + "]"))
	}), "banking-payment", "list", "--filter-by", "status:open", "--filter-by", "currency:CHF", "--per-page", "20")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{bankingPaymentUUID, "open", "1250", "CHF", "Bexio AG", "CH3000784295116252003", "2026-09-01"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

// The 4.0 list endpoints ship a {"data": [...], "paging": {...}} envelope
// on some resources; the payment list must decode that too, Raw intact.
func TestBankingPaymentListEnvelopeJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[` + bankingPaymentJSON + `],"paging":{"page":0,"page_size":500,"total":1}}`))
	}), "banking-payment", "list", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["uuid"] != bankingPaymentUUID {
		t.Fatalf("got %v", got)
	}
	if got[0]["sender"] == nil {
		t.Fatalf("raw API object not preserved: %v", got[0])
	}
}

func TestBankingPaymentViewJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments/"+bankingPaymentUUID {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(bankingPaymentJSON))
	}), "banking-payment", "view", bankingPaymentUUID, "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if m["status"] != "open" {
		t.Fatalf("got %v", m)
	}
}

func bankingPaymentCreateArgs(extra ...string) []string {
	args := []string{
		"banking-payment", "create",
		"--type", "iban",
		"--account-id", "7a1c1d3e-0000-4000-8000-000000000001",
		"--amount", "1250.00",
		"--currency", "CHF",
		"--execution-date", "2026-09-01",
		"--recipient-name", "Bexio AG",
		"--recipient-iban", "CH3000784295116252003",
		"--recipient-address-street-name", "Föhrenstrasse",
		"--recipient-address-house-number", "34",
		"--recipient-address-zip", "5003",
		"--recipient-address-city", "Zürich",
	}
	return append(args, extra...)
}

func TestBankingPaymentCreateRequiresForce(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), bankingPaymentCreateArgs()...)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}
}

func TestBankingPaymentCreateSendsPayload(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &created); err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(b), `"amount":1250.00`) {
			t.Errorf("amount not sent as a bare decimal: %s", b)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(bankingPaymentJSON))
	}), bankingPaymentCreateArgs("--force")...)
	if err != nil {
		t.Fatal(err)
	}
	if created["type"] != "iban" || created["currency"] != "CHF" || created["execution_date"] != "2026-09-01" {
		t.Fatalf("payload = %v", created)
	}
	if created["account_id"] != "7a1c1d3e-0000-4000-8000-000000000001" {
		t.Fatalf("account_id = %v", created["account_id"])
	}
	if created["is_salary"] != false {
		t.Fatalf("is_salary default missing: %v", created["is_salary"])
	}
	recipient, ok := created["recipient"].(map[string]any)
	if !ok || recipient["name"] != "Bexio AG" || recipient["iban"] != "CH3000784295116252003" {
		t.Fatalf("recipient = %v", created["recipient"])
	}
	address, ok := recipient["address"].(map[string]any)
	if !ok || address["zip"] != "5003" || address["city"] != "Zürich" {
		t.Fatalf("recipient address = %v", recipient["address"])
	}
	if !strings.Contains(out, "Created payment "+bankingPaymentUUID) || !strings.Contains(out, "Bexio AG") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBankingPaymentCreateRejectsIncompletePayload(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "banking-payment", "create", "--force", "--type", "qr", "--account-id", "acc",
		"--amount", "10", "--currency", "CHF", "--execution-date", "2026-09-01",
		"--recipient-name", "X", "--recipient-iban", "CH3000784295116252003")
	if err == nil || !strings.Contains(err.Error(), "--qr-reference-number") {
		t.Fatalf("expected qr reference error, got %v", err)
	}
	if called {
		t.Fatal("API was called with an incomplete payload")
	}
}

func TestBankingPaymentUpdateUsesPUT(t *testing.T) {
	var sent map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments/"+bankingPaymentUUID {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("method = %q (want PUT)", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &sent); err != nil {
			t.Error(err)
		}
		w.Write([]byte(bankingPaymentJSON))
	}), "banking-payment", "update", bankingPaymentUUID, "--force",
		"--amount", "990.50", "--execution-date", "2026-09-15")
	if err != nil {
		t.Fatal(err)
	}
	if sent["amount"] != 990.50 || sent["execution_date"] != "2026-09-15" {
		t.Fatalf("payload = %v", sent)
	}
	if _, ok := sent["currency"]; ok {
		t.Fatalf("unchanged flags must not be sent: %v", sent)
	}
	if !strings.Contains(out, "Updated payment "+bankingPaymentUUID) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBankingPaymentUpdateRequiresForce(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "banking-payment", "update", bankingPaymentUUID, "--amount", "990.50")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}
}

func TestBankingPaymentDeleteRequiresForce(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "banking-payment", "delete", bankingPaymentUUID)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}

	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments/"+bankingPaymentUUID || r.Method != http.MethodDelete {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"success":true}`))
	}), "banking-payment", "delete", bankingPaymentUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Deleted payment "+bankingPaymentUUID) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBankingPaymentCancel(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "banking-payment", "cancel", bankingPaymentUUID)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force error, got %v", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}

	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/banking/payments/"+bankingPaymentUUID+"/cancel" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if b, _ := io.ReadAll(r.Body); len(b) != 0 {
			t.Errorf("cancel must not send a body, got %q", b)
		}
		w.Write([]byte(strings.Replace(bankingPaymentJSON, `"status":"open"`, `"status":"cancelled"`, 1)))
	}), "banking-payment", "cancel", bankingPaymentUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cancelled payment "+bankingPaymentUUID) || !strings.Contains(out, "cancelled") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBankingPaymentCreateBlockedByReadOnly(t *testing.T) {
	t.Setenv("BEXIO_READ_ONLY", "1")
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), bankingPaymentCreateArgs("--force")...)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
	if called {
		t.Fatal("API was called in read-only mode")
	}
}
