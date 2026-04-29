# Repository Guidelines — `wacli-reader`

`wacli-reader` is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli), forked at upstream `v0.7.0`. It exposes only commands that read a `wacli.db` produced by the upstream writer. The fork is intended to be **rebased periodically on upstream wacli**, so changes to surviving files are kept narrow and surgical.

## Project Structure
- `cmd/wacli/`: CLI command wiring (chats, contacts search/show, groups list, messages, doctor, version). The source directory keeps its upstream name; the compiled binary is `wacli-reader`.
- `internal/app/`: collapsed `App` struct that holds only the read-only DB handle.
- `internal/store/`: SQLite schema, migrations, FTS5 search, query layer. **Untouched from upstream** — write helpers (`Upsert*`, `Mark*`, etc.) and migration code stay so upstream rebases apply cleanly. Production code reaches the package only through `store.OpenReadOnly`.
- `internal/config/`: store-dir resolution (`WACLI_STORE_DIR` env → XDG state dir on Linux → `~/.wacli`). Defaults still resolve to the upstream writer's location so `wacli-reader` transparently sees the writer's data.
- `internal/out/`: JSON + table output helpers; all human text goes through here.
- `internal/fsutil/`, `internal/pathutil/`, `internal/sqliteutil/`: small filesystem/path/sqlite helpers.
- Tests sit next to the code they cover (`*_test.go`).

### Removed from upstream
- `internal/wa/` (whatsmeow client) — gone. The fork has no WhatsApp client and no network code.
- `internal/lock/` — gone. The reader does not acquire any lock.
- `cmd/wacli/{auth,sync,send,send_*,media,history,presence,groups_info_rename,groups_invite_join,groups_participants,groups_persist,signal}.go` and their tests — gone.
- `internal/app/{sync*,backfill*,bootstrap*,media*,jid,clock,fake_wa_test}.go` — gone.

## Key Architectural Facts
- **One database**: `wacli.db` only. `session.db` is never opened; the binary contains no code path that references it.
- **Read-only by construction**: `App.New` calls `store.OpenReadOnly`, which opens SQLite with `?mode=ro`. The connection physically rejects writes with `attempt to write a readonly database`. There is no flag, env var, or config file that enables writes. Schema migrations are not run.
- **FTS5**: requires `-tags sqlite_fts5` at build time. The `wacli-reader` binary always builds with this tag.
- **Concurrency with upstream writer**: SQLite WAL (set by the upstream writer) lets `wacli-reader` read while `wacli sync --follow` writes. The reader does not acquire the writer's `LOCK` file.
- **Store path precedence**: `--store` flag → `WACLI_STORE_DIR` env → XDG `~/.local/state/wacli` on Linux (legacy `~/.wacli` fallback) → `~/.wacli` elsewhere. Unchanged from upstream so the reader and writer share one store by default.
- **Version**: the fork has its own version line (currently `0.0.1`) recorded in `cmd/wacli/root.go` and surfaced by `wacli-reader version` and `wacli-reader --version`. The upstream fork point is recorded in `CHANGELOG.md` only.

## Build, Test, and Development Commands
- Build: `pnpm build` — compiles with `-tags sqlite_fts5` and `CGO_CFLAGS=-Wno-error=missing-braces` (required for GCC 15+). Output: `dist/wacli-reader`.
- Run: `pnpm wacli-reader -- <args>` — rebuilds then runs.
- Test: `pnpm test` — runs `go test ./...` (plain) and `go test -tags sqlite_fts5 ./...` (FTS).
- Lint: `pnpm lint` — `go vet ./...`.
- Format fix: `pnpm format` — `gofmt -w .`.
- Format check: `pnpm format:check` — fails if any file would change.
- **Full gate** (must pass before every PR): `pnpm format:check && pnpm lint && pnpm test && pnpm build && git diff --check`.

## Coding Style
- Standard `gofmt` formatting; run `pnpm format` before committing.
- Output: send structured data to stdout (`--json` / table); send human hints, progress, and errors to stderr via `internal/out`.
- Prefer explicit error returns over panics; write short, early-return functions.
- No build-time CGO beyond sqlite3; keep the dependency tree minimal. **Do not re-introduce `whatsmeow` or any WhatsApp client dependency.**

## Testing Guidelines
- Upstream's `internal/store` test suite is preserved and exercised by `pnpm test`. It uses the writable `store.Open` path with `Upsert*` helpers — that is intentional, so upstream changes rebase cleanly. Production code only ever calls `store.OpenReadOnly`.
- New tests should sit next to the code they cover.
- FTS-sensitive tests must run under `-tags sqlite_fts5`; non-FTS path tests must also pass without the tag.

## Rebasing on upstream wacli
When pulling upstream changes:
1. Resolve conflicts by **keeping the deletions** in `internal/wa/`, `internal/lock/`, and the deleted `cmd/wacli/*.go` and `internal/app/*.go` files. Anything upstream adds to those paths is intentionally not shipped here.
2. Re-apply rename touchpoints if upstream changed them: `cobra.Command.Use` (`wacli-reader`), `SetVersionTemplate` template, the `version` constant in `cmd/wacli/root.go`, the `version` subcommand printf in `cmd/wacli/version.go`, the binary name in `package.json` scripts and both `.goreleaser*.yaml` files, and the `name` field in `package.json`.
3. If upstream changes `store.Open`, `init`, or migrations: keep upstream's version. The fork's `OpenReadOnly` is purely additive and does not depend on the migration path.
4. If upstream removes a write helper that `wacli-reader` does not call: take the removal.
5. After resolving conflicts, run the full gate locally and confirm `wacli-reader version` still prints `0.0.1` (or whatever the fork's current version is).

## Commit & Pull Request Guidelines
- Follow Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `security:`, `ci:` with an imperative summary.
- Keep commits focused; avoid bundling unrelated changes.
- PRs should state: what changed, why, how it was tested, and any new flags or env vars.
- Run the full gate locally before opening a PR; CI runs the same commands.

## Agent Notes
- This repo uses `AGENTS.md` as its agent-instruction source; `CLAUDE.md` is informational and may lag behind.
- The whole point of the fork is that the binary is unconditionally read-only — there is no flag to disable that. Pair the binary with read-only filesystem ACLs on `wacli.db` (and `wacli.db-wal`) and deny access to `session.db` for full defence in depth.
- Prefer `--json` output for machine-readable parsing.
- Do not add dependencies or change build tooling without confirming with the maintainer. In particular, do not re-introduce `whatsmeow`, `internal/wa`, or any code that opens `session.db`.
