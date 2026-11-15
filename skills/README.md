# Agent skills

This directory ships **one** skill, `wlodev-go`. It covers everything a wlodev Go service must follow:
- project layout
- the `_oas/` OpenAPI structure and the `models.openapi.yml` registry
- the oapi-codegen (std-http + strict) setup and its quirks
- handlers, the database layer (goose, sqlx, dbmodels, queries), services and utils
- loading related data without JOINs (batch `…ByIDs` queries + shared loaders)
- Go writing style
- how to use this library (`postgres`, `pagination`, `ptr`, `publicid`, `zerolog`)

There is a single skill on purpose: a project can't install only half the rules and drift.

```
skills/wlodev-go/
├── SKILL.md        # core rules + routing table (always loaded)
├── references/     # detailed docs, loaded on demand
└── examples/       # a complete, consistent "widgets" domain: spec, migration, dbmodels, queries, service, handlers, main
```

## Install in a project
```sh
npx skills add wlodev-labs/go --skill wlodev-go
```
To pick an agent explicitly, add for example `-a claude-code`.

## Update
```sh
npx skills update
```
The skill tracks `main` of this repository. When conventions change, change them here, push, and run `npx skills update` in each project.

## Make sure the agent always uses it
Skills load automatically based on their description, but add this line to each project's `CLAUDE.md` / `AGENTS.md` to make it deterministic:

```md
Always load and follow the `wlodev-go` skill before writing or reviewing code in this repository.
```

## Changing the conventions
- Keep `SKILL.md` short: it holds the rules and points to the references. Put detail in `references/<topic>.md`.
- Every rule needs a ✅/❌ or a code example. Write quirks as symptom → cause → fix.
- Examples under `examples/` must stay consistent with each other (one `widgets` domain end to end). Go files use the `.go.txt` extension so they never compile into this module.
- When a lib API changes, update `references/lib.md` and the minimum version line in `SKILL.md` together.
