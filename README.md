# bexio-cli

Command-line client for the [bexio](https://www.bexio.com) business software:
contacts, quotes, sales orders, invoices, items & stock, projects &
timesheets, notes & tasks, and master data. Single static Go binary, no
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
`BEXIO_READ_ONLY=1`. Handy for AI agents that only need lookups.

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

| Command | Description |
| --- | --- |
| `bexio contact …` | contacts (delete archives, `restore` brings back), `contact address …` for additional addresses |
| `bexio contact-group / contact-relation / contact-sector …` | contact master data |
| `bexio kb-offer …` (`quote`) | quotes: CRUD, pdf, issue/accept/reject/send/copy, convert via `order`/`invoice` |
| `bexio kb-order …` (`order`) | sales orders: CRUD, pdf, positions, repetition, convert via `invoice`/`delivery` |
| `bexio kb-invoice …` (`invoice`) | invoices: CRUD, pdf, lifecycle, `payment …`, `reminder …` |
| `bexio kb-delivery …` (`delivery`) | deliveries: list/view/issue (issue books stock) |
| `bexio article …` (`item`) | items/products, plus `stock` / `stock-area` lookups |
| `bexio pr-project …` (`project`) | projects incl. `milestone`/`package`, archive/reactivate |
| `bexio timesheet …` / `bexio client-service …` | time tracking + business activities |
| `bexio note …` / `bexio task …` / `bexio comment …` | notes, tasks, comments on kb documents |
| `bexio country/language/salutation/title/unit/payment-type …` | 2.0 lookups |
| `bexio currency/tax/bank-account/user/company-profile/permission …` | 3.0 master data |
| `bexio api METHOD /2.0/…` | raw authenticated request to any endpoint |
| `bexio auth login/status/logout` | authentication (OAuth module checklist or PAT) |
| `bexio docs [command]` | compact LLM reference; per-command details on demand (`--full` for everything) |

Quote/order/invoice positions use compact specs:

```sh
bexio kb-order create --contact-id 17 --title "Relaunch" \
    --position "type=custom,text=Consulting,amount=8,unit_price=150" \
    --position "type=article,article_id=5,amount=2"
```

The CLI mirrors the bexio API scheme: resource names and field flags map 1:1
to the API (`--name-1` → `name_1`, `contact-group` → `contact_group`, …).
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
