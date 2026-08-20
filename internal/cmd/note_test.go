package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const noteJSON = `{"id":4,"user_id":1,"event_start":"2026-08-20 14:20:00","subject":"API conception",` +
	`"info":"some details","contact_id":14,"project_id":null,"entry_id":null,"module_id":null}`

func TestNoteListTable(t *testing.T) {
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/note" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		w.Write([]byte("[" + noteJSON + "]"))
	}), "note", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "API conception") || !strings.Contains(out, "2026-08-20 14:20") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestNoteSearchBareTerm(t *testing.T) {
	var body string
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/note/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`[]`))
	}), "note", "search", "conception")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `{"field":"subject","value":"%conception%","criteria":"like"}`) {
		t.Fatalf("missing term clause: %s", body)
	}
}

func TestTaskCreateDefaultsUserID(t *testing.T) {
	var created map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3.0/users/me":
			w.Write([]byte(`{"id":4,"firstname":"Test","lastname":"User","email":"t@example.com"}`))
		case "/2.0/task":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(`{"id":9,"user_id":4,"subject":"Send documents","todo_status_id":1}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}), "task", "create", "--subject", "Send documents")
	if err != nil {
		t.Fatal(err)
	}
	if created["user_id"] != float64(4) {
		t.Fatalf("user_id default missing: %v", created)
	}
	if created["subject"] != "Send documents" {
		t.Fatalf("subject = %v", created["subject"])
	}
	if len(created) != 2 {
		t.Fatalf("expected only subject and user_id in payload, got %v", created)
	}
	if !strings.Contains(out, "Created task 9") {
		t.Fatalf("output: %s", out)
	}
}

func TestCommentCreatePath(t *testing.T) {
	var body map[string]any
	out, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/kb_order/5/comment" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"id":12,"text":"Shipment delayed","user_id":1,"user_name":"Peter Smith","is_public":true,"date":"2026-08-20 15:41:53"}`))
	}), "comment", "create", "kb_order", "5", "--text", "Shipment delayed", "--user-id", "1", "--is-public")
	if err != nil {
		t.Fatal(err)
	}
	if body["text"] != "Shipment delayed" || body["user_id"] != float64(1) || body["is_public"] != true {
		t.Fatalf("payload = %v", body)
	}
	if !strings.Contains(out, "Created comment 12 on kb_order 5") {
		t.Fatalf("output: %s", out)
	}
}

func TestCommentListRejectsBadType(t *testing.T) {
	_, err := runCmd(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}), "comment", "list", "kb_offert", "5")
	if err == nil || !strings.Contains(err.Error(), "invalid document type") {
		t.Fatalf("err = %v", err)
	}
}
