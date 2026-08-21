package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const manualEntryJSON = `{"id":42,"type":"manual_single_entry","date":"2026-01-31",` +
	`"reference_nr":"BA-22","is_locked":false,"entries":[{"id":32,"debit_account_id":77,` +
	`"credit_account_id":139,"amount":328.25,"description":"Rent","currency_id":1}]}`

func TestManualEntryCreateNestedEntries(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/manual_entries" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &created); err != nil {
			t.Error(err)
		}
		w.Write([]byte(manualEntryJSON))
	}), "manual-entry", "create", "--date", "2026-01-31", "--reference-nr", "BA-22",
		"--entry", "debit_account_id=77,credit_account_id=139,amount=328.25,description=Rent",
		"--entry", "debit_account_id=78,credit_account_id=140,amount=10,tax_id=3")
	if err != nil {
		t.Fatal(err)
	}
	if created["type"] != "manual_single_entry" || created["date"] != "2026-01-31" || created["reference_nr"] != "BA-22" {
		t.Fatalf("payload = %v", created)
	}
	entries, ok := created["entries"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("entries = %v", created["entries"])
	}
	first, _ := entries[0].(map[string]any)
	if first["debit_account_id"] != float64(77) || first["credit_account_id"] != float64(139) {
		t.Fatalf("first entry = %v", first)
	}
	if first["amount"] != 328.25 || first["description"] != "Rent" {
		t.Fatalf("first entry = %v", first)
	}
	second, _ := entries[1].(map[string]any)
	if second["tax_id"] != float64(3) {
		t.Fatalf("second entry = %v", second)
	}
	if !strings.Contains(out, "Created manual entry 42") {
		t.Fatalf("output: %s", out)
	}
}

func TestManualEntryUpdateUsesPUT(t *testing.T) {
	var method, path string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Write([]byte(manualEntryJSON))
	}), "manual-entry", "update", "42", "--type", "manual_single_entry", "--date", "2026-02-01",
		"--entry", "id=32,debit_account_id=77,credit_account_id=139,amount=400")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/3.0/accounting/manual_entries/42" {
		t.Fatalf("%s %s", method, path)
	}
}

func TestManualEntryUpdateRequiresEntries(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "manual-entry", "update", "42", "--date", "2026-02-01")
	if err == nil || !strings.Contains(err.Error(), "replaces the whole entry") {
		t.Fatalf("err = %v", err)
	}
}

func TestManualEntryEntriesJSONEscape(t *testing.T) {
	var created map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &created); err != nil {
			t.Error(err)
		}
		w.Write([]byte(manualEntryJSON))
	}), "manual-entry", "create", "--date", "2026-01-31",
		"--entries-json", `[{"debit_account_id":77,"credit_account_id":139,"amount":50,"description":"a, b"}]`)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := created["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %v", created["entries"])
	}
	if first, _ := entries[0].(map[string]any); first["description"] != "a, b" {
		t.Fatalf("first entry = %v", entries[0])
	}
}

func TestManualEntryFileListPerLine(t *testing.T) {
	var path string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`[{"id":7,"name":"receipt","extension":"pdf","mime_type":"application/pdf","size_in_bytes":1234,"created_at":"2026-01-31T08:52:10+00:00"}]`))
	}), "manual-entry", "file", "list", "42", "--entry-id", "32")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/3.0/accounting/manual_entries/42/entries/32/files" {
		t.Fatalf("path = %q", path)
	}
	if !strings.Contains(out, "receipt") || !strings.Contains(out, "1234") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestManualEntryFileAttachMultipart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	var reqPath, contentType, body string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath, contentType = r.URL.Path, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`[{"id":7,"name":"receipt.pdf"}]`))
	}), "manual-entry", "file", "attach", "42", path)
	if err != nil {
		t.Fatal(err)
	}
	if reqPath != "/3.0/accounting/manual_entries/42/files" {
		t.Fatalf("path = %q", reqPath)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("content-type = %q", contentType)
	}
	if !strings.Contains(body, `name="fileName"`) || !strings.Contains(body, "%PDF-1.4 fake") {
		t.Fatalf("body:\n%s", body)
	}
	if !strings.Contains(out, "Attached file 7") {
		t.Fatalf("output: %s", out)
	}
}

