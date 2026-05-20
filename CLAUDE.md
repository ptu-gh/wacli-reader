# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository. `AGENTS.md` is the canonical agent-instruction source for `wacli-reader` — start there for the architectural facts and rebase policy. This file is a quick orientation.

## Project

**`wacli-reader`** is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli) (forked at upstream `v0.7.0`). It exposes only commands that **read** a `wacli.db` produced by the upstream writer; it never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store with `mode=ro` so writes are rejected at the SQLite-driver layer.

The fork is rebased periodically on upstream `wacli`, so changes to surviving files (especially `internal/store/` and its tests) are kept narrow and surgical.

## Branches

- **`main`** mirrors upstream (`upstream/main`, the canonical wacli repo). It is kept fast-forwarded to upstream; no fork-local commits ever land on `main`.
- **`reader`** carries the fork's changes — the original fork commit, ongoing maintenance, and upstream merge passes. `origin/reader` is the published fork branch. Upstream merge work targets `reader`, not a side branch.

## Commands

All build automation runs through pnpm, which delegates to Go tooling.

```bash
pnpm build          # produces ./dist/wacli-reader (builds with -tags sqlite_fts5)
pnpm lint           # go vet ./...
pnpm format:check   # gofmt -l . (check only)
pnpm format         # gofmt -w . (auto-fix)
pnpm test           # runs both test suites below
pnpm test:go        # go test ./...
pnpm test:fts       # go test -tags sqlite_fts5 ./...
```

To run a single test:
```bash
go test ./internal/store -run TestMessageUpsertIdempotent
go test -tags sqlite_fts5 ./internal/store -run TestSearch
go test ./cmd/wacli -run TestDoctor -v
```

## Architecture

The codebase is split between the CLI layer (`cmd/wacli/`, still named after upstream for rebase ergonomics) and internal packages (`internal/`).

### Packages

**`internal/app`** — Trivial wrapper. The `App` struct holds only a `*store.DB`. `New(opts)` opens `wacli.db` read-only via `store.OpenReadOnly`. There is no WhatsApp client, no event loop, no sync code.

**`internal/store`** — SQLite layer. **Untouched from upstream** so rebases apply cleanly. The fork adds one new function — `OpenReadOnly(path)` — alongside upstream's `Open`. Production code reaches the package only through `OpenReadOnly`. The package's `Upsert*`, `Mark*`, `Set*`, `Add*`, `Remove*` write helpers and migration code remain dead-but-present so upstream changes don't conflict.

**`internal/config`** — Resolves XDG-compliant data/state directories for the store path. Unchanged from upstream so the reader and writer share one store dir by default.

**`internal/out`** — Output formatting: JSON and table renderers, shared error helpers.

**`internal/fsutil`, `internal/pathutil`, `internal/sqliteutil`** — Small helpers; mostly unchanged.

**`cmd/wacli`** — Cobra CLI commands. `root.go` wires up `newApp(flags)` and registers the surviving sub-commands: `chats`, `contacts`, `doctor`, `groups`, `messages`, `version`. The cobra `Use` field is `wacli-reader`.

### Removed from upstream

- `internal/wa/` (whatsmeow client) — gone. No network code, no `session.db` access.
- `internal/lock/` — gone. The reader does not acquire any lock.
- All upstream commands that send, sync, authenticate, download media, mutate groups, or write contact aliases/tags. The corresponding tests are also gone.

### Key Data Flows

- **Search**: `wacli-reader messages search` → `store.SearchMessages()` → FTS5 `MATCH` or `LIKE` fallback → formatted output.
- **List**: `wacli-reader messages list` / `chats list` / `contacts search` / `groups list` → corresponding `store.List*` / `Search*` / `Get*` query → formatted output.
- **Doctor**: `wacli-reader doctor` → `store.Stats()` + `store.HasFTS()` → formatted report.

### Build Tag: `sqlite_fts5`

The production binary always uses `-tags sqlite_fts5`. FTS5-specific code is guarded by this tag. Tests run both with and without it (`pnpm test:go` vs `pnpm test:fts`).

## Concurrency with upstream writer

`wacli-reader` is designed to run alongside an upstream `wacli sync --follow` that owns the same `wacli.db`. SQLite WAL mode (set by the writer) lets the reader see consistent snapshots without blocking the writer. The reader does not acquire the writer's `LOCK` file.
