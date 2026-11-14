# 🗃️ wacli-reader — agent-safe read-only WhatsApp message access

`wacli-reader` is a heavily trimmed fork of [`wacli`](https://github.com/steipete/wacli) (now developed at [`github.com/openclaw/wacli`](https://github.com/openclaw/wacli)), rebuilt on upstream `v0.26.0` (upstream `main` at `2108fe7`). It exposes only commands that **read** a `wacli.db` produced by the upstream writer — searching messages, listing chats and groups, looking up contacts, inspecting calls and polls, exporting message history, following the local change feed. It never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store read-only so the connection itself rejects writes at the driver layer.

Run `wacli-reader` alongside an upstream `wacli sync --follow` instance to give an AI agent (or any read-only consumer) safe, scoped access to your message history without exposing send/auth/group-management capabilities.

`wacli-reader` itself does **not** speak the WhatsApp Web protocol and does not depend on `whatsmeow`. It only reads a `wacli.db` SQLite file that upstream `wacli` (which does use `whatsmeow`) has already populated. This project is third-party and not affiliated with WhatsApp.

## Why this fork exists

Upstream `wacli` is a full-featured WhatsApp client. It needs `session.db` (which holds a full-access WhatsApp session token) to run, and it ships commands that send messages, react, edit and delete messages, modify groups, change participants, save contacts, and revoke invite links. That makes it unsafe to hand to an AI agent: a compromised or rogue agent can use any subcommand to send messages or leak data on the user's behalf. Upstream's `--read-only` flag is opt-in and trivially bypassed by the agent.

`wacli-reader` removes the dangerous code paths entirely so there is nothing to bypass:

- The binary contains no code that opens `session.db`.
- The SQLite connection is opened with `mode=ro`; writes fail at the driver layer with `attempt to write a readonly database`.
- There is no flag, env var, or config file that re-enables writes. (`--read-only` is accepted for compatibility with upstream command lines, and `--read-only=false` is an error.)
- `internal/wa` (the whatsmeow client), `internal/lock`, and `internal/delegate` (hand-off to a running writer) are absent from the binary, and `whatsmeow` is absent from the module graph.
- The binary performs no network I/O.

The test suite enforces these: it pins the exact command list, rejects source that imports a WhatsApp client, lock, delegate or network package or names `session.db`, and checks that every read command leaves the store byte-identical.

Pair the binary with filesystem ACLs for defence in depth: give the agent's user read-only access to `wacli.db` (and `wacli.db-wal`, `wacli.db-shm`) and deny access to `session.db`.

## Relationship to upstream

`wacli-reader` is rebuilt on upstream `wacli` periodically. Sync, send, media, presence, history-backfill, auth, and group/contact-management commands are deleted in this fork; bug fixes, search improvements, and new read features that land upstream are pulled in on each rebase. To keep rebases tractable we leave upstream's `internal/store` package and its tests untouched — the read-only guarantee comes from the `store.OpenReadOnly` path in production code, not from removing helpers.

## Requirements

- Go 1.27.4 (pinned by `go.mod`'s `toolchain` line; with `GOTOOLCHAIN=auto` an older Go fetches it) and a C compiler for `go-sqlite3`.
- A `wacli.db` migrated by upstream `wacli` **0.23 or newer** (schema v33). The reader never migrates; against an older store some commands fail with `no such column`. `wacli-reader doctor` shows the store's schema version next to the one this build expects — let a current upstream `wacli` open the store once to upgrade it.

## Install / Build

```bash
pnpm build    # = go build -tags sqlite_fts5 -o ./dist/wacli-reader ./cmd/wacli
```

The source directory is still `./cmd/wacli` (unchanged from upstream for rebase ergonomics); the resulting binary is `wacli-reader`.

```bash
./dist/wacli-reader --help
```

## Quick start

Default store directory is the upstream writer's location: the XDG state dir on Linux (`~/.local/state/wacli`) and `~/.wacli` elsewhere. Existing Linux `~/.wacli` stores keep working. Override with `--store DIR` or `WACLI_STORE_DIR`.

```bash
# Diagnostics — store path, FTS flag, schema version, counts, last sync
pnpm wacli-reader doctor

# Search messages (FTS5 if available, LIKE fallback), with two messages of context per hit
pnpm wacli-reader messages search "meeting" --context-before 2 --context-after 2

# List recent messages from a chat — by number, JID, or exact local name — oldest first
pnpm wacli-reader messages list --chat "Alice" --asc

# Show context around a message
pnpm wacli-reader messages context --chat 1234567890@s.whatsapp.net --id <message-id>

# Show one message
pnpm wacli-reader messages show --chat 1234567890@s.whatsapp.net --id <message-id>

# Chats
pnpm wacli-reader chats list --unread --no-archived
pnpm wacli-reader chats show --jid 1234567890@s.whatsapp.net

# Contacts
pnpm wacli-reader contacts search "alice"
pnpm wacli-reader contacts show --jid 1234567890@s.whatsapp.net

# Groups and their last-synced participant lists
pnpm wacli-reader groups list
pnpm wacli-reader groups participants list --jid 123456789@g.us

# Call events
pnpm wacli-reader calls list --limit 20

# Polls (stored locally; no live voting)
pnpm wacli-reader polls list
pnpm wacli-reader poll show --chat 1234567890@s.whatsapp.net --id <poll-msg-id>

# Local archive coverage per chat
pnpm wacli-reader history coverage --only-actionable

# Starred messages, message export, and store row counts
pnpm wacli-reader messages starred --limit 50
pnpm wacli-reader messages export --chat 1234567890@s.whatsapp.net --output thread.json
pnpm wacli-reader store stats

# Incremental consumers: take a checkpoint, then read what changed since
pnpm wacli-reader messages changes --checkpoint
pnpm wacli-reader messages changes --since <checkpoint>
```

## Command surface

- `wacli-reader chats list [--query TEXT] [--limit N] [--archived|--no-archived] [--pinned|--no-pinned] [--muted|--no-muted] [--unread|--no-unread]`
- `wacli-reader chats show --jid JID`
- `wacli-reader contacts search <query> [--limit N]`
- `wacli-reader contacts show --jid JID`
- `wacli-reader groups list [--query TEXT] [--limit N]`
- `wacli-reader groups participants list --jid GROUP_JID`
- `wacli-reader messages list [--chat CHAT] [--sender WHO] [--from-me|--from-them] [--asc] [--limit N] [--after DATE] [--before DATE] [--forwarded] [--starred] [--cursor TOKEN]`
- `wacli-reader messages search <query> [--chat CHAT] [--from WHO] [--has-media] [--type text|image|video|audio|document] [--forwarded] [--starred] [--limit N] [--after DATE] [--before DATE] [--context-before N] [--context-after N] [--cursor TOKEN]`
- `wacli-reader messages starred [--chat CHAT] [--limit N] [--after DATE] [--before DATE] [--asc]`
- `wacli-reader messages show --chat CHAT --id MSG_ID`
- `wacli-reader messages context --chat CHAT --id MSG_ID [--before N] [--after N]`
- `wacli-reader messages export [--chat CHAT] [--limit N] [--after DATE] [--before DATE] [--cursor TOKEN] [--output PATH]`
- `wacli-reader messages changes (--checkpoint | --since TOKEN|beginning) [--limit N]`
- `wacli-reader calls list [--chat JID] [--limit N] [--after DATE] [--before DATE] [--asc]`
- `wacli-reader polls list [--chat JID] [--limit N]`
- `wacli-reader poll show --chat JID --id MSG_ID`
- `wacli-reader history coverage [--chat JID] [--query TEXT] [--kind KIND] [--limit N] [--include-blocked] [--only-actionable]`
- `wacli-reader accounts list`
- `wacli-reader accounts show NAME`
- `wacli-reader store stats`
- `wacli-reader doctor`
- `wacli-reader docs`
- `wacli-reader version`
- `wacli-reader help`, `wacli-reader completion <shell>` (cobra built-ins)

`CHAT` and `WHO` accept a phone number, a JID, or the exact local name of a chat or contact. `--cursor` continues a paged read: pass the `next_cursor` from the previous page's `page` object and repeat the original filters.

## Storage and concurrency

By default `wacli-reader` resolves the same store directory as upstream `wacli` (`~/.local/state/wacli` on Linux, `~/.wacli` elsewhere), so it transparently reads whatever the writer has populated. It opens `wacli.db` with `mode=ro&_query_only=1&readonly_shm=1` and never acquires the upstream writer's `LOCK` file, so it is safe to run while `wacli sync --follow` is writing, and any number of readers can run at once. SQLite's WAL mode (configured by the writer) gives every query — and every multi-query page, which runs in one read transaction — a consistent snapshot without blocking the writer; fresh WAL commits become visible on the next query. `readonly_shm=1` means the reader never writes even SQLite's shared-memory index. When no WAL/SHM sidecars exist next to the database (no writer has it open, or a read-only filesystem), the reader adds `immutable=1`.

`wacli-reader` does not create the store directory, run schema migrations, or chmod database files — those are the writer's responsibility.

## Global flags

- `--store DIR`: store directory. Bypasses `config.yaml` entirely.
- `--account NAME`: named account from `config.yaml`. Mutually exclusive with `--store`.
- `--json`: force JSON output. By default output is JSON when stdout is not a TTY and a human-readable table when it is, so piped and scripted callers get JSON without asking.
- `--no-json`: force the table output even when stdout is not a TTY.
- `--full`: disable table truncation.
- `--read-only`: always on; accepted so that command lines written for upstream `wacli` keep working. `--read-only=false` is rejected.
- `--timeout DURATION`: accepted for parity with upstream `wacli`, but currently a **no-op** — the store layer takes no context, so nothing observes the deadline. Read commands are local SQLite queries and return promptly.

## Environment overrides

- `WACLI_STORE_DIR`: override the default store directory.

Store resolution order is identical to upstream `wacli`: `--store` → `--account` → `$WACLI_STORE_DIR` → `default_account` from `config.yaml` → the platform default. That means the reader and the writer always agree on which store they are looking at, including on multi-account installs.

## Prior art / credit

`wacli-reader` is a fork of [`wacli`](https://github.com/steipete/wacli) by [@steipete](https://github.com/steipete) and [@dinakars777](https://github.com/dinakars777), which is itself heavily inspired by Vicente Reig's [`whatsapp-cli`](https://github.com/vicentereig/whatsapp-cli). All the credit for the synced data model, FTS5 search infrastructure, and CLI structure goes upstream — this fork is purely a reductive variant.

## License

See `LICENSE`.
