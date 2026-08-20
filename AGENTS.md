# Agent instructions

Contributor guidance for AI coding agents lives in [CLAUDE.md](CLAUDE.md):
build/test/lint commands, architecture, verified bexio API facts (including
several places where the live API contradicts docs.bexio.com), and code
conventions. Read it before changing code.

Commit style is Conventional Commits — see [CONTRIBUTING.md](CONTRIBUTING.md);
commits drive the automated changelog and semantic version.

If you are an agent *using* the installed `bexio` CLI (rather than developing
it), run `bexio docs` — a compact reference written for LLM consumption —
and `bexio docs <command>` for per-resource details on demand. A ready-made
skill is in [skills/bexio/SKILL.md](skills/bexio/SKILL.md).
