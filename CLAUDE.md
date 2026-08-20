# bexio-cli

Go CLI for the bexio REST API (contacts, quotes/orders/invoices,
items/stock/deliveries, projects/timesheets, notes/tasks, master data).
Single static binary, cobra command tree, hand-rolled client on
`net/http`, no SDK.

## Commands

```sh
make build          # build bin/bexio (version via ldflags)
make test           # go test ./...
make vet            # go vet ./...
make lint           # golangci-lint run (config: .golangci.yml, v2 format)
go test ./internal/api/    # single package
```

## Architecture

- `cmd/bexio/main.go` — entrypoint; `version` injected via `-ldflags -X main.version=`
- `internal/cmd/` — one file per command group (root, auth, contact, address,
  group, relation, sector, order, quote, invoice, article, project, note,
  lookup, api, docs). Core commands are added in `root.go`; module files
  self-register via `func init() { registerModule(newXxxCmd) }` — adding a
  module never edits root.go. All commands: resolve client via
  `newClient()`, respect `flagOutput` (`table`|`json`), render via
  `internal/output`. JSON output prints the raw API response (`.Raw`),
  never re-marshaled structs. Shared helpers in root.go (parseWhere,
  setIfChanged, listFlags, parseID) and order.go (parsePositionSpec,
  positionTypeNames, reportCreatedDocument — reused by quote/invoice).
- `internal/api/` — bexio REST client. `client.go` holds `Do()` (bearer
  token via `TokenSource`, JSON, error mapping, `ListOptions`,
  `SearchCriterion`); resource methods in `contacts.go`, `addresses.go`,
  `groups.go`, `relations.go`, `users.go`, `orders.go` (also the shared
  position + PDF helpers), `quotes.go`, `invoices.go`, `articles.go`,
  `projects.go`, `notes.go`, `lookups.go`.
- `internal/auth/` — OpenID Connect: Authorization Code Flow with PKCE on a
  loopback server (port 23946) and the refresh grant. Endpoint URLs are
  package vars so tests can stub them. `scopes.go` maps CLI modules to
  OAuth scopes (`auth login` module checklist / --modules); extend
  `auth.Modules` when adding a module with new scopes.
- `internal/config/` — `~/.config/bexio/config.yml` (0600), multiple
  instances (one per bexio company, auto-named after the slugified company
  name on login). Resolution order: `BEXIO_TOKEN` env → `--instance` flag →
  `BEXIO_INSTANCE` env → the sole configured instance (several configured
  without a selection is an error; deliberately no sticky default).
  `BEXIO_URL` overrides the API base URL (used by tests).
- `internal/output/` — tabwriter tables + JSON printer.

Tests use `httptest.Server` fixtures; no live account required. The cmd
tests drive the real command tree via env vars (`runCmd` in
`internal/cmd/contact_test.go`).

## bexio API facts (verified against docs.bexio.com)

- Base URL `https://api.bexio.com`; auth is always `Authorization: Bearer`.
  Three token kinds, same header: PAT (developer.bexio.com/pat, all scopes,
  60 days), API tokens, OAuth access tokens (JWT, short-lived).
- OAuth: IdP `https://auth.bexio.com/realms/bexio` (Keycloak;
  `/protocol/openid-connect/auth` + `/token`). `offline_access` scope yields
  a refresh token; refresh tokens rotate — always persist the new one.
  Scopes for contacts: `contact_show` / `contact_edit` (write implies read).
  A token is bound to exactly one company — multi-company = multiple config
  instances.
- Contacts are the 2.0 API: `GET/POST /2.0/contact`,
  `POST /2.0/contact/search`, `GET/POST/DELETE /2.0/contact/{id}`,
  `PATCH /2.0/contact/{id}/restore`, `POST /2.0/contact/_bulk_create`.
  Related: `contact_group`, `contact_relation`, `contact_branch` (sectors,
  read-only), `contact/{id}/additional_address`.
- Edit is POST (not PUT/PATCH) and partial: only sent fields change.
  DELETE returns `{"success": bool}` and archives; restore un-archives;
  `show_archived=true` lists archived only.
- `contact_type_id`: 1 company, 2 person (person: name_1 = last name,
  name_2 = first name). `user_id` and `owner_id` are required on create —
  the CLI defaults both to `/3.0/users/me`. `contact_group_ids` /
  `contact_branch_ids` are comma-separated strings, not arrays.
- The `address` field is deprecated in request payloads (2025-12-09):
  send `street_name` + `house_number` + `address_addition` instead;
  `house_number`/`address_addition` require `street_name`.
