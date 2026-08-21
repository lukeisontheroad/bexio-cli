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
	// OptIn keeps a module out of the "all" selection: its scopes are only
	// requested when the module is named explicitly. Used for scopes that
	// authorize irreversible real-world effects, so a plain `auth login`
	// never mints a token capable of them.
	OptIn bool
}

var Modules = []Module{
	{Name: "contacts", Description: "contacts, groups, relations, addresses", Scopes: []string{"contact_show", "contact_edit"}},
	{Name: "orders", Description: "sales orders (+ create invoices/deliveries from them)", Scopes: []string{"kb_order_show", "kb_order_edit", "kb_invoice_edit", "kb_delivery_edit"}},
	{Name: "quotes", Description: "quotes (+ convert to orders/invoices)", Scopes: []string{"kb_offer_show", "kb_offer_edit", "kb_order_edit", "kb_invoice_edit"}},
	{Name: "invoices", Description: "invoices, payments, reminders", Scopes: []string{"kb_invoice_show", "kb_invoice_edit"}},
	{Name: "items", Description: "items/products, stock, deliveries", Scopes: []string{"article_show", "article_edit", "stock_edit", "kb_delivery_show", "kb_delivery_edit"}},
	{Name: "projects", Description: "projects, milestones, timesheets", Scopes: []string{"project_show", "project_edit", "monitoring_show", "monitoring_edit"}},
	{Name: "notes-tasks", Description: "notes and tasks", Scopes: []string{"note_show", "note_edit", "task_show", "task_edit"}},
	{Name: "master-data", Description: "users, bank accounts, taxes, currencies", Scopes: []string{"bank_account_show"}},
	{Name: "files", Description: "file manager (upload, download, attachments)", Scopes: []string{"file"}},
	{Name: "accounting", Description: "manual entries, accounts, business/calendar years, VAT, journal", Scopes: []string{"accounting"}},
	// Deliberately without bank_payment_edit, which only `outgoing-payment
	// update` needs: that scope also authorizes /4.0/banking/payments, so
	// granting it here would hand every default login the ability to move
	// money. Combine with the banking-payments module when you need it.
	{Name: "purchase", Description: "supplier bills, expenses, purchase orders, outgoing payments", Scopes: []string{"contact_show", "kb_bill_show", "kb_expense_show", "kb_article_order_show", "kb_article_order_edit"}},
	{Name: "banking-payments", Description: "outgoing bank transfers — MOVES MONEY, opt in explicitly", Scopes: []string{"bank_payment_show", "bank_payment_edit"}, OptIn: true},
	{Name: "payroll", Description: "employees, absences, paystubs — SENSITIVE, opt in explicitly", Scopes: []string{"payroll_employee_show", "payroll_employee_edit", "payroll_absence_show", "payroll_absence_edit", "payroll_paystub_show"}, OptIn: true},
}

// AllModuleNames returns the module names in display order.
func AllModuleNames() []string {
	names := make([]string, len(Modules))
	for i, m := range Modules {
		names[i] = m.Name
	}
	return names
}

// singleFlavourScopes are scopes bexio does not split into _show/_edit.
// Dropping them from a read-only login would remove read access entirely
// (no stock lists, no files, no journal), so they are kept — but the server
// cannot enforce read-only for them: only the client-side Client.ReadOnly
// guard refuses writes. Everything else in a read-only login is enforced by
// bexio itself. stock_edit alone unlocks just the read-only stock lists;
// writing stock additionally needs article_edit.
var singleFlavourScopes = map[string]bool{
	"stock_edit": true,
	"file":       true,
	"accounting": true,
}

// readOnlyScope reports whether a scope belongs in a read-only login.
func readOnlyScope(s string) bool {
	return strings.HasSuffix(s, "_show") || singleFlavourScopes[s]
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
		if m.OptIn {
			// Opt-in modules are never covered by "all".
			if !selected[m.Name] {
				continue
			}
		} else if !all && !selected[m.Name] {
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
