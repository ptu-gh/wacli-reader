# Repository Guidelines — `wacli-reader`

`wacli-reader` is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli) (now developed at [`github.com/openclaw/wacli`](https://github.com/openclaw/wacli)), originally forked at upstream `v0.7.0` and most recently rebuilt on upstream `v0.26.0` (upstream `main` at `2108fe7`). It exposes only commands that read a `wacli.db` produced by the upstream writer. The fork is intended to be **rebuilt on upstream wacli periodically**, so changes to surviving files are kept narrow and surgical.

## Project Structure
- `cmd/wacli/`: CLI command wiring. Read-only surface: chats list/show, contacts search/show, groups list, groups participants list, messages list/search/starred/show/context/export/changes, calls list, polls list, poll show, history coverage, accounts list/show, store stats, doctor, docs, version. The source directory keeps its upstream name; the compiled binary is `wacli-reader`.
- `internal/app/`: collapsed `App` struct holding the read-only DB handle plus its `Options` (exposed via `DB()`/`StoreDir()`/`Version()`/`ReadOnly()`). Mirrors upstream's `Options.ReadOnly` knob for interface parity but hard-wires it on; `New()` rejects `ReadOnly=false`. Declares upstream's `LocalResolver` interface with no implementation (see "Known divergences").
- `internal/jid/`: **fork-only.** A whatsmeow-free replacement for the two identifier APIs upstream's read commands call — `go.mau.fi/whatsmeow/types` (`JID`, `ParseJID`, server constants) and `internal/wa.ParseUserOrJID`. Files taken from upstream import it under both names (`types "…/internal/jid"` and `wa "…/internal/jid"`) so their bodies stay identical to upstream's. Parsing and formatting were differentially fuzzed against whatsmeow; keep them exact. Extend it when upstream read code starts calling something new — never import whatsmeow instead.
- `internal/store/`: SQLite schema, migrations, FTS5 search, query layer. **Taken wholesale from upstream** — write helpers (`Upsert*`, `Mark*`, etc.) and migration code stay so upstream rebases apply cleanly. Production code reaches the package only through `store.OpenReadOnly`. Uses the sqlc-generated `internal/store/storedb/` package internally.
- `internal/store/sqlc/`, `internal/store/storedb/`, `sqlc.yaml`: sqlc input (`queries.sql`, `fts_stub.sql`) and the typed Go code generated from it. Regenerate with `pnpm generate:sqlc` (sqlc is run at a pinned version via `go run`, not as a `go.mod` `tool` dependency — upstream's `tool` line pulls ~60 modules into `go.mod`).
- `internal/config/`: store-dir and account resolution, **taken wholesale from upstream** (including the `SaveAccountsConfig` writer, dead here). Defaults resolve to the upstream writer's location — including its `config.yaml` `default_account` — so `wacli-reader` transparently sees the writer's data.
- `internal/out/`: JSON + table output helpers; all human text goes through here. Verbatim except for the omitted `events.go`.
- `internal/fsutil/`, `internal/pathutil/`, `internal/sqliteutil/`: small filesystem/path/sqlite helpers, verbatim.
- Tests sit next to the code they cover (`*_test.go`).

### Removed from upstream
- `internal/wa/` (whatsmeow client) — gone. The fork has no WhatsApp client and no network code.
- `internal/lock/` — gone. The reader does not acquire any lock.
- `internal/delegate/` (IPC hand-off of commands to a running `sync`) — gone.
- `internal/{linkpreview,resolve,syscontacts}/` — gone.
- `internal/app/` — everything except `app.go` (rewritten) is gone: sync, backfill, bootstrap, media, app state, chat state, contact save, presence, receipts, session resolver/state, webhooks, heartbeat, unread bookkeeping, and their tests.
- `internal/out/events.go` (NDJSON lifecycle events for `--events`) — gone.
- `cmd/wacli/`: every command file not listed under "Partially taken" or "Verbatim" below — `auth*`, `sync`, `send*`, `media*`, `presence`, `profile`, `channels`, `recipient`, `messages_{delete,edit,forward,purge,media}`, `contacts_{check,resolve,save,import_system}`, `groups_{admin,info_rename,invite_join,prune}`, `chats_cleanup`, `store_cleanup`, `storage_limits`, `read_file_limited`, `cgo_required`, `signal` — and their tests. `contacts resolve` is gone because its only data source is `session.db`'s LID map.
- Repo tooling the fork does not use: the docs site, release-linker and release scripts, Docker, crabbox, the `.agents/` skills, and upstream's extra workflows.

### Partially taken from upstream

Many upstream files mix read-only code with write/network code. Where the compiler catches a bad wholesale take (a dropped command constructor fails to link), the risk is low. The table below lists the cases where it does **not** — the fork ships only part of an upstream file, so a blanket "keep the deletion" on rebase would silently drop working read-only features instead of failing loudly. **Re-trim these; do not drop them and do not take them whole.** Every fork file here carries upstream's filename, so `git diff upstream/main -- <path>` is meaningful.

| File | Kept | Dropped |
|---|---|---|
| `cmd/wacli/accounts.go` | `accounts list`, `accounts show` | `accounts add`/`use`/`remove` |
| `cmd/wacli/chats.go` | `chats list`, `chats show`; `resolveStoredChats` and `mappedChatJIDs` reduced to upstream's no-`session.db` result | archive/pin/mute/mark-read/cleanup registrations; LID→phone display resolution |
| `cmd/wacli/chats_state.go` | `validateBoolFilter`, `boolFilter`, `chatFlagsString`, `formatMutedUntil` | `archive`/`pin`/`mute`/`unmute`/`mark-read` commands |
| `cmd/wacli/contact_read.go` | everything; `contactReadResolver` returns a nil resolver | the `session.db`-backed resolver |
| `cmd/wacli/contacts.go` | `contacts search`, `contacts show` | `resolve`, `check`, `refresh`, `import-system`, `save`, `alias`, `tags` |
| `cmd/wacli/groups.go` | `list`, `participants` registrations | every other group subcommand |
| `cmd/wacli/groups_participants.go` | `groups participants list` | `add`/`remove`/`promote`/`demote` |
| `cmd/wacli/groups_persist.go` | `canonicalCLIJID`, `groupKindLabel` | group persistence plumbing |
| `cmd/wacli/groups_refresh_list.go` | `groups list` (truncation warning as plain stderr) | `groups refresh`; the `--events` warning branch |
| `cmd/wacli/history.go` | `history coverage` + its renderers | `history fill`, `history backfill` |
| `cmd/wacli/messages.go` | read subcommand registrations | `delete`/`purge`/`revoke`/`edit`/`forward` |
| `cmd/wacli/messages_helpers.go` | `messageChatJIDFilter` (chat-name resolution, no PN/LID twin), `canonicalMessageFilterJID`, `jidStrings`, `getMessageByChatFilter`, `getMessageContextByChatFilter`, `isNoRows`; `resolveMessageSenderNames(InView)` as pass-throughs | LID sender resolution, `loadMessageMutationTarget` |
| `cmd/wacli/poll_cmd.go` | `polls list`, `poll show`, their renderers, `resolveVoterName`'s local-contact half | `poll vote`, `--pick`/`--to` recipient resolution |
| `cmd/wacli/store.go` | `store stats` registration | `store cleanup` |
| `cmd/wacli/root_test.go` | output-capture and config-isolation helpers, `resolveStoreDir` tests | tests of `--events`, `WACLI_READONLY`, `requireWritable` |

Fork-owned rewrites (upstream's file exists but the fork's content is its own): `cmd/wacli/root.go`, `cmd/wacli/doctor.go`, `cmd/wacli/doctor_test.go`, `cmd/wacli/main.go` (upstream's minus the whatsmeow device label), `cmd/wacli/version.go`, `cmd/wacli/messages_test.go`, `internal/app/app.go`.

Verbatim from upstream in `cmd/wacli`: `calls.go`, `docs.go`, `helpers.go`, `messages_changes.go`, `messages_format.go`, `messages_read.go`, `output_signals_{unix,windows}.go`, `store_stats.go`, `table.go`; `messages_pages.go` differs only in its import line. Tests: `account_home_regression_test.go`, `groups_participants_test.go`, `history_content_test.go`, `output_pipe_test.go`.

Fork-only files (no upstream counterpart, so they never conflict): `internal/jid/`, `internal/store/reader_schema.go` (+ test), `internal/store/db_readonly_test.go`, `internal/app/reader_app_test.go`, `cmd/wacli/readonly_guard_test.go`.

Fork-local deltas inside otherwise-verbatim copies (keep them when re-trimming):
- `sanitize()` is applied to `Kind`/`JID`/parent-JID fields in the `chats list`, `contacts search`, `groups list`, `history coverage`, and `accounts list` table renderers, to `USER JID`/`ROLE` in `groups participants list`, and to `STORE` in `doctor`. Upstream interpolates these raw.
- `resolveVoterName` keeps upstream's signature but returns early instead of falling back to the `session.db`-backed `LocalResolver`.

### Known divergences and their cost

Ranked by how much they cost on each rebase. Anyone touching this fork should know these exist before "fixing" them.

Total fork-side cost is roughly **300 lines** across `cmd/wacli` and `internal` — the lines a rebase has to re-apply — plus the fork-only files above, which never conflict. Everything else in a diff against upstream is a clean deletion ("keep the deletion") or a file upstream simply does not have.

1. **`cmd/wacli/root.go` (~100 lines)** — the deliberate concentration point. Holds the binary rename, `docsURL`, the TTY/`--no-json` default, the always-on `--read-only` (accepted so upstream command lines work; `=false` is an error), the `newRootCmd` split that lets `TestCommandSurfaceIsReadOnly` walk the tree, the schema-mismatch hint on `no such column`, and the fork's `newApp`/`closeApp`. `newApp` keeps **upstream's exact arity** — `newApp(ctx, flags, needLock, allowUnauthed) (*app.App, *readerLock, error)` — precisely so that every command's RunE prologue stays byte-identical to upstream. `readerLock` stands in for `*lock.Lock` and is always nil. Do not "simplify" this signature; the awkwardness is load-bearing.
2. **LID↔phone resolution is absent.** Upstream resolves `@lid` identities through `session.db` in `chats.go`, `contact_read.go`, `messages_helpers.go` and `poll_cmd.go`. The fork keeps those functions' signatures and reduces each to upstream's own no-`session.db` fallback (raw JIDs, nil resolver). Every upstream call site already guards the resolver with an `os.Stat(session.db)` check, so the *behaviour* is upstream's — the trims only remove the compile-time dependency. On rebase, re-cut these few functions; the rest of each file comes over unchanged.
3. **`cmd/wacli/doctor.go` (~65 lines)** — upstream's `doctorReport` carries lock, auth, connection, session-revocation and heartbeat fields that the reader cannot observe without `internal/lock` and `session.db`. The fork's report is a strict subset rather than one that reports `"authenticated": false` for a question it cannot answer. It adds the fork-local `schema_version` / `expected_schema_version` / `schema_warning` fields (see "Store-schema compatibility"). `LAST_SYNC` follows upstream: the writer's recorded last sync, not the newest message.
4. **`internal/app/app.go` (~35 lines)** — the collapsed `App` (66 lines vs upstream's 344, plus upstream's ~100 other files in the package) and the `ReadOnly=false` rejection. This is the fork's core design.
5. **`--timeout` is inert.** `withTimeout` builds a context that the store layer never observes, because no `internal/store` method accepts a `context.Context`. The flag is kept for upstream parity, passed to `newApp` exactly as upstream does, and documented as a no-op. Wiring it up is an upstream change, not a fork one.
6. **`sanitize()` on table-rendered JID/Kind fields** — see the note under "Partially taken from upstream". Proposed upstream (see below); until it lands, keep the fork's version.

`internal/store` (minus the fork-only files listed above), `internal/config`, `internal/fsutil`, `internal/pathutil`, `internal/sqliteutil`, `internal/out` (minus `events.go`), and `cmd/wacli/{calls,docs,helpers,messages_changes,messages_format,messages_read,store_stats,table}.go` are **byte-identical to upstream**. Keep them that way: take upstream's version wholesale on every rebase and do not hand-edit them.

## Key Architectural Facts
- **One database**: `wacli.db` only. `session.db` is never opened; no code path references it (`TestSourceHasNoWriteOrNetworkPaths` rejects any string literal naming it).
- **Read-only by construction**: `App.New` calls `store.OpenReadOnly`, which opens SQLite with `mode=ro&_query_only=1&readonly_shm=1` (plus `immutable=1` when no WAL/SHM/journal sidecars are present, so a read-only filesystem still works). The connection physically rejects writes with `attempt to write a readonly database`, and `readonly_shm=1` means even SQLite's shared-memory WAL index is opened read-only. There is no flag, env var, or config file that enables writes — `config.yaml` only selects *which* store to read. Schema migrations are not run.
- **The command surface is pinned.** `TestCommandSurfaceIsReadOnly` compares the full cobra tree against an explicit allowlist. A rebase that pulls in a new upstream subcommand fails it; add the command only after checking that it reads `wacli.db` and nothing else.
- **FTS5**: requires `-tags sqlite_fts5` at build time. The `wacli-reader` binary always builds with this tag.
- **Concurrency with upstream writer and other readers**: SQLite WAL (set by the upstream writer) lets any number of `wacli-reader` processes read while `wacli sync --follow` writes. The reader does not acquire the writer's `LOCK` file. Paged reads use upstream's `DB.ReadView`, a deferred read transaction, so a page and its cursor share one snapshot. `TestConcurrentReaderProcesses` runs reader processes against a continuously committing writer (`WACLI_READER_STRESS_ROUNDS=N` for a longer local stress run).
- **Store path precedence**: `--store` flag → `--account` → `WACLI_STORE_DIR` env → `default_account` from `config.yaml` → XDG `~/.local/state/wacli` on Linux (legacy `~/.wacli` fallback) → `~/.wacli` elsewhere. `resolveStoreDir` is taken verbatim from upstream so the reader and writer always agree on which store to use, including on multi-account installs.
- **Version**: the fork has its own version line (currently `0.0.4`) recorded in `cmd/wacli/root.go` and surfaced by `wacli-reader version` and `wacli-reader --version`. The upstream base is recorded in `CHANGELOG.md` only.
- **Repo name vs Go module path**: the GitHub repo is `ptu-gh/wacli-reader` but the Go module path is `github.com/openclaw/wacli` (upstream's). Intentional — keeping the module path aligned with upstream lets upstream code apply without sed-replacing every import line. Don't "fix" the mismatch.
- **Toolchain**: `go.mod` is upstream's (`go 1.27.0`, `toolchain go1.27.4`). The devcontainer sets `GOTOOLCHAIN=auto` so an older image Go fetches the pinned toolchain.

## Build, Test, and Development Commands
- Build: `pnpm build` — compiles with `-tags sqlite_fts5`, `CGO_ENABLED=1` and `CGO_CFLAGS=-Wno-error=missing-braces` (required for GCC 15+). Output: `dist/wacli-reader`.
- Run: `pnpm wacli-reader <args>` — rebuilds then runs. (Do not insert `--` before the args; pnpm swallows everything after it and you get the root help instead.)
- Test: `pnpm test` — runs `go test ./...` (plain) and `go test -tags sqlite_fts5 ./...` (FTS).
- Lint: `pnpm lint` — `go vet ./...`.
- Format fix: `pnpm format` — `gofmt -w .`.
- Format check: `pnpm format:check` — fails if any file would change.
- **Full gate** (must pass before every PR): `pnpm format:check && pnpm lint && pnpm test && pnpm build && git diff --check`.

## Coding Style
- Standard `gofmt` formatting; run `pnpm format` before committing.
- Output: send structured data to stdout (`--json` / table); send human hints, progress, and errors to stderr via `internal/out`.
- Prefer explicit error returns over panics; write short, early-return functions.
- No build-time CGO beyond sqlite3; keep the dependency tree minimal — five direct dependencies (`go-sqlite3`, `cobra`, `x/term`, `x/sys`, `yaml.v3`), all also upstream's. `x/sys` is there because upstream's verbatim `internal/out` uses it for Windows console handling. **Do not re-introduce `whatsmeow` or any WhatsApp client dependency.**

## Testing Guidelines
- Upstream's `internal/store` test suite is preserved and exercised by `pnpm test`. It uses the writable `store.Open` path with `Upsert*` helpers — that is intentional, so upstream changes rebase cleanly. Production code only ever calls `store.OpenReadOnly`.
- `cmd/wacli/readonly_guard_test.go` holds the fork's guarantees: pinned command surface, no forbidden imports or `session.db` literals, every read command leaves the store byte-identical, `--read-only=false` is rejected, `newApp` refuses the lock, and readers run concurrently with a live writer. Its `seedReaderStore`/`readSweep` helpers are the place to add coverage for a newly surfaced read command.
- Upstream `cmd/wacli` tests are taken verbatim when they compile against the trimmed package and pass; most do not, because they share helpers with write-command tests or need a fake WhatsApp client.
- New tests should sit next to the code they cover.
- FTS-sensitive tests must run under `-tags sqlite_fts5`; non-FTS path tests must also pass without the tag.

## Branches
- **`main`** mirrors upstream (`upstream/main`, the canonical wacli repo). It is kept fast-forwarded to upstream; no fork-local commits ever land on `main`.
- **`reader`** carries the fork's changes, as a small number of commits on top of `main`. `origin/reader` is the published fork branch.
- When upstream advances: `git fetch upstream && git checkout main && git merge --ff-only upstream/main`. Upstream merge work targets `reader`, not a side branch.

## Rebasing on upstream wacli

For a short upstream delta, rebase `reader` onto `main` and resolve conflicts. For a long one (months of upstream work), it is cheaper to **rebuild**: reset `reader` to `main`, then re-apply the fork using the previous `reader` as the specification — delete what "Removed from upstream" lists, re-trim the "Partially taken" files from upstream's current versions, restore the fork-owned and fork-only files, and re-derive the docs. Either way:

1. Resolve conflicts by **keeping the deletions** in `internal/wa/`, `internal/lock/`, `internal/delegate/`, and the deleted `cmd/wacli/*.go` and `internal/app/*.go` files. Anything upstream adds to those paths is intentionally not shipped here — **except** the files in "Partially taken from upstream" above, which must be re-trimmed rather than dropped or taken whole. Before blanket-keeping a deletion, grep the upstream file for a command that only touches `wacli.db`; upstream has form for parking read-only features in otherwise-network files (`groups participants list` lived in `groups_participants.go` next to the participant mutations).
2. **Triage new upstream commands.** `TestCommandSurfaceIsReadOnly` lists exactly what ships. For each new upstream subcommand, read its RunE: it qualifies only if it reaches nothing but `a.DB()` read methods (check the store method too — no `Exec`, no `Upsert*`/`Mark*`/`Set*`), passes `needLock=false`, and has no `requireWritable`, `EnsureAuthed`, `OpenWA`, `WA()`, delegate or `session.db` path. Then add it to the allowlist and to `readSweep`.
3. **Watch for read-only commands acquiring a session.db or whatsmeow dependency.** Upstream routes `chats`, `contacts`, `messages`, `calls`, and `polls` through `a.LocalResolver()` / `ReadOnlyResolver()` (LID↔phone mapping out of `session.db`) and through `whatsmeow/types` JID parsing. Re-cut the resolver calls to upstream's no-session fallback (divergence 2) and point `types`/`wa` imports at `internal/jid`. If upstream read code calls a `types`/`wa` identifier `internal/jid` lacks, add it there — matching whatsmeow's behaviour exactly — rather than importing whatsmeow.
4. Re-apply rename touchpoints if upstream changed them: `cobra.Command.Use` (`wacli-reader`), `SetVersionTemplate` template, the `version` variable and `docsURL` constant in `cmd/wacli/root.go`, the `version` subcommand printf in `cmd/wacli/version.go`, the binary name in `package.json` scripts and both `.goreleaser*.yaml` files, and the `name` field in `package.json`.
5. **Do not adopt upstream's `effectiveVersion()` / release-linker machinery.** Upstream ties `version` to an ldflag marker (`main.releaseLinkerSetting`) verified against a codesign designated requirement. The fork keeps a plain `var version` injected by `-X main.version`. Taking upstream's `root.go` version block without also taking its goreleaser ldflags makes released binaries print `invalid-release-linker-version`.
6. If upstream changes `store.Open`, `OpenReadOnly`, `init`, or migrations: keep upstream's version. The fork adds `reader_schema.go` alongside rather than editing them.
7. If upstream removes a write helper that `wacli-reader` does not call: take the removal.
8. `go.mod`: take upstream's, delete the `tool github.com/sqlc-dev/sqlc/cmd/sqlc` line, then `go mod tidy`. `whatsmeow`, `libsignal`, `protobuf`, `websocket` and the sqlc tree must drop out; if they do not, something still imports them. Keep `generate:sqlc`'s pinned sqlc version in step with upstream's.
9. After resolving conflicts, run the full gate locally, confirm `wacli-reader version` still prints the fork's version (upstream's own `version` command prints a bare number with no name prefix — that difference is deliberate), and update the "minimum writer version" in `README.md` and `docs/spec.md` if upstream added migrations.

### Config-file failure mode

`resolveStoreDir` is upstream's, so a malformed or invalid `~/.wacli/config.yaml` — bad YAML, an unknown field, a `default_account` naming an account that is not defined — makes **every** read command fail, even though the reader does not otherwise need that file. This is upstream's behaviour verbatim, and failing loudly beats silently reading the wrong store, but it is a way for a read-only tool to be taken down by a file it does not need. `--store DIR` bypasses config entirely and is the escape hatch.

### Store-schema compatibility

The fork never runs migrations, so it inherits whatever schema the upstream writer last applied, while its queries are upstream's current ones. This build reads schema **v33**, which upstream `wacli` 0.23 and later write. Against an older store, `messages` and `polls` commands fail with `no such column` / `no such table` (some others still work, and `messages changes` explains itself). The root error handler appends a hint to those SQLite errors, and `doctor` reports `schema_version` against `expected_schema_version` via the fork-only `internal/store/reader_schema.go`. The fix is to run the current upstream `wacli` once against the store; the fork deliberately does not carry compatibility shims, because per-column fallbacks would diverge from upstream's query text and make every future rebase conflict.

One edge case is upstream's, not the fork's: when no WAL/SHM sidecars exist at open time the reader uses `immutable=1`, which tells SQLite the file cannot change. If a writer starts *and checkpoints into the main file* during that one short-lived read command, the reader could see a torn page. Commands are short and a running `sync --follow` keeps the sidecars present, so this does not arise in the supported deployment.

## Upstream patches to propose

These would benefit upstream openclaw/wacli AND the fork. Worth filing PRs against upstream when there's time:

- **Sanitize JID and kind fields in table output.** Upstream routes most human output through `internal/out/sanitize.go`, but table renderers still interpolate DB values raw: `c.Kind`/`c.JID` in `chats list` (`cmd/wacli/chats.go`), `c.JID` in `contacts search`, `g.JID`/parent JID in `groups list`, `p.UserJID`/`p.Role` in `groups participants list`, `c.Kind` in `history coverage`, and `rep.StoreDir` in `doctor`. Every neighbouring column already goes through `tableCell`, which sanitizes — so this reads as an oversight. A `wacli.db` row containing an ANSI/OSC sequence in a JID emits raw escapes to the terminal, including window-title rewrites. Low exploitability upstream (JIDs are server-assigned, so writing one requires DB write access), but it matters for any consumer that treats `wacli.db` as untrusted input. The fork carries the one-word fixes; upstreaming them would remove the divergence.

- **Move the LID↔phone-JID mapping out of `session.db`.** Upstream's `internal/app/session_resolver.go` still opens `session.db` to map `@lid` identities to their `@s.whatsapp.net` counterparts, and `contacts resolve` exists only to expose that map. The mapping is identity metadata, not auth state — it belongs in `wacli.db` (a new `identity_aliases` table or similar). Upstream now rewrites historical LID-keyed rows to phone JIDs inside `wacli.db` (`internal/store/lid_migration.go`), which shrinks the problem, but rows the writer has not yet reconciled, and every resolver call site in divergence 2, still need the map. With it in `wacli.db`, the fork could drop divergence 2 entirely and ship `contacts resolve`. Until then, `chats show --jid <lid>` can return no rows for LID identities — `chats list --query "Name"` is the workaround documented in `SKILL.md`.

- **Expose the store's schema version.** A public `(*store.DB).SchemaVersion()` (and the newest version the build knows) would let upstream's own `doctor` report migration state, and would retire the fork's `internal/store/reader_schema.go`.

## Commit & Pull Request Guidelines
- Follow Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `security:`, `ci:` with an imperative summary.
- Keep commits focused; avoid bundling unrelated changes.
- PRs should state: what changed, why, how it was tested, and any new flags or env vars.
- Run the full gate locally before opening a PR; CI runs the same commands.

## Agent Notes
- This repo uses `AGENTS.md` as its agent-instruction source; `CLAUDE.md` is informational and may lag behind.
- The whole point of the fork is that the binary is unconditionally read-only — there is no flag to disable that. Pair the binary with read-only filesystem ACLs on `wacli.db` (and `wacli.db-wal`, `wacli.db-shm`) and deny access to `session.db` for full defence in depth.
- Prefer `--json` output for machine-readable parsing.
- Do not add dependencies or change build tooling without confirming with the maintainer. In particular, do not re-introduce `whatsmeow`, `internal/wa`, or any code that opens `session.db`.
