# Contributing

## Commit style

This repo uses [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
Commit messages drive the changelog and the semantic version — write them for
the release notes, not for yourself.

```
<type>(<optional scope>): <description>

[optional body]

[optional footer]
```

### Types and their semver effect

| Type | Meaning | Version bump |
|---|---|---|
| `feat` | new user-facing feature | **minor** |
| `fix` | bug fix | **patch** |
| `perf` | performance improvement | patch |
| `refactor` | code change, no behavior change | none |
| `docs` | documentation only | none |
| `test` | tests only | none |
| `build` | build system, dependencies | none |
| `ci` | CI configuration | none |
| `chore` | maintenance, tooling | none |

**Breaking changes**: append `!` after the type (`feat!: ...`) or add a
`BREAKING CHANGE:` footer — either forces a **major** bump.

### Examples

```
feat(contact): add --archived to list and search
fix(auth): refresh rotated OAuth tokens before expiry
feat!: rename --output values from yaml to json
docs: document the module scope selection in README
```

Scopes are optional; use the command or package name when it helps
(`contact`, `kb-invoice`, `auth`, `api`, `config`, `output`).

### Rules

- Imperative mood, lower case, no trailing period: `add X`, not `Added X.`
- The description completes: "this commit will …"
- One logical change per commit; split unrelated changes.

Commits are linted in CI (commitlint, PRs only); non-conforming messages fail
the build.

## Releases

Releasing is automated, no manual version picking:

1. [release-please](https://github.com/googleapis/release-please) watches
   `main`, collects conventional commits, and maintains a release PR with the
   next semver version and a generated `CHANGELOG.md`.
2. Merging that PR creates the `v*` tag.
3. The tag triggers goreleaser: binaries, GitHub release (notes grouped by
   commit type), and the Homebrew formula update.

## Development

```sh
make build   # build bin/bexio
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint run
```

All three of vet, test, and lint must pass; CI enforces them plus govulncheck.

Adding a new API module: create `internal/api/<module>.go` and
`internal/cmd/<module>.go`, register the command with
`func init() { registerModule(newXxxCmd) }`, and extend `auth.Modules` in
`internal/auth/scopes.go` if the module needs new OAuth scopes. See
[CLAUDE.md](CLAUDE.md) for the conventions and verified API facts.
