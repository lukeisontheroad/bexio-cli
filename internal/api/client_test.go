package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, StaticToken("test-token"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDoSetsHeaders(t *testing.T) {
	var gotAuth, gotAccept string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Write([]byte(`{}`))
	})
	if err := c.Get(context.Background(), "/3.0/users/me", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
}

func TestDoErrorMapping(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error_code":401,"message":"The bearer token is invalid"}`))
	})
	err := c.Get(context.Background(), "/2.0/contact", nil, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 || apiErr.Message != "The bearer token is invalid" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestListContactsPreservesRaw(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/contact" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Errorf("limit = %q", got)
		}
		w.Write([]byte(`[{"id":1,"nr":"10001","contact_type_id":1,"name_1":"ACME AG","mail":"info@acme.ch","custom_field":"kept"}]`))
	})
	contacts, err := c.ListContacts(context.Background(), ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Name1 != "ACME AG" {
		t.Fatalf("got %+v", contacts)
	}
	var raw map[string]any
	if err := json.Unmarshal(contacts[0].Raw, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["custom_field"] != "kept" {
		t.Fatalf("raw JSON not preserved: %v", raw)
	}
	if contacts[0].TypeName() != "company" {
		t.Fatalf("TypeName = %q", contacts[0].TypeName())
	}
}

func TestSearchContactsBody(t *testing.T) {
	var body []byte
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Write([]byte(`[]`))
	})
	criteria := []SearchCriterion{{Field: "name_1", Value: "%Meyer%", Criteria: "like"}}
	if _, err := c.SearchContacts(context.Background(), criteria, ListOptions{}); err != nil {
		t.Fatal(err)
	}
	want := `[{"field":"name_1","value":"%Meyer%","criteria":"like"}]`
	if string(body) != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

func TestPersonName(t *testing.T) {
	c := Contact{ContactTypeID: 2, Name1: "Meyer", Name2: "Anna"}
	if c.Name() != "Anna Meyer" {
		t.Fatalf("Name() = %q", c.Name())
	}
	c = Contact{ContactTypeID: 1, Name1: "ACME AG", Name2: "Holding"}
	if c.Name() != "ACME AG Holding" {
		t.Fatalf("Name() = %q", c.Name())
	}
}

func TestDeleteContactChecksSuccess(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q", r.Method)
		}
		w.Write([]byte(`{"success":false}`))
	})
	if err := c.DeleteContact(context.Background(), 5); err == nil {
		t.Fatal("expected error on success=false")
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	var hits int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`[]`))
	})
	c.ReadOnly = true
	// GET and POST .../search pass.
	if err := c.Get(context.Background(), "/2.0/contact", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SearchContacts(context.Background(), nil, ListOptions{}); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("expected 2 requests, got %d", hits)
	}
	// Everything else is refused before any request is sent.
	if err := c.Do(context.Background(), http.MethodPost, "/2.0/contact", nil, map[string]any{}, nil); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("POST not blocked: %v", err)
	}
	if err := c.DeleteContact(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("DELETE not blocked: %v", err)
	}
	if err := c.Do(context.Background(), http.MethodPatch, "/2.0/contact/1/restore", nil, nil, nil); err == nil {
		t.Fatal("PATCH not blocked")
	}
	if hits != 2 {
		t.Fatalf("blocked request still reached the server (hits=%d)", hits)
	}
}
