package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const quoteJSON = `{"id":4,"document_nr":"AN-00001","title":"Website relaunch",` +
	`"contact_id":17,"user_id":1,"total":"1750.000000","total_net":"1750.000000",` +
	`"currency_id":1,"kb_item_status_id":2,"is_valid_from":"2026-08-01",` +
	`"is_valid_until":"2026-09-01","updated_at":"2026-08-01 10:30:00"}`

func TestQuoteListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_offer" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		w.Write([]byte("[" + quoteJSON + "]"))
	}), "kb-offer", "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AN-00001", "pending", "Website relaunch", "17", "1750.000000"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestQuoteViewJSON(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_offer/4" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(quoteJSON))
	}), "kb-offer", "view", "4", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	// JSON output must be the raw API object, not a re-marshaled struct.
	var got, want any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if err := json.Unmarshal([]byte(quoteJSON), &want); err != nil {
		t.Fatal(err)
	}
	gotB, _ := json.Marshal(got)
	wantB, _ := json.Marshal(want)
	if string(gotB) != string(wantB) {
		t.Fatalf("raw passthrough mismatch:\ngot  %s\nwant %s", gotB, wantB)
	}
}

func TestQuoteCreatePayload(t *testing.T) {
	var payload map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":99}`))
		case "/2.0/kb_offer":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			w.Write([]byte(quoteJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "kb-offer", "create", "--contact-id", "17", "--title", "Website relaunch",
		"--position", "type=article,article_id=5,amount=2")
	if err != nil {
		t.Fatal(err)
	}
	if got := payload["contact_id"]; got != float64(17) {
		t.Errorf("contact_id = %v", got)
	}
	if got := payload["user_id"]; got != float64(99) {
		t.Errorf("user_id = %v (want default from /3.0/users/me)", got)
	}
	positions, ok := payload["positions"].([]any)
	if !ok || len(positions) != 1 {
		t.Fatalf("positions = %v", payload["positions"])
	}
	pos := positions[0].(map[string]any)
	if pos["type"] != "KbPositionArticle" || pos["article_id"] != float64(5) {
		t.Errorf("position = %v", pos)
	}
	if !strings.Contains(out, "Created quote 4") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestQuoteRevertIssuePath(t *testing.T) {
	var hitPath string
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		w.Write([]byte(`{"success":true}`))
	}), "kb-offer", "revert-issue", "4")
	if err != nil {
		t.Fatal(err)
	}
	if hitPath != "/2.0/kb_offer/4/revertIssue" {
		t.Errorf("path = %q (want the camelCase /revertIssue)", hitPath)
	}
	if !strings.Contains(out, "Reverted issue of quote 4") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestQuoteDeleteRequiresForce(t *testing.T) {
	called := false
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), "kb-offer", "delete", "4")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v (want --force requirement)", err)
	}
	if called {
		t.Fatal("API was called without --force")
	}
}
