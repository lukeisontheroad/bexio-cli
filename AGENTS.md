# Agent instructions

Contributor guidance for AI coding agents lives in [CLAUDE.md](CLAUDE.md):
build/test/lint commands, architecture, verified bexio API facts (including
several places where the live API contradicts docs.bexio.com), and code
conventions. Read it before changing code.

Commit style is Conventional Commits — see [CONTRIBUTING.md](CONTRIBUTING.md);
commits drive the automated changelog and semantic version.

If you are an agent *using* the installed `bexio` CLI (rather than developing
it), run `bexio docs` — it prints the complete command reference in one page,
written for LLM consumption. A ready-made skill is in
[skills/bexio/SKILL.md](skills/bexio/SKILL.md).
