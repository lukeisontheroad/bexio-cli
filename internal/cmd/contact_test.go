package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukeisontheroad/bexio-cli/internal/config"
)

// runCmd executes the CLI against a fake API server and returns stdout.
func runCmd(t *testing.T, handler http.Handler, args ...string) (string, error) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvToken, "test-token")
	t.Setenv(config.EnvURL, srv.URL)
	t.Setenv(config.EnvConfig, filepath.Join(t.TempDir(), "config.yml"))

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.Execute()
	// Reset package-level flag state for the next test.
	flagInstance, flagOutput, flagVerbose = "", "table", false
	return out.String(), err
}

const contactJSON = `{"id":17,"nr":"10017","contact_type_id":2,"name_1":"Meyer","name_2":"Anna",` +
	`"mail":"anna@example.com","city":"Zürich","updated_at":"2026-08-01 10:30:00"}`

func TestContactListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/contact" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte("[" + contactJSON + "]"))
	}), "contact", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Anna Meyer") || !strings.Contains(out, "person") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestContactViewJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/contact/17" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(contactJSON))
	}), "contact", "view", "17", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if m["name_1"] != "Meyer" {
		t.Fatalf("got %v", m)
	}
}

func TestContactSearchBuildsCriteria(t *testing.T) {
	var body string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/contact/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`[]`))
	}), "contact", "search", "Meyer", "-w", "city=Zürich")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `{"field":"city","value":"Zürich","criteria":"equal"}`) {
		t.Fatalf("missing where clause: %s", body)
	}
	if !strings.Contains(body, `{"field":"name_1","value":"%Meyer%","criteria":"like"}`) {
		t.Fatalf("missing term clause: %s", body)
	}
}

func TestContactCreateDefaultsUserID(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/contact":
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(contactJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "contact", "create", "--type", "person", "--name-1", "Meyer", "--name-2", "Anna")
	if err != nil {
		t.Fatal(err)
	}
	if created["contact_type_id"] != float64(2) {
		t.Fatalf("contact_type_id = %v", created["contact_type_id"])
	}
	if created["user_id"] != float64(4) || created["owner_id"] != float64(4) {
		t.Fatalf("user/owner defaults missing: %v", created)
	}
	if !strings.Contains(out, "Created contact 17") {
		t.Fatalf("output: %s", out)
	}
}

func TestContactUpdatePartial(t *testing.T) {
	var body map[string]any
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(contactJSON))
	}), "contact", "update", "17", "--mail", "new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["mail"] != "new@example.com" {
		t.Fatalf("expected only mail in payload, got %v", body)
	}
}

func TestContactUpdateNothing(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "contact", "update", "17")
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("err = %v", err)
	}
}

func TestAPICommandRawBody(t *testing.T) {
	var body string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`[{"id":1}]`))
	}), "api", "POST", "/2.0/contact/search", "--body", `[{"field":"mail","value":"%x%"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if body != `[{"field":"mail","value":"%x%"}]` {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(out, `"id": 1`) {
		t.Fatalf("output: %s", out)
	}
}

func TestParseWhere(t *testing.T) {
	cases := map[string][2]string{
		"name_1~Meyer":           {"like", "Meyer"},
		"nr>10":                  {"greater_than", "10"},
		"nr>=10":                 {"greater_equal", "10"},
		"mail!=x@y.z":            {"not_equal", "x@y.z"},
		"mail!~%spam%":           {"not_like", "%spam%"},
		"updated_at<=2026-01-01": {"less_equal", "2026-01-01"},
		"city=Zürich":            {"equal", "Zürich"},
	}
	for clause, want := range cases {
		got, err := parseWhereClause(clause)
		if err != nil {
			t.Fatalf("%s: %v", clause, err)
		}
		if got.Criteria != want[0] || got.Value != want[1] {
			t.Fatalf("%s: got %+v", clause, got)
		}
	}
	if _, err := parseWhereClause("no-operator"); err == nil {
		t.Fatal("expected error")
	}
}
