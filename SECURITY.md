# Security Policy

## Supported versions

Only the latest release receives security fixes.

## Reporting a vulnerability

Please do not open public issues for security problems.

Report vulnerabilities via
[GitHub private vulnerability reporting](https://github.com/lukeisontheroad/bexio-cli/security/advisories/new)
on this repository. You will receive a response within a few days.

## Scope notes

- The CLI stores tokens in `~/.config/bexio/config.yml` with `0600`
  permissions, or reads them from environment variables. Tokens are only
  ever sent to `api.bexio.com` (Authorization header) and, for OAuth
  refreshes, to `auth.bexio.com`.
- The OAuth client id and secret embedded in the binary are deliberately
  public: for native apps the client secret is not confidential
  (RFC 8252 §8.5). Security rests on the user consent screen, the
  localhost-only redirect URL whitelist, and PKCE — not on the secret.
- The OAuth redirect listener binds to localhost only, validates the
  `state` parameter, and uses PKCE (S256).
- `--verbose` logs request URLs (not the token) to stderr.
- Dependencies are scanned with govulncheck in CI and updated via Dependabot.
