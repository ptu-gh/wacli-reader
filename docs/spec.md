# wacli-reader specification

`wacli-reader` is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli), forked at upstream **`v0.7.0`**. It exposes only commands that read a `wacli.db` produced by the upstream writer; it never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store with `mode=ro` so writes are rejected at the driver layer.

The full client/storage spec — auth model, sync model, send pipeline, schema design — lives in upstream `wacli`'s `docs/spec.md` and is implicitly inherited here. This document covers only what is *different* in `wacli-reader`.

## Goals

- **Agent-safe read access** to a `wacli.db` populated by upstream `wacli sync`.
- **No `session.db` access**: the binary contains no code path that opens, reads, or writes `session.db`.
- **No network I/O**: there is no whatsmeow client and no WhatsApp connectivity.
- **Read-only at the SQLite layer**: the connection is opened `mode=ro`; any write attempt fails with `attempt to write a readonly database`.
- **Concurrent with upstream writer**: an upstream `wacli sync --follow` may run against the same `wacli.db` while `wacli-reader` reads.
- **Minimal diff vs. upstream**: the fork is intended to be rebased periodically. Surviving files keep upstream's structure as much as possible.

## Non-goals

- Authentication, message sending, reactions, media upload/download, presence indicators, history backfill, group/contact management. (All upstream commands that perform these have been removed.)
- Schema evolution. The schema is owned by upstream `wacli`. `wacli-reader` does not run migrations; the upstream writer is expected to have created and migrated the database.
- Re-enabling writes via flag, env var, or config file. There is no such switch.

## Storage model

- Directory: defaults to upstream's resolution (`WACLI_STORE_DIR` env → XDG state dir on Linux → `~/.wacli` elsewhere). `wacli-reader` does not create the directory.
- File: `wacli.db` only. Upstream's `session.db` may sit alongside it on disk but is never read.
- Open: `sql.Open("sqlite3", "file:<path>?mode=ro&_foreign_keys=on&_busy_timeout=5000")`.
- FTS5: `messages_fts` virtual table is detected at open time. If absent, `messages search` falls back to `LIKE`.
- WAL: WAL mode is configured by the upstream writer. Readers see consistent snapshots without blocking the writer.

## CLI surface

| Command | Description |
| --- | --- |
| `wacli-reader chats list [--query TEXT] [--limit N]` | List chats. |
| `wacli-reader chats show --jid JID` | Show one chat. |
| `wacli-reader contacts search <query> [--limit N]` | Search contacts. |
| `wacli-reader contacts show --jid JID` | Show one contact. |
| `wacli-reader groups list [--query TEXT] [--limit N]` | List groups. |
| `wacli-reader messages list [filters]` | List messages. Filters: `--chat`, `--sender`, `--from-me`/`--from-them`, `--asc`, `--limit`, `--after`, `--before`. |
| `wacli-reader messages search <query> [filters]` | FTS5 (or `LIKE`) message search. Filters: `--chat`, `--from`, `--has-media`, `--type`, `--limit`, `--after`, `--before`. |
| `wacli-reader messages show --chat JID --id MSG_ID` | Show one message. |
| `wacli-reader messages context --chat JID --id MSG_ID [--before N] [--after N]` | Show context around a message. |
| `wacli-reader doctor` | Store dir, FTS flag, message/chat/contact/group counts. |
| `wacli-reader version` | Print `wacli-reader <version>`. |
| `wacli-reader help`, `wacli-reader completion <shell>` | Cobra built-ins. |

Global flags: `--store DIR`, `--json`, `--full`, `--timeout DURATION`. Environment overrides: `WACLI_STORE_DIR`.

## Concurrency

- WAL + `mode=ro` + no lock acquisition. The reader can run while upstream `wacli sync --follow` is writing; SQLite handles isolation.
- The reader does **not** create the `LOCK` file and does not block on it. The upstream writer's lock is irrelevant to this binary.

## Safety properties

1. The binary contains no code that opens `session.db`.
2. The SQLite connection is opened with `mode=ro`; writes fail at the driver layer.
3. There is no flag, env var, or config file that re-enables writes.
4. `internal/wa` (whatsmeow client) and `internal/lock` are absent from the binary.
5. The binary performs no network I/O.

For defence in depth, pair `wacli-reader` with filesystem ACLs: give the consuming process read-only access to `wacli.db` and `wacli.db-wal`, and deny access to `session.db`.

## Compatibility and rebase policy

- Compatible with `wacli.db` produced by upstream `wacli` v0.7.0 and later.
- The fork is rebased periodically on upstream. See `AGENTS.md` § "Rebasing on upstream wacli" for the conflict-resolution checklist.
