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

const payrollEmployeeID = "309bf968-ea25-4819-8f2e-ca08aa369690"

const payrollEmployeeJSON = `{"id":"309bf968-ea25-4819-8f2e-ca08aa369690","first_name":"Anna",` +
	`"last_name":"Meyer","personal_number":"E-42","email":"anna@example.com","employment_level":80,` +
	`"annual_vacation_days_total":25,"nationality":"CH","language":"de","marital_status":"single",` +
	`"address":{"street_name":"Bahnhofstrasse","house_number":"1","zip_code":"8001","city":"Zürich"}}`

const payrollAbsenceJSON = `{"id":"7c1f0f1a-6f0a-4f2e-9d1a-2a0d5c9a1b23","reason":"Vacation",` +
	`"start_date":"2026-07-06","end_date":"2026-07-17","half_day":false,"paid_hours":8.5,` +
	`"continued_pay":100,"disability":0}`

// The 4.0 payroll list endpoints wrap their payload in a {"data": [...]}
// envelope instead of returning a bare array.
func TestEmployeeListTableDecodesEnvelope(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/payroll/employees" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"data":[` + payrollEmployeeJSON + `]}`))
	}), "payroll-employee", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Anna Meyer") || !strings.Contains(out, payrollEmployeeID) {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "80") || !strings.Contains(out, "E-42") {
		t.Fatalf("employment level / personal number missing:\n%s", out)
	}
}

func TestEmployeeViewSendsDate(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/payroll/employees/"+payrollEmployeeID {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("date"); got != "2026-01-31" {
			t.Errorf("date = %q", got)
		}
		w.Write([]byte(payrollEmployeeJSON))
	}), "payroll-employee", "view", payrollEmployeeID, "--date", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Meyer") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// The payroll employee edit endpoint is a PATCH answering 204 No Content —
// unlike the 2.0 resources, which edit via POST and echo the object back.
func TestEmployeeUpdateUsesPatch(t *testing.T) {
	var method string
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.URL.Path != "/4.0/payroll/employees/"+payrollEmployeeID {
			t.Errorf("path = %q", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("body is not JSON: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "payroll-employee", "update", payrollEmployeeID,
		"--email", "anna.meyer@example.com", "--address-street-name", "Seestrasse",
		"--address-house-number", "7")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", method)
	}
	if body["email"] != "anna.meyer@example.com" {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["first_name"]; ok {
		t.Fatalf("unchanged flag was sent: %v", body)
	}
	address, ok := body["address"].(map[string]any)
	if !ok || address["street_name"] != "Seestrasse" || address["house_number"] != "7" {
		t.Fatalf("nested address = %v", body["address"])
	}
	if !strings.Contains(out, "Updated employee "+payrollEmployeeID) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestAbsenceListSendsBusinessYear(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/4.0/payroll/employees/"+payrollEmployeeID+"/absences" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("businessYear"); got != "2025" {
			t.Errorf("businessYear = %q", got)
		}
		w.Write([]byte(`{"data":[` + payrollAbsenceJSON + `]}`))
	}), "payroll-employee", "absence", "list", payrollEmployeeID, "--year", "2025")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Vacation") || !strings.Contains(out, "2026-07-06") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestAbsenceCreateHitsNestedPath(t *testing.T) {
	var path string
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("body is not JSON: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(payrollAbsenceJSON))
	}), "payroll-employee", "absence", "create", payrollEmployeeID,
		"--reason", "Vacation", "--start-date", "2026-07-06", "--end-date", "2026-07-17")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/4.0/payroll/employees/" + payrollEmployeeID + "/absences"; path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if body["reason"] != "Vacation" || body["start_date"] != "2026-07-06" || body["end_date"] != "2026-07-17" {
		t.Fatalf("body = %v", body)
	}
	if !strings.Contains(out, "Created absence 7c1f0f1a-6f0a-4f2e-9d1a-2a0d5c9a1b23") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// Absence edit is a PUT that replaces the whole object, so the CLI reads
