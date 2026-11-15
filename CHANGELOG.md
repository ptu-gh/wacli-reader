# Changelog

All notable changes to `wacli-reader` will be documented in this file. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
`wacli-reader` carries its own version history, independent of the upstream
[`wacli`](https://github.com/steipete/wacli) version it was forked from.

## [Unreleased]

### Changed

- `SKILL.md` is about 40% shorter and tuned against a local 27B model on real questions. Answers are right more often and need fewer tool calls.
  - Reactions are marked as reactions, so they are no longer reported as the last message.
  - Media is copied into `/tmp/openclaw/wa/` and attached with a `MEDIA:` line. The copy stays until the next media request, so it still exists when the reply is delivered. The skill no longer points to the `message` tool.
  - Photos are viewed through OpenClaw's `view_image` action.
  - The task table gains rows for the last photo from a person and for a topic with the replies around it. The worked example is gone.

## [0.0.4] — 2026-11-14 — rebuild on upstream openclaw/wacli v0.26.0

Rebuilds the fork on upstream [`openclaw/wacli`](https://github.com/openclaw/wacli) `v0.26.0` (upstream `main` at `2108fe7`), 182 upstream commits past 0.0.3's base. The fork was re-applied on a fresh upstream base rather than replayed commit by commit. It remains read-only by construction; `whatsmeow` is still absent from the module graph and the binary never opens `session.db`.

### Breaking

- **Requires a store written by upstream `wacli` 0.23 or newer (schema v33).** The reader never migrates. Against an older store, `messages` and `polls` commands fail with `no such column` / `no such table`; the error now says why, and `doctor` shows the store's schema version next to the expected one. Run a current upstream `wacli` once against the store to upgrade it.
- Builds need Go 1.27.4 (upstream's `go.mod` toolchain pin). The devcontainer sets `GOTOOLCHAIN=auto`.

### Added

- `messages changes` — a content-free feed of message upserts, deletions and identity moves since a checkpoint (`--checkpoint`, `--since TOKEN|beginning`, `--limit`), for incremental consumers.
- `groups participants list --jid GROUP` — the last synced participant snapshot of a group.
- `messages list/search/export --cursor` — paged reads. Each page reports `page.has_more` / `page.next_cursor` and the writer's last sync (`source`); a page and its cursor come from one SQLite snapshot.
- `messages search --context-before N --context-after N` — neighbouring messages for each hit, in one call.
- `--chat` / `--sender` / `--from` accept a chat or contact's exact local name as well as a JID or phone number.
- Message JSON gains `DeliveredTo` / `ReadBy` receipt counts and `Edited`; the `messages show` table adds deletion time and reason and payload-purge time (from upstream).
- `doctor` reports `schema_version` and `expected_schema_version`, and warns when the store is older than the reader.
- `--read-only` is accepted (always on), so command lines written for upstream `wacli` work unchanged; `--read-only=false` is an error.
- Piping output into a reader that exits early (`| head`) no longer prints a broken-pipe error (from upstream).

### Changed

- `doctor`'s `LAST_SYNC` is the writer's recorded last successful sync rather than the newest message's timestamp (from upstream).
- Store opens add `readonly_shm=1` (from upstream): the reader opens even SQLite's shared-memory WAL index read-only.
- FTS detection on open reads one row instead of counting the whole index (from upstream), so every command starts faster on large archives.
- `groups list` warns on stderr when `--limit` truncates the result (from upstream).
- `SKILL.md` is rewritten for the 0.26 command surface and for smaller local models: fixed rules, a find-the-chat-first procedure, a task-to-command table with result paths, exact field names, an error table keyed on error text, and a worked example.

### Security

- `sanitize()` now also covers the `USER JID` and `ROLE` columns of the new `groups participants list` table.
- New guard tests pin the command surface, reject source that imports a WhatsApp client, lock, delegate or HTTP package or names `session.db`, check that every read command leaves the store byte-identical, and run reader processes concurrently with a live writer.

### Removed

- `whatsmeow`, `libsignal`, `protobuf`, `websocket` and sqlc's module tree are gone from `go.mod`; sqlc runs at a pinned version through `go run` instead of a `go.mod` `tool` line. Direct dependencies: `go-sqlite3`, `cobra`, `x/term`, `x/sys` (needed by upstream's Windows console handling in `internal/out`), `yaml.v3`.

## [0.0.3] — 2026-07-18 — track upstream openclaw/wacli to v0.13.1-dev

Rebases the fork onto upstream [`openclaw/wacli`](https://github.com/openclaw/wacli) `0.13.1-dev`. The fork remains read-only by construction; nothing from upstream's write paths is shipped, and `whatsmeow` is still absent from the module graph.

### Added

- `history coverage` — local archive coverage per chat, with `--chat`, `--query`, `--kind`, `--limit`, `--include-blocked`, and `--only-actionable`.
- `docs` — prints the fork's documentation URL.
- `accounts list` / `accounts show NAME` — inspect the named accounts in `config.yaml`.
- `--account NAME` — read a named account's store. Mutually exclusive with `--store`.
- `poll show` — voter names, resolved from the local contacts table.
- `chats list` — `--archived`/`--pinned`/`--muted`/`--unread` filters (each with a `--no-` counterpart) and a `FLAGS` column.
- `chats show` — `Archived`, `Pinned`, `Muted`, `Muted until`, `Unread`, `Unread count`.
- `groups list` — `TYPE` (group/subgroup/community) and `PARENT` columns.
- `contacts show` — `System Name`.
- `messages show` — quoted message and sender, media caption, filename, MIME type, download state, forwarding metadata, starred state, deletion tombstones, and a `Raw text` section when the raw text differs from the display text.

### Security

- All human-readable output now routes through `out.SanitizeHuman` / `out.SanitizeBody`, which strip ANSI/OSC terminal escape sequences from attacker-controlled message content. Previously a crafted push name or message body could emit raw escapes into the reader's terminal.
- Additionally sanitizes the `Kind`/`JID` fields in the `chats list`, `contacts search`, `groups list`, `history coverage`, and `accounts list` tables, and `STORE` in `doctor`. Upstream interpolates these raw; a JID containing an OSC sequence could rewrite the terminal's window title. Fork-local until upstreamed.
- Migration `PRAGMA table_info` calls now go through an identifier whitelist instead of string concatenation (from upstream).
- SQLite URI construction moved to `sqliteutil.FileURI` (from upstream), replacing the fork's local `fmt.Sprintf`.

### Fixed

- Store resolution now honours `config.yaml`'s `default_account`, matching upstream. Previously, on a multi-account install, `wacli` read the account's store while `wacli-reader` silently read `~/.wacli` — a wrong answer with no error.
- `doctor` no longer reports a clean bill of health for a store that opens but fails to query — the underlying error now surfaces as `store_error`.
- `internal/pathutil/` was dead but had drifted from upstream; taken verbatim, picking up `#` sanitization and the `.` → `unknown`/`file` guards.

### Changed

- Output now defaults to JSON when stdout is not a TTY and to a table when it is; `--no-json` forces the table. (Carried over from 0.0.2's development, previously undocumented.)
- `--timeout` is documented as the no-op it is: the store layer takes no context, so nothing observes the deadline.
- `internal/store/` and `internal/sqliteutil/` taken wholesale from upstream. Schema migrations v18–v20 add `chats.unread_count`, `messages.quoted_msg_id`, `messages.quoted_sender_jid`, and `messages.media_unavailable_at`.
- `messages` output prefers the stored sender name over a bare JID.
- Shared dependencies aligned with upstream: `go-sqlite3` 1.14.48, `x/term` 0.45.0, `x/sys` 0.47.0, Go 1.25.12. Adds `gopkg.in/yaml.v3` (upstream's, pure Go) for account config.
- `internal/config/`, `internal/fsutil/`, `internal/pathutil/` and `internal/sqliteutil/` are now byte-identical to upstream, and `newApp`/`closeApp` keep upstream's signature so command bodies match upstream line for line. Roughly halves the fork-local code a rebase has to re-apply.

### Compatibility

- Because the reader never migrates, this build requires a `wacli.db` that a `wacli` of 0.10 or later has written. Against an older store, queries selecting `unread_count` or the quoted-message columns fail with `no such column`. Run the current upstream `wacli` once to migrate.

### Not ported (deliberately)

- Upstream now resolves LID↔phone identities through `session.db` inside `chats`, `messages`, `calls`, and `polls`. The fork does not open `session.db`, so those commands continue to show raw JIDs for LID identities.
- `doctor` authentication fields (`session.db`), `lock.Probe` (`internal/lock`), `accounts add`/`use`/`remove`, and the `--pick` recipient-resolution flags.

## [0.0.2] — 2026-05-20 — track upstream openclaw/wacli to v0.9.2

Rebases the fork onto upstream [`openclaw/wacli`](https://github.com/openclaw/wacli) `v0.9.2`. The fork remains read-only by construction; nothing from upstream's write paths is shipped.

### Changed

- Go module path migrated from `github.com/steipete/wacli` to `github.com/openclaw/wacli` to match the upstream rename. Doc URLs that reference `https://github.com/steipete/wacli` are kept as historical fork-attribution.
- `internal/store/` taken wholesale from upstream — schema, migrations, FTS, and query layer are now identical to upstream `v0.9.2`. Schema migrations add tables for calls, polls, starred messages, status messages, history coverage, delete-for-me tombstones, structured reactions, forwarded-message metadata, interactive buttons, and group community hierarchy.
- `internal/store` now uses sqlc-generated typed queries internally (`internal/store/storedb/`); regenerate with `pnpm generate:sqlc`.
- `OpenReadOnly` adopts upstream's convergent implementation: `mode=ro&_query_only=1`, with `immutable=1` added only when no WAL/SHM/journal sidecars exist next to the database. This lets concurrent upstream-writer WAL commits remain visible to the reader, while still working on truly read-only filesystems.
- `internal/app/Options.ReadOnly` field added for interface parity with upstream's `app.Options`. `wacli-reader` hard-wires it on; `New()` rejects `ReadOnly=false` as defence in depth.
- Table-output truncation is now rune-aware (matches upstream).

### Added — new read-only sub-commands

- `messages starred [filters]` — list starred messages by stored star time.
- `messages export [--chat] [--limit] [--after] [--before] [--output]` — export messages as a JSON envelope, oldest first.
- `calls list [filters]` — list stored call events.
- `polls list [--chat] [--limit]` — list polls stored locally.
- `poll show --chat --id` — show one poll's question, options, per-option aggregate counts, and per-voter selections (read-only; no `poll vote`).
- `store stats` — row counts (chats, groups, left groups, messages).

### Added — new filters on existing commands

- `messages list` gained `--forwarded` and `--starred`.
- `messages search` gained `--forwarded` and `--starred`.

### Not brought in from upstream

- `auth`, `accounts`, `send` (text/file/voice/poll/status/sticker/react/mentions/link-previews), `media download`, `sync`, `history`, `presence`, `profile`, `contacts import-system`, `chats archive|unarchive|pin|unpin|mute|unmute|mark-read|mark-unread|cleanup`, `channels` (live fetch/join/leave), `poll vote`, `messages edit|delete`, `groups info|rename|leave|participants|invite|join|prune`, `store cleanup` — all of these are write-path commands or require an authenticated session.
- `internal/app/session_resolver.go` — opens `session.db`, which violates the fork's no-session-db invariant.
- NDJSON sync lifecycle events / sync status / sync limits / webhook scaffolding — all sync-side.
- `docs/` hosted-site content — every upstream page references write commands and would need rewriting; deferred to its own pass.

## [0.0.1] — initial fork from upstream wacli v0.7.0

`wacli-reader` is a heavily trimmed, agent-safe fork of
[`wacli`](https://github.com/steipete/wacli), forked at upstream version
`v0.7.0`. It exposes only commands that read a local `wacli.db` produced by
the upstream writer.

### Removed (relative to upstream wacli v0.7.0)

- All commands that talk to WhatsApp: `auth`, `auth status`, `auth logout`,
  `sync`, `send text`, `send file`, `send react`, `media download`,
  `history backfill`, `presence typing`, `presence paused`,
  `contacts refresh`, `groups refresh`, `groups info`, `groups rename`,
  `groups leave`, `groups participants` (add/remove/promote/demote),
  `groups invite link` (get/revoke), `groups join`.
- All commands that write the local store, even without network access:
  `contacts alias` (set/rm), `contacts tags` (add/rm).
- `--read-only` flag and `WACLI_READONLY` env var (the binary is now
  unconditionally read-only at the SQLite-driver level).
- `--lock-wait` flag and the file-based store lock (`internal/lock`); the
  reader does not acquire it and does not need it.
- `WACLI_DEVICE_LABEL` and `WACLI_DEVICE_PLATFORM` env vars (no
  authentication).
- The `whatsmeow` dependency tree (`internal/wa`, all `go.mau.fi/...`
  imports), `signal`, and related packages.
- `doctor --connect` and the auth/connection/lock fields on the doctor
  report.

### Changed

- Binary renamed to `wacli-reader`. Source directory `cmd/wacli/` retains
  its upstream name to keep upstream rebases clean.
- Production code opens the store with a new `store.OpenReadOnly` helper
  (URI flag `mode=ro`); the existing `store.Open` (writable, runs
  migrations) is preserved unchanged so upstream's store tests and
  upstream changes to the store package rebase cleanly. Schema migrations
  are not run by `wacli-reader`; the upstream writer is expected to have
  created and migrated the database.
- Default store directory is unchanged (still `~/.local/state/wacli` on
  Linux, `~/.wacli` elsewhere) so the reader transparently sees the
  upstream writer's data.
- `doctor` report slimmed to store dir, FTS flag, and DB stats.

### Kept (rebased from upstream)

- `chats list`, `chats show`
- `contacts search`, `contacts show`
- `groups list`
- `messages list`, `messages search`, `messages show`, `messages context`
- `doctor`, `version`, `completion`, `help`
- FTS5-backed search with `LIKE` fallback when the build tag is absent
- `--store`, `--json`, `--full`, `--timeout` global flags

### Compatibility

- Designed to read a `wacli.db` produced by upstream `wacli` v0.7.0.
- Two binaries can run concurrently against the same store (upstream
  `wacli sync --follow` as writer, `wacli-reader` as reader); SQLite WAL
  prevents reader/writer blocking.

### Why this fork exists

Upstream `wacli` requires `session.db` (a full-access WhatsApp session
token) to run, and exposes commands that send messages and modify groups.
That makes the upstream binary unsafe to hand to an AI agent: a compromised
agent can use any subcommand to act on the user's behalf. `wacli-reader`
removes the dangerous code paths entirely, so there is nothing to disable
or bypass.
