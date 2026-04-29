# Changelog

All notable changes to `wacli-reader` will be documented in this file. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
`wacli-reader` carries its own version history, independent of the upstream
[`wacli`](https://github.com/steipete/wacli) version it was forked from.

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