- Search: POST a JSON array of `{field, value, criteria}`; criteria include
  `=`, `!=`, `>`, `>=`, `<`, `<=`, `like`, `not_like`, `is_null`,
  `not_null`, `in`, `not_in` (default `like`). `like` needs explicit `%`
  wildcards. All array entries are ANDed. List/search params:
  `order_by` (comma-separated, `_desc` suffix), `limit` (max 2000),
  `offset`, `show_archived`.
- kb documents (kb_offer/kb_order/kb_invoice) share the position endpoints
  `/2.0/{kb_document_type}/{document_id}/kb_position_*` and the comment
  endpoint. Status enums (`kb_item_status_id`, read-only) differ per type:
  quotes 1 draft/2 pending/3 confirmed/4 declined; orders 5 pending/6
  done/15 partial/21 canceled; invoices 7 draft/8 pending/9 paid/16
  partial/19 canceled/31 unpaid; deliveries 10 draft/18 done/20 canceled.
- kb action endpoints take no body and return `{"success"}` (issue,
  accept, reject, cancel, mark_as_sent, delivery issue...). Quirk: the
  quote revert path is camelCase `/kb_offer/{id}/revertIssue`; the invoice
  one is `/kb_invoice/{id}/revert_issue`. `copy` requires `contact_id`.
  `send` bodies require recipient_email/subject/message and the message
  must contain the literal `[Network Link]` placeholder.
- Invoice reminders live under `/kb_invoice/{id}/kb_reminder` (not
  /reminder); reminder_level auto-increments.
- Stock reads (`/2.0/stock`, `/2.0/stock_place`) require the `stock_edit`
  scope — no show scope exists. Article prices are decimal strings.
- Notes/tasks/projects: writable project link is `pr_project_id`,
  responses return read-only `project_id`. Timesheet create needs a nested
  write-only `tracking` object ({type:"duration",date,duration} or
  {type:"range",start,end}); top-level date/duration are read-only.
- 3.0 endpoints paginate with limit/offset only (no order_by); project
  milestone edit is POST but package edit is PATCH. Project create
  requires name, contact_id, pr_state_id, pr_project_type_id, user_id.
- Deletes: kb documents and projects delete permanently (CLI requires
  --force); contacts archive + restore; projects also archive/reactivate.
- Lookup resources (country, language, salutation, title, unit,
  payment_type, users, company_profile, permissions) are tagged with a
  `general` scope in the docs, but `general` is NOT requestable — the IdP
  returns invalid_scope for it (one bad scope rejects the whole request);
  it is granted implicitly with any token. Bank accounts need
  `bank_account_show`; currencies and taxes declare no scope. `/3.0/taxes?types=sales_tax&scope=active` lists
  the taxes valid for document positions.
- Errors: `{"error_code": <int>, "message": "..."}`. 429 = rate limit per
  company per minute (`RateLimit-*` response headers).
- No official OpenAPI download, but docs.bexio.com is a pre-rendered Redoc
  page with the full spec embedded in `__redoc_state` (extractable with
  `json.JSONDecoder.raw_decode` after `"__redoc_state = "`).

## Conventions

- New commands: constructor `newXxxCmd() *cobra.Command` in `internal/cmd/`,
  registered in `root.go` (or under the resource's parent command).
  Always support `-o json`; call `validateOutput()` first in RunE.
- Follow the bexio API scheme: command and flag names mirror API resource
  and field names 1:1 (`--name-1` -> `name_1`); don't invent friendlier
  aliases beyond `--type company|person`.
- Write commands build partial payloads via `setIfChanged` (only flags the
  user passed are sent).
- Search filters go through `parseWhere` (`field=value`, `~` for like, …) —
  extend that rather than adding parallel logic.
- Errors: return them; root command has `SilenceUsage` — no manual usage dumps.

## Commits

Conventional Commits, enforced by commitlint in CI and consumed by
release-please for versioning + CHANGELOG. `feat:` = minor, `fix:`/`perf:` =
patch, `!`/`BREAKING CHANGE:` = major; `docs:`/`test:`/`ci:`/`chore:`/
`refactor:` don't bump. Imperative, lowercase, no trailing period; optional
scope = command or package name (`feat(contact): ...`).

## Release

Fully automated: release-please watches `main`, maintains a release PR;
merging it creates the `v*` tag, which triggers goreleaser
(`.github/workflows/release.yml`): darwin/linux/windows binaries + Homebrew
formula pushed to `lukeisontheroad/homebrew-tap` (needs `HOMEBREW_TAP_TOKEN`
secret). Never tag manually.
