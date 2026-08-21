---
name: bexio
description: >
  Work with the bexio business software via the bexio CLI: contacts, quotes,
  sales orders, invoices (payments, reminders), items & stock, deliveries,
  purchase (supplier bills, expenses), accounting (manual entries, journal),
  files, projects & timesheets, notes & tasks, payroll, and master data
  lookups. Use when the user asks about their bexio contacts, offers, orders,
  invoices, bills, bookkeeping, projects, or time tracking from the terminal.
---

# Operating the bexio CLI

## Setup check

Run `bexio auth status`. If it fails, the user must authenticate first:
`bexio auth login` (browser OAuth flow) or `bexio auth login --token <PAT>`.
Do not attempt to create tokens or complete the browser consent yourself.

With several bexio companies configured, every command needs
`--instance <name>` (or `BEXIO_INSTANCE`); the error message lists the
available names. A single configured company is used automatically.

## Reference

Run `bexio docs` once — a compact page (setup, conventions, search syntax,
document workflows, command index) designed to stay in context. For the
subcommands, flags, and examples of one resource, fetch on demand:
`bexio docs <command>` (e.g. `bexio docs kb-invoice`). Avoid
`bexio docs --full` — it prints ~3000 lines.

## Rules

- Use `-o json` whenever you consume output programmatically; tables
  truncate long values. JSON output is the raw bexio API objects with the
  exact field names from docs.bexio.com.
- Search uses `--where` clauses over raw API field names
  (`--where mail~%acme%`, `--where contact_type_id=1`); `~` values need
  explicit `%` wildcards, bare positional terms are wrapped automatically.
- Read operations are safe to run freely. Write operations change the
  company's books — confirm with the user before `create`/`update`/
  `delete`/`issue`/`send`/`payment` unless they explicitly asked for the
  exact change.
- If you only need lookups, add the global `--read-only` flag (or set
  `BEXIO_READ_ONLY=1`): the CLI then refuses every modifying request.
- **`banking-payment` instructs REAL bank transfers** — money leaving the
  company account, irreversible once the bank executes. Never run its
  `create`/`update`/`delete`/`cancel` on your own initiative; only when the
  user asked for that exact transfer, and quote back amount, recipient IBAN,
  and execution date first. The `--force` flag those commands require is not
  your confirmation to give.
- `payroll-*` commands expose salary and absence data about real employees.
  Treat output as confidential: do not summarize it into places the user did
  not ask for, and do not modify employee records without an explicit request.
- `manual-entry` writes into the general ledger, and booked entries in a
  closed period may be locked. Prefer reading (`journal list`) unless asked
  to book something.
- `send` commands e-mail REAL customers; the message body must contain the
  literal `[Network Link]` placeholder.
- Deleting quotes, orders, invoices, bills, expenses, or projects is
  permanent (`--force`); contact delete only archives (`contact restore`
  undoes it), and files have no restore either — archive instead.
- Ids reference lookups: resolve country/language/salutation/title/unit/
  tax/currency ids via their list commands; `contact_type_id` is 1 company,
  2 person (persons: name_1 = last name, name_2 = first name).
- The CLI spans three API generations and they behave differently: 2.0
  (contacts, kb documents) uses integer ids, POST edits and `--where`
  searches; 3.0 (files, accounting, purchase orders) uses PUT/PATCH and
  limit/offset; 4.0 (purchase bills/expenses, banking, payroll) uses UUID
  ids, full-replace updates, `{"data":…,"paging":…}` list envelopes, and
  query-parameter filters instead of `--where`.
- A 403 usually means the stored token lacks that module's OAuth scope
  (scopes are chosen at login), not that the user lacks permission. Tell
  the user to re-run `bexio auth login`; `banking-payments` and `payroll`
  additionally need `--modules …` because they are opt-in.
- Anything not covered by a command: `bexio api METHOD /2.0/...` reaches
  every endpoint with auth handled.
