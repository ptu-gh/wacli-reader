# Changelog

All notable changes to `wacli-reader` will be documented in this file. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
`wacli-reader` carries its own version history, independent of the upstream
[`wacli`](https://github.com/steipete/wacli) version it was forked from.

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
