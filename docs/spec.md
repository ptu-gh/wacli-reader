# wacli-reader specification

`wacli-reader` is a heavily trimmed, agent-safe fork of [`wacli`](https://github.com/steipete/wacli) (now developed at [`github.com/openclaw/wacli`](https://github.com/openclaw/wacli)), originally forked at upstream **`v0.7.0`** and rebuilt on upstream **`v0.26.0`** (upstream `main` at `2108fe7`). It exposes only commands that read a `wacli.db` produced by the upstream writer; it never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store read-only so writes are rejected at the driver layer.

The full client/storage spec — auth model, sync model, send pipeline, schema design — lives in upstream `wacli`'s `docs/` and is implicitly inherited here. This document covers only what is *different* in `wacli-reader`.

## Goals

- **Agent-safe read access** to a `wacli.db` populated by upstream `wacli sync`.
- **No `session.db` access**: the binary contains no code path that opens, reads, or writes `session.db`.
- **No network I/O**: there is no whatsmeow client and no WhatsApp connectivity.
- **Read-only at the SQLite layer**: the connection is opened `mode=ro`; any write attempt fails with `attempt to write a readonly database`.
- **Concurrent with upstream writer and with other readers**: an upstream `wacli sync --follow` may run against the same `wacli.db` while any number of `wacli-reader` processes read.
- **Minimal diff vs. upstream**: the fork is rebuilt on upstream periodically. Surviving files keep upstream's structure as much as possible.

## Non-goals

- Authentication, message sending, editing, deletion, reactions, media upload/download, presence, history backfill, group/contact management. (All upstream commands that perform these have been removed.)
- Schema evolution. The schema is owned by upstream `wacli`. `wacli-reader` does not run migrations; the upstream writer is expected to have created and migrated the database.
- Re-enabling writes via flag, env var, or config file. There is no such switch.

## Storage model

- Directory: defaults to upstream's resolution (`--store` → `--account` → `WACLI_STORE_DIR` → `config.yaml` `default_account` → XDG state dir on Linux → `~/.wacli` elsewhere). `wacli-reader` does not create the directory.
- File: `wacli.db` only. Upstream's `session.db` may sit alongside it on disk but is never read.
- Open: upstream's `store.OpenReadOnly`, i.e. `file:<path>?_foreign_keys=on&_busy_timeout=5000&mode=ro&_query_only=1&readonly_shm=1`. `immutable=1` is added when no WAL/SHM/journal sidecar files exist next to the DB (no writer has it open, or a read-only filesystem); otherwise the reader uses SQLite's normal locking so the upstream writer's WAL commits remain visible.
- FTS5: the `messages_fts` virtual table is detected at open time. If absent, `messages search` falls back to `LIKE`.
- WAL: WAL mode is configured by the upstream writer. Readers see consistent snapshots without blocking the writer. Paged reads (`messages list/search/export`) run their queries in one read transaction, so a page, its cursor, and its sender names come from one snapshot.

## CLI surface

| Command | Description |
| --- | --- |
| `wacli-reader chats list [filters]` | List chats. Filters: `--query`, `--limit`, `--archived`/`--pinned`/`--muted`/`--unread` and their `--no-` forms. |
| `wacli-reader chats show --jid JID` | Show one chat. |
| `wacli-reader contacts search <query> [--limit N]` | Search contacts. |
| `wacli-reader contacts show --jid JID` | Show one contact. |
| `wacli-reader groups list [--query TEXT] [--limit N]` | List groups. |
| `wacli-reader groups participants list --jid JID` | Last synced participant snapshot of one group. |
| `wacli-reader messages list [filters]` | List messages. Filters: `--chat`, `--sender`, `--from-me`/`--from-them`, `--asc`, `--limit`, `--after`, `--before`, `--forwarded`, `--starred`, `--cursor`. |
| `wacli-reader messages search <query> [filters]` | FTS5 (or `LIKE`) message search. Filters: `--chat`, `--from`, `--has-media`, `--type`, `--limit`, `--after`, `--before`, `--forwarded`, `--starred`, `--context-before`, `--context-after`, `--cursor`. |
| `wacli-reader messages starred [filters]` | List starred messages by stored star time. Filters: `--chat`, `--limit`, `--after`, `--before`, `--asc`. |
| `wacli-reader messages show --chat CHAT --id MSG_ID` | Show one message. |
| `wacli-reader messages context --chat CHAT --id MSG_ID [--before N] [--after N]` | Show context around a message. |
| `wacli-reader messages export [filters] [--output PATH]` | Export messages as JSON, oldest first. Filters: `--chat`, `--limit`, `--after`, `--before`, `--cursor`. |
| `wacli-reader messages changes (--checkpoint \| --since TOKEN) [--limit N]` | Content-free feed of message upserts, deletions and identity moves since a checkpoint. |
| `wacli-reader calls list [filters]` | List call events. Filters: `--chat`, `--limit`, `--after`, `--before`, `--asc`. |
| `wacli-reader polls list [--chat JID] [--limit N]` | List stored polls. |
| `wacli-reader poll show --chat JID --id MSG_ID` | Show one poll's question, options, and votes. |
| `wacli-reader history coverage [filters]` | Per-chat local archive coverage. |
| `wacli-reader accounts list` / `accounts show NAME` | Inspect `config.yaml` accounts. |
| `wacli-reader store stats` | Row counts (chats, groups, left groups, messages). |
| `wacli-reader doctor` | Store dir, FTS flag, schema version vs. expected, counts, last sync. |
| `wacli-reader docs` | Print the documentation URL. |
| `wacli-reader version` | Print `wacli-reader <version>`. |
| `wacli-reader help`, `wacli-reader completion <shell>` | Cobra built-ins. |

`CHAT` accepts a phone number, a JID, or the exact local name of a chat or contact.

Global flags: `--store DIR`, `--account NAME`, `--json`, `--no-json`, `--full`, `--read-only` (always on; `=false` is an error), `--timeout DURATION` (inert). Environment overrides: `WACLI_STORE_DIR`.

## Concurrency

- WAL + `mode=ro` + `readonly_shm=1` + no lock acquisition. The reader can run while upstream `wacli sync --follow` is writing, and readers do not contend with each other; SQLite handles isolation.
- The reader does **not** create the `LOCK` file and does not block on it. The upstream writer's lock is irrelevant to this binary.
- `TestConcurrentReaderProcesses` runs reader processes in parallel with a live writer committing continuously and requires every reader to succeed and see the writer's commits.

## Safety properties

1. The binary contains no code that opens `session.db`.
2. The SQLite connection is opened with `mode=ro`; writes fail at the driver layer.
3. There is no flag, env var, or config file that re-enables writes.
4. `internal/wa` (whatsmeow client), `internal/lock`, and `internal/delegate` are absent from the binary; `whatsmeow` is absent from the module graph.
5. The binary performs no network I/O.

Tests in `cmd/wacli/readonly_guard_test.go` enforce 1, 4 and 5 at the source level, pin the command list, and check that every read command leaves the store directory byte-identical.

For defence in depth, pair `wacli-reader` with filesystem ACLs: give the consuming process read-only access to `wacli.db`, `wacli.db-wal` and `wacli.db-shm`, and deny access to `session.db`.

## Compatibility and rebase policy

- Requires a `wacli.db` migrated by upstream `wacli` 0.23 or newer (schema v33). Against an older store, commands whose queries need newer columns fail with `no such column`; `doctor` reports the store's schema version and the version this build expects.
- The fork is rebuilt on upstream periodically. See `AGENTS.md` § "Rebasing on upstream wacli" for the checklist.
