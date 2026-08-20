package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const projectJSON = `{"id":3,"uuid":"046b6c7f-0b8a-43b9-b35d-6489e6daee91","nr":"000003",` +
	`"name":"Website Relaunch","start_date":"2026-08-01 00:00:00","end_date":null,` +
	`"comment":"","pr_state_id":1,"pr_project_type_id":1,"contact_id":17,` +
	`"contact_sub_id":null,"pr_invoice_type_id":null,"pr_invoice_type_amount":"0.00",` +
	`"pr_budget_type_id":null,"pr_budget_type_amount":"0.00","user_id":4}`

func TestProjectListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/pr_project" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte("[" + projectJSON + "]"))
	}), "pr-project", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Website Relaunch") || !strings.Contains(out, "000003") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestProjectCreatePayload(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/pr_project":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(projectJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "pr-project", "create", "--name", "Website Relaunch", "--contact-id", "17",
		"--pr-state-id", "1", "--pr-project-type-id", "1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"contact_id": 17, "pr_state_id": 1, "pr_project_type_id": 1, "user_id": 4}
	for field, v := range want {
		if created[field] != v {
			t.Fatalf("%s = %v, want %v (payload %v)", field, created[field], v, created)
		}
	}
	if created["name"] != "Website Relaunch" {
		t.Fatalf("name = %v", created["name"])
	}
	if !strings.Contains(out, "Created project 3") {
		t.Fatalf("output: %s", out)
	}
}

func TestProjectCreateRequiresName(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "pr-project", "create", "--contact-id", "17", "--pr-state-id", "1", "--pr-project-type-id", "1")
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestTimesheetCreateTracking(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/timesheet":
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(`{"id":42,"user_id":4,"client_service_id":1,"allowable_bill":true,` +
				`"date":"2026-08-20","duration":"01:30"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "timesheet", "create", "--client-service-id", "1", "--pr-project-id", "3",
		"--allowable-bill", "--tracking-type", "duration", "--date", "2026-08-20", "--duration", "01:30")
	if err != nil {
		t.Fatal(err)
	}
	tracking, ok := created["tracking"].(map[string]any)
	if !ok {
		t.Fatalf("tracking missing or not an object: %v", created)
	}
	if tracking["type"] != "duration" || tracking["date"] != "2026-08-20" || tracking["duration"] != "01:30" {
		t.Fatalf("tracking = %v", tracking)
	}
	if created["allowable_bill"] != true {
		t.Fatalf("allowable_bill = %v", created["allowable_bill"])
	}
	if created["client_service_id"] != float64(1) || created["pr_project_id"] != float64(3) {
		t.Fatalf("payload = %v", created)
	}
	if created["user_id"] != float64(4) {
		t.Fatalf("user_id default missing: %v", created)
	}
	if _, ok := created["date"]; ok {
		t.Fatalf("top-level date must not be sent: %v", created)
	}
	if !strings.Contains(out, "Created timesheet 42") {
		t.Fatalf("output: %s", out)
	}
}

func TestTimesheetCreateRangeValidation(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "timesheet", "create", "--client-service-id", "1",
		"--tracking-type", "range", "--start", "2026-08-20 14:00:00")
	if err == nil || !strings.Contains(err.Error(), "--start and --end") {
		t.Fatalf("err = %v", err)
	}
}

func TestMilestoneCreatePath(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/projects/9/milestones" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &created); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"id":4,"name":"Go live","end_date":"2026-12-01","comment":""}`))
	}), "pr-project", "milestone", "create", "9", "--name", "Go live", "--end-date", "2026-12-01")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "Go live" || created["end_date"] != "2026-12-01" {
		t.Fatalf("payload = %v", created)
	}
	if !strings.Contains(out, "Created milestone 4") {
		t.Fatalf("output: %s", out)
	}
}