func TestManualEntryViewScansList(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/manual_entries" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte("[" + manualEntryJSON + "]"))
	}), "manual-entry", "view", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "BA-22") || !strings.Contains(out, "manual_single_entry") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestManualEntryNextRefNr(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/manual_entries/next_ref_nr" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"next_ref_nr":"MB-23"}`))
	}), "manual-entry", "next-ref-nr")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "MB-23" {
		t.Fatalf("output = %q", out)
	}
}

func TestAccountListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit = %q", got)
		}
		w.Write([]byte(`[{"id":90,"account_no":"1020","name":"Bank","account_type":3,` +
			`"fibu_account_group_id":4,"is_active":true,"is_locked":false}]`))
	}), "account", "list", "--limit", "50")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1020") || !strings.Contains(out, "Bank") || !strings.Contains(out, "active") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestJournalListParams(t *testing.T) {
	var query string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/journal" {
			t.Errorf("path = %q", r.URL.Path)
		}
		query = r.URL.RawQuery
		w.Write([]byte(`[{"id":5,"date":"2026-02-01","debit_account_id":77,"credit_account_id":139,` +
			`"amount":328.25,"currency_id":1,"ref_class":"AccountingJournal\\Entry","description":"Rent"}]`))
	}), "journal", "list", "--from", "2026-01-01", "--to", "2026-03-31",
		"--account-uuid", "474cc93a", "--limit", "10", "--offset", "20")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"from=2026-01-01", "to=2026-03-31", "account_uuid=474cc93a", "limit=10", "offset=20"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q missing %q", query, want)
		}
	}
	if !strings.Contains(out, "328.25") || !strings.Contains(out, "Rent") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestCalendarYearCreatePayload(t *testing.T) {
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/calendar_years" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`[{"id":3,"start":"2027-01-01","end":"2027-12-31","is_vat_subject":true,` +
			`"vat_accounting_method":"effective","vat_accounting_type":"agreed"}]`))
	}), "calendar-year", "create", "--year", "2027", "--is-vat-subject",
		"--vat-accounting-method", "effective", "--vat-accounting-type", "agreed",
		"--default-tax-income-id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if body["year"] != "2027" || body["is_vat_subject"] != true {
		t.Fatalf("payload = %v", body)
	}
	if body["vat_accounting_method"] != "effective" || body["vat_accounting_type"] != "agreed" {
		t.Fatalf("payload = %v", body)
	}
	if body["default_tax_income_id"] != float64(1) {
		t.Fatalf("payload = %v", body)
	}
	if _, ok := body["default_tax_expense_id"]; ok {
		t.Fatalf("unset flag leaked into payload: %v", body)
	}
	if !strings.Contains(out, "Created calendar year 3") {
		t.Fatalf("output: %s", out)
	}
}

func TestVatPeriodViewDetail(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/accounting/vat_periods/2" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"id":2,"start":"2026-01-01","end":"2026-03-31","type":"quarter","status":"open"}`))
	}), "vat-period", "view", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "quarter") || !strings.Contains(out, "open") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestParseManualEntryLineSpec(t *testing.T) {
	fields, err := parseManualEntryLineSpec("debit_account_id=77,amount=12.5,description=Rent,currency_factor=1")
	if err != nil {
		t.Fatal(err)
	}
	if fields["debit_account_id"] != 77 || fields["amount"] != 12.5 {
		t.Fatalf("fields = %v", fields)
	}
	if fields["description"] != "Rent" || fields["currency_factor"] != float64(1) {
		t.Fatalf("fields = %v", fields)
	}
	if _, err := parseManualEntryLineSpec("debit_account_id=abc"); err == nil {
		t.Fatal("expected error for non-numeric id")
	}
	if _, err := parseManualEntryLineSpec("nokeyvalue"); err == nil {
		t.Fatal("expected error for malformed spec")
	}
}