// the current absence first and merges the changed flags on top.
func TestAbsenceUpdateMergesCurrentState(t *testing.T) {
	var methods []string
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte(payrollAbsenceJSON))
		case http.MethodPut:
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &body); err != nil {
				t.Errorf("body is not JSON: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
	}), "payroll-employee", "absence", "update", payrollEmployeeID,
		"7c1f0f1a-6f0a-4f2e-9d1a-2a0d5c9a1b23", "--end-date", "2026-07-24")
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodPut {
		t.Fatalf("methods = %v, want [GET PUT]", methods)
	}
	if body["end_date"] != "2026-07-24" {
		t.Fatalf("changed field not applied: %v", body)
	}
	if body["reason"] != "Vacation" || body["start_date"] != "2026-07-06" || body["paid_hours"] != 8.5 {
		t.Fatalf("current state not preserved: %v", body)
	}
}

func TestPaystubDownloadWritesFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "paystub.pdf")
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/4.0/payroll/employees/" + payrollEmployeeID + "/paystub-pdf-download/2026/7"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4 payslip"))
	}), "payroll-employee", "paystub", payrollEmployeeID, "2026", "7", "--out", target)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "%PDF-1.4 payslip" {
		t.Fatalf("file content = %q", data)
	}
	if !strings.Contains(out, "Wrote "+target) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestContactBulkCreatePostsArray(t *testing.T) {
	file := filepath.Join(t.TempDir(), "contacts.json")
	input := `[{"contact_type_id":1,"name_1":"ACME AG"},
	           {"contact_type_id":2,"name_1":"Meyer","name_2":"Anna","user_id":9,"owner_id":9}]`
	if err := os.WriteFile(file, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var posted []map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":1,"firstname":"Test","lastname":"User"}`))
		case "/2.0/contact/_bulk_create":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &posted); err != nil {
				t.Errorf("body is not JSON: %v", err)
			}
			w.Write([]byte(`[{"id":4,"nr":"10004","contact_type_id":1,"name_1":"ACME AG"},` +
				`{"id":5,"nr":"10005","contact_type_id":2,"name_1":"Meyer","name_2":"Anna"}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "contact-bulk-create", "--file", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(posted) != 2 {
		t.Fatalf("posted %d contacts: %v", len(posted), posted)
	}
	if posted[0]["name_1"] != "ACME AG" || posted[1]["name_2"] != "Anna" {
		t.Fatalf("posted = %v", posted)
	}
	// user_id/owner_id are required by the API and default to the
	// authenticated user; explicit values are kept.
	if posted[0]["user_id"] != float64(1) || posted[0]["owner_id"] != float64(1) {
		t.Fatalf("defaults not filled: %v", posted[0])
	}
	if posted[1]["user_id"] != float64(9) {
		t.Fatalf("explicit user_id overwritten: %v", posted[1])
	}
	if !strings.Contains(out, "Created 2 contacts") || !strings.Contains(out, "ACME AG") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestCommunicationKindListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/communication_kind" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":1,"name":"Mobile Phone"}]`))
	}), "communication-kind", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Mobile Phone") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestDocumentSettingViewTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_item_setting" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":1,"text":"Quote","kb_item_class":"KbOffer",` +
			`"enumeration_format":"AN-%nummer%","use_automatic_enumeration":true,` +
			`"next_nr":7,"default_time_period_in_days":14}]`))
	}), "document-setting", "view")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "KbOffer") || !strings.Contains(out, "AN-%nummer%") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestDocumentTemplateListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/document_templates" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"template_slug":"5f118cbc200a0c76ef1f34b2","name":"Standard template",` +
			`"is_default":true,"default_for_document_types":["type_offer","type_invoice"]}]`))
	}), "document-template", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "5f118cbc200a0c76ef1f34b2") || !strings.Contains(out, "type_offer,type_invoice") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}
