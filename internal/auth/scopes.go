package auth

import (
	"fmt"
	"strconv"
	"strings"
)

// BaseScopes are always requested: OIDC identity, refresh tokens, and
// company info for `auth status`. Note: the docs tag master-data endpoints
// with a `general` scope, but the IdP rejects it as requestable
// (invalid_scope) — it is granted implicitly with any valid token.
var BaseScopes = []string{
	"openid", "profile", "email", "offline_access", "company_profile",
}

// Module groups the CLI's commands by the OAuth scopes they need, so
// `auth login` can request access per module. Scope sets are taken from the
// endpoint security definitions on docs.bexio.com (write implies read).
type Module struct {
	Name        string
	Description string
	Scopes      []string
}

var Modules = []Module{
	{"contacts", "contacts, groups, relations, addresses", []string{"contact_show", "contact_edit"}},
	{"orders", "sales orders (+ create invoices/deliveries from them)", []string{"kb_order_show", "kb_order_edit", "kb_invoice_edit", "kb_delivery_edit"}},
	{"quotes", "quotes (+ convert to orders/invoices)", []string{"kb_offer_show", "kb_offer_edit", "kb_order_edit", "kb_invoice_edit"}},
	{"invoices", "invoices, payments, reminders", []string{"kb_invoice_show", "kb_invoice_edit"}},
	{"items", "items/products, stock, deliveries", []string{"article_show", "article_edit", "stock_edit", "kb_delivery_show", "kb_delivery_edit"}},
	{"projects", "projects, milestones, timesheets", []string{"project_show", "project_edit", "monitoring_show", "monitoring_edit"}},
	{"notes-tasks", "notes and tasks", []string{"note_show", "note_edit", "task_show", "task_edit"}},
	{"master-data", "users, bank accounts, taxes, currencies", []string{"bank_account_show"}},
}

// AllModuleNames returns the module names in display order.
func AllModuleNames() []string {
	names := make([]string, len(Modules))
	for i, m := range Modules {
		names[i] = m.Name
	}
	return names
}

// readOnlyScope reports whether a scope is safe for a read-only login:
// the _show variants, plus stock_edit — the API has no stock_show, and
// stock_edit alone only unlocks the read-only stock list endpoints (writing
// stock requires article_edit on top).
func readOnlyScope(s string) bool {
	return strings.HasSuffix(s, "_show") || s == "stock_edit"
}

// ScopesFor resolves module selections (names or 1-based list numbers;
// "all" or empty selects everything) into the space-separated scope string
// for the authorization request. readOnly drops all write scopes, so the
// server itself rejects modifications with this token.
func ScopesFor(selection []string, readOnly bool) (string, error) {
	selected := map[string]bool{}
	all := len(selection) == 0
	for _, s := range selection {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || s == "all" {
			all = true
			continue
		}
		if n, err := strconv.Atoi(s); err == nil {
			if n < 1 || n > len(Modules) {
				return "", fmt.Errorf("module number %d out of range 1-%d", n, len(Modules))
			}
			selected[Modules[n-1].Name] = true
			continue
		}
		found := false
		for _, m := range Modules {
			if m.Name == s {
				selected[m.Name] = true
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("unknown module %q (available: %s)", s, strings.Join(AllModuleNames(), ", "))
		}
	}

	scopes := append([]string{}, BaseScopes...)
	seen := map[string]bool{}
	for _, s := range scopes {
		seen[s] = true
	}
	for _, m := range Modules {
		if !all && !selected[m.Name] {
			continue
		}
		for _, s := range m.Scopes {
			if readOnly && !readOnlyScope(s) {
				continue
			}
			if !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	return strings.Join(scopes, " "), nil
}
