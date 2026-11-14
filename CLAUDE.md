# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository. `AGENTS.md` is the canonical agent-instruction source for `wacli-reader` — start there for the architectural facts and rebase policy. This file is a quick orientation.

## Project

**`wacli-reader`** is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli) (now developed at [`github.com/openclaw/wacli`](https://github.com/openclaw/wacli)), originally forked at upstream `v0.7.0` and most recently rebuilt on upstream `v0.26.0` (upstream `main` at `2108fe7`). It exposes only commands that **read** a `wacli.db` produced by the upstream writer; it never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store read-only so writes are rejected at the SQLite-driver layer.

The fork is rebuilt on upstream `wacli` periodically, so changes to surviving files (especially `internal/store/` and its tests) are kept narrow and surgical.

## Branches

- **`main`** mirrors upstream (`upstream/main`, the canonical wacli repo). It is kept fast-forwarded to upstream; no fork-local commits ever land on `main`.
- **`reader`** carries the fork's changes as a small number of commits on top of `main`. `origin/reader` is the published fork branch. Upstream merge work targets `reader`, not a side branch.

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

`go.mod` pins `toolchain go1.27.4`; with an older Go, set `GOTOOLCHAIN=auto` (the devcontainer does).

To run a single test:
```bash
go test ./internal/store -run TestMessageUpsertIdempotent
go test -tags sqlite_fts5 ./internal/store -run TestSearch
go test ./cmd/wacli -run TestDoctor -v
go test ./cmd/wacli -run 'TestCommandSurface|TestSource|TestReadCommands|TestConcurrent' -v   # the fork's guarantees
```

## Architecture

The codebase is split between the CLI layer (`cmd/wacli/`, still named after upstream for rebase ergonomics) and internal packages (`internal/`).

### Packages

**`internal/app`** — Trivial wrapper. The `App` struct holds a `*store.DB` plus its `Options`, exposed via `DB()`, `StoreDir()` and `Version()`. It also declares upstream's `LocalResolver` interface, never implemented here. `New(opts)` opens `wacli.db` read-only via `store.OpenReadOnly`. There is no WhatsApp client, no event loop, no sync code. `Options.ReadOnly` mirrors upstream's flag but is hard-wired on; `New` rejects `ReadOnly=false`.

**`internal/jid`** — Fork-only, whatsmeow-free JID parsing. Upstream read code imports it as both `types` and `wa`, so its bodies stay identical to upstream's.

**`internal/store`** — SQLite layer. **Taken wholesale from upstream** so rebases apply cleanly. Both `Open` and `OpenReadOnly` come from upstream (convergent fork). Production code reaches the package only through `OpenReadOnly`. The package's `Upsert*`, `Mark*`, `Set*`, `Add*`, `Remove*` write helpers and migration code remain dead-but-present so upstream changes don't conflict. Internally uses the sqlc-generated `internal/store/storedb/` package; regenerate with `pnpm generate:sqlc`. The only fork additions are new files: `reader_schema.go` (schema version for `doctor`) and `db_readonly_test.go`.

**`internal/config`** — Store-dir and named-account resolution. Byte-identical to upstream, so the reader and writer agree on the store dir — including `config.yaml`'s `default_account` — by default.

**`internal/out`** — Output formatting: JSON and table renderers, shared error helpers. Verbatim from upstream minus `events.go`.

**`internal/fsutil`, `internal/pathutil`, `internal/sqliteutil`** — Small helpers; byte-identical to upstream.

**`cmd/wacli`** — Cobra CLI commands. `root.go` wires up `newApp(ctx, flags, needLock, allowUnauthed)` — upstream's signature, kept so command bodies stay identical — and registers the surviving sub-commands: `accounts`, `calls`, `chats`, `contacts`, `docs`, `doctor`, `groups`, `history`, `messages`, `poll`, `polls`, `store`, `version`. The cobra `Use` field is `wacli-reader`.

Many files (`accounts.go`, `chats.go`, `chats_state.go`, `contact_read.go`, `contacts.go`, `groups*.go`, `history.go`, `messages.go`, `messages_helpers.go`, `poll_cmd.go`, `store.go`) hold read-only code carved out of upstream files that otherwise contain write commands — see the "Partially taken from upstream" table in `AGENTS.md` before rebasing them. All of them use upstream's filename, so `git diff upstream/main -- <path>` shows real changes. `messages_read.go`, `messages_changes.go`, `calls.go` and the formatting helpers are verbatim.

`readonly_guard_test.go` pins the exact command list and checks that no source imports a WhatsApp client, lock, delegate or network package or names `session.db`; a rebase that pulls in a write command fails it.

### Removed from upstream

- `internal/wa/` (whatsmeow client) — gone. No network code, no `session.db` access.
- `internal/lock/` — gone. The reader does not acquire any lock.
- `internal/delegate/` — gone. Nothing is handed to a running writer.
- All upstream commands that send, sync, authenticate, download media, mutate groups, or write contact aliases/tags. The corresponding tests are also gone.

### Key Data Flows

- **Search**: `wacli-reader messages search` → `messageChatJIDFilter` (number/JID/local name → JID) → `DB.ReadView` → `store.SearchMessagesPage()` (FTS5 `MATCH` or `LIKE` fallback) [+ `MessageContextInChats` per hit] → formatted output with a `page` cursor.
- **List / export**: `wacli-reader messages list|export` → `DB.ReadView` → `store.ListMessagesPage()` → JSON envelope (stdout or `--output`) with a `page` cursor.
- **Other lists**: `messages starred` / `chats list` / `contacts search` / `groups list` / `groups participants list` / `calls list` / `polls list` → corresponding `store.List*` / `Search*` / `Get*` query → formatted output.
- **Changes**: `wacli-reader messages changes` → `store.MessageChanges()` → content-free change feed plus an opaque checkpoint token.
- **Doctor**: `wacli-reader doctor` → `store.SchemaVersion()` + `store.Stats()` + `store.LastSuccessfulSync()` + `store.HasFTS()` → formatted report.
- **Store stats**: `wacli-reader store stats` → `store.CountChats/Groups/LeftGroups/Messages` → counts table.
- **Coverage**: `wacli-reader history coverage` → `store.ListHistoryCoverage()` → per-chat archive coverage table.

### Build Tag: `sqlite_fts5`

The production binary always uses `-tags sqlite_fts5`. FTS5-specific code is guarded by this tag. Tests run both with and without it (`pnpm test:go` vs `pnpm test:fts`).

## Concurrency with upstream writer

`wacli-reader` is designed to run alongside an upstream `wacli sync --follow` that owns the same `wacli.db`, with any number of reader processes at once. SQLite WAL mode (set by the writer) lets each reader see consistent snapshots without blocking the writer; the reader opens with `mode=ro&_query_only=1&readonly_shm=1` and does not acquire the writer's `LOCK` file. `TestConcurrentReaderProcesses` exercises this.

The reader never migrates. It needs a store written by upstream `wacli` 0.23+ (schema v33); `doctor` reports a mismatch.
