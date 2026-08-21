# bexio-cli

Command-line client for the [bexio](https://www.bexio.com) business software,
covering the documented API surface: contacts, quotes, sales orders,
invoices, items & stock, purchase, accounting, files, projects & timesheets,
notes & tasks, payroll, and master data. Single static Go binary, no
dependencies on an SDK — the API client follows
[docs.bexio.com](https://docs.bexio.com) directly.

```sh
bexio contact search Meyer
bexio contact create --type person --name-1 Meyer --name-2 Anna --mail anna@example.com
bexio contact view 17 -o json
```

## Install

```sh
go install github.com/lukeisontheroad/bexio-cli/cmd/bexio@latest
# or
make build   # -> bin/bexio
```

## Authentication

Two methods, both stored per instance in `~/.config/bexio/config.yml` (0600):

**OAuth (default, does not expire).**

```sh
bexio auth login
```

Opens the browser once for consent using the CLI's built-in app; afterwards
tokens refresh automatically. (The embedded client secret is not
confidential — see RFC 8252 §8.5; consent, the localhost redirect whitelist,
and PKCE provide the security.) To use your own app from
[developer.bexio.com](https://developer.bexio.com), allow the redirect URL
`http://localhost:23946/callback` and pass `--client-id`/`--client-secret`.

**Personal Access Token.** Create one at
[developer.bexio.com/pat](https://developer.bexio.com/pat) (all scopes, valid
60 days), then:

```sh
bexio auth login --token <PAT>   # or --pat to be prompted (hidden input)
```

Scripting: `BEXIO_TOKEN=<token>` overrides everything.

**Read-only mode.** `bexio auth login --read-only` requests only read
(`_show`) scopes — the server itself rejects writes — and additionally marks
the instance read-only so the CLI refuses every modifying request (searches
still work). For a single invocation use the global `--read-only` flag or
`BEXIO_READ_ONLY=1`. Handy for AI agents that only need lookups. Three bexio
scopes (`file`, `accounting`, `stock_edit`) have no read-only counterpart, so
for those the client-side guard is the only line of defence.

**Scope selection.** The login shows a checklist of modules and requests only
the scopes those modules need. Modules that authorize money movement
(`banking-payments`) or personal data (`payroll`) are **opt-in**: they are
never part of "all" and must be named explicitly, e.g.
`bexio auth login --modules contacts,invoices,banking-payments`.

### Multiple bexio companies

A bexio token is bound to one company. Log in once per company — each login
is stored as an instance named after the company (slugified, `--name`
overrides). One instance is used automatically; with several, pick per call
with `--instance` (or `BEXIO_INSTANCE`) — there is no sticky default:

```sh
bexio auth login                     # -> instance "acme-ag"
bexio auth login                     # second company -> e.g. "freelance-gmbh"
bexio --instance freelance-gmbh contact list
```

## Commands

Run `bexio docs` for the full reference. Commands are named after bexio's own
resource names; the API's internal prefixes (`kb_offer`, `pr_project`, …) work
as aliases.

**Contacts**

| Command | Description |
| --- | --- |
| `contact` | list, view, search, create, update; `delete` archives and `restore` brings it back |
| `contact address` | additional addresses of a contact |
| `contact-group` | contact groups |
| `contact-relation` | company ↔ person links |
| `contact-sector` | contact sectors (read-only) |
| `contact-bulk-create` | create many contacts from a JSON file |

**Sales documents**

| Command | Description |
| --- | --- |
| `quote` | quotes: CRUD, pdf, issue/accept/reject/send/copy, convert to order or invoice |
| `order` | sales orders: CRUD, pdf, positions, repetition, convert to invoice or delivery |
| `invoice` | invoices: CRUD, pdf, lifecycle, plus `payment` and `reminder` subcommands |
| `delivery` | delivery notes: list, view, issue (issuing books the stock movements) |

**Purchase**

| Command | Description |
| --- | --- |
| `bill` | supplier bills: CRUD, booking transitions, duplicate |
| `expense` | expenses, same shape as bills |
| `purchase-order` | purchase orders sent to suppliers |
| `outgoing-payment` | payments recorded against a bill |

**Accounting**

| Command | Description |
| --- | --- |
| `manual-entry` | manual bookings with debit/credit lines and receipt attachments |
| `journal` | the accounting journal report |
| `account` | ledger accounts |
| `account-group` | account groups |
| `business-year` | business years |
| `calendar-year` | calendar years |
| `vat-period` | VAT periods |

**Items, projects, and work**

| Command | Description |
| --- | --- |
| `article` | items and products (alias `item`) |
| `stock` | stock locations (read-only) |
| `stock-area` | stock areas (read-only) |
| `project` | projects, incl. `milestone` and `package`, archive/reactivate |
| `timesheet` | time tracking |
| `client-service` | business activities referenced by timesheets |
| `note` | notes |
| `task` | tasks |
| `comment` | comments on quotes, orders, and invoices |

**Files, payroll, banking**

| Command | Description |
| --- | --- |
| `file` | upload, download, preview, search files |
| `payroll-employee` | employees, absences, paystub download (opt-in scope) |
| `banking-payment` | outgoing bank transfers — **moves real money**, opt-in scope, `--force` on every write |

**Master data**

| Command | Description |
| --- | --- |
| `country`, `language`, `salutation`, `title`, `unit` | 2.0 lookups referenced by contacts and documents |
| `payment-type`, `communication-kind` | payment and communication types |
| `currency`, `tax`, `bank-account` | 3.0 financial master data |
| `user`, `company-profile`, `permission` | the authenticated account and its access |
| `document-setting`, `document-template` | document defaults and templates |

**Tooling**

| Command | Description |
| --- | --- |
| `auth login`, `auth status`, `auth logout` | authentication (OAuth module checklist or PAT) |
| `api METHOD /2.0/…` | raw authenticated request to any endpoint |
| `docs [command]` | compact LLM reference; per-command details on demand (`--full` for everything) |

Quote/order/invoice positions use compact specs:

```sh
bexio order create --contact-id 17 --title "Relaunch" \
    --position "type=custom,text=Consulting,amount=8,unit_price=150" \
    --position "type=article,article_id=5,amount=2"
```

Field flags map 1:1 to the API fields (`--name-1` → `name_1`,
`--contact-group-ids` → `contact_group_ids`, …), so payloads stay predictable
against [docs.bexio.com](https://docs.bexio.com). Command names use bexio's
documentation names rather than the internal resource paths, with the latter
kept as aliases: `quote` = `kb-offer`, `order` = `kb-order`,
`invoice` = `kb-invoice`, `delivery` = `kb-delivery`, `project` = `pr-project`.
Every read command supports `-o json` to print the raw API objects.

Search uses the API's criteria syntax via repeatable `--where` clauses:

```sh
bexio contact search --where mail~@acme.ch --where contact_type_id=1
bexio contact search --where "updated_at>2026-01-01" -o json
```

## Development

```sh
make build   # build bin/bexio
make test    # go test ./...
make lint    # golangci-lint run
```

Conventional Commits enforced in CI; releases are automated via
release-please + goreleaser. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE.md)

## Trademark notice

This is an independent, community-built tool. It is not affiliated with,
endorsed by, or supported by bexio AG. "bexio" is a trademark of bexio AG;
its use here is purely descriptive, to indicate which API this tool talks
to. Use of the bexio API is subject to bexio's
[terms and conditions](https://cdn.www.bexio.com/assets/content/documents/legal/bexio_marketplace_agb_EN.pdf).
