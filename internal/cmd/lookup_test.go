package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCountryListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/country" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`[{"id":1,"name":"Switzerland","name_short":"CH","iso3166_alpha2":"CH"}]`))
	}), "country", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Switzerland") || !strings.Contains(out, "CH") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestTaxListSendsTypesAndScope(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/taxes" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("types") != "sales_tax" {
			t.Errorf("types = %q", q.Get("types"))
		}
		if q.Get("scope") != "active" {
			t.Errorf("scope = %q", q.Get("scope"))
		}
		w.Write([]byte(`[{"id":1,"code":"UN77","type":"sales_tax","value":7.7,` +
			`"display_name":"UN77 (7.70%)","is_active":true}]`))
	}), "tax", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "UN77") || !strings.Contains(out, "7.7") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestUserMe(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3.0/users/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"id":4,"salutation_type":"male","firstname":"Rudolph",` +
			`"lastname":"Smith","email":"rudolph.smith@example.com","is_superadmin":true}`))
	}), "user", "me")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Rudolph") || !strings.Contains(out, "rudolph.smith@example.com") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestSalutationCreatePayload(t *testing.T) {
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/salutation" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"id":7,"name":"Herr"}`))
	}), "salutation", "create", "Herr")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["name"] != "Herr" {
		t.Fatalf("payload = %v", body)
	}
	if !strings.Contains(out, "Created salutation 7 (Herr)") {
		t.Fatalf("output: %s", out)
	}
}
