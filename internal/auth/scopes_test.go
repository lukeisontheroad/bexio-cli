package auth

import (
	"strings"
	"testing"
)

func TestScopesForAll(t *testing.T) {
	s, err := ScopesFor(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"openid", "offline_access", "contact_edit", "kb_order_edit", "kb_invoice_show"} {
		if !strings.Contains(" "+s+" ", " "+want+" ") {
			t.Fatalf("all scopes missing %q: %s", want, s)
		}
	}
}

func TestScopesForSelection(t *testing.T) {
	s, err := ScopesFor([]string{"contacts", "2"}, false) // 2 = orders
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"contact_edit", "kb_order_edit"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if strings.Contains(s, "kb_offer_show") || strings.Contains(s, "note_show") {
		t.Fatalf("unselected module scopes leaked: %s", s)
	}
}

func TestScopesForErrors(t *testing.T) {
	if _, err := ScopesFor([]string{"nope"}, false); err == nil {
		t.Fatal("expected error for unknown module")
	}
	if _, err := ScopesFor([]string{"99"}, false); err == nil {
		t.Fatal("expected error for out-of-range number")
	}
}

func TestScopesForReadOnly(t *testing.T) {
	s, err := ScopesFor(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, "_edit") && !strings.Contains(s, "stock_edit") {
		t.Fatalf("write scopes leaked into read-only set: %s", s)
	}
	for _, banned := range []string{"contact_edit", "kb_order_edit", "kb_invoice_edit", "article_edit", "project_edit", "note_edit", "task_edit", "monitoring_edit", "kb_offer_edit", "kb_delivery_edit"} {
		if strings.Contains(" "+s+" ", " "+banned+" ") {
			t.Fatalf("read-only scopes contain %q: %s", banned, s)
		}
	}
	for _, want := range []string{"contact_show", "kb_order_show", "stock_edit", "bank_account_show"} {
		if !strings.Contains(" "+s+" ", " "+want+" ") {
			t.Fatalf("read-only scopes missing %q: %s", want, s)
		}
	}
}
