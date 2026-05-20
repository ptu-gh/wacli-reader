# 🗃️ wacli-reader — agent-safe read-only WhatsApp message access

`wacli-reader` is a heavily trimmed fork of [`wacli`](https://github.com/steipete/wacli) (now developed at [`github.com/openclaw/wacli`](https://github.com/openclaw/wacli)) — originally forked at upstream `v0.7.0` and tracked forward to upstream `v0.9.2`. It exposes only commands that **read** a `wacli.db` produced by the upstream writer — searching messages, listing chats and groups, looking up contacts, inspecting calls and polls, exporting message history. It never authenticates with WhatsApp, never opens `session.db`, and opens the SQLite store read-only so the connection itself rejects writes at the driver layer.

Run `wacli-reader` alongside an upstream `wacli sync --follow` instance to give an AI agent (or any read-only consumer) safe, scoped access to your message history without exposing send/auth/group-management capabilities.

`wacli-reader` itself does **not** speak the WhatsApp Web protocol and does not depend on `whatsmeow`. It only reads a `wacli.db` SQLite file that upstream `wacli` (which does use `whatsmeow`) has already populated. This project is third-party and not affiliated with WhatsApp.

## Why this fork exists

Upstream `wacli` is a full-featured WhatsApp client. It needs `session.db` (which holds a full-access WhatsApp session token) to run, and it ships commands that send messages, react, modify groups, change participants, and revoke invite links. That makes it unsafe to hand to an AI agent: a compromised or rogue agent can use any subcommand to send messages or leak data on the user's behalf. Upstream's `--read-only` flag is opt-in and trivially bypassed by the agent.

`wacli-reader` removes the dangerous code paths entirely so there is nothing to bypass:

- The binary contains no code that opens `session.db`.
- The SQLite connection is opened with `mode=ro`; writes fail at the driver layer with `attempt to write a readonly database`.
- There is no flag, env var, or config file that re-enables writes.
- `internal/wa` (the whatsmeow client) and `internal/lock` are absent from the binary.
- The binary performs no network I/O.

Pair the binary with filesystem ACLs for defence in depth: give the agent's user read-only access to `wacli.db` (and `wacli.db-wal`) and deny access to `session.db`.

## Relationship to upstream

`wacli-reader` is rebased periodically on upstream `wacli`. Sync, send, media, presence, history-backfill, auth, and group/contact-management commands are deleted in this fork; bug fixes and search improvements that flow into upstream are pulled into the fork on each rebase. To keep rebases tractable we leave upstream's `internal/store` package and its tests untouched — the read-only guarantee comes from the new `store.OpenReadOnly` wrapper in production code, not from removing helpers.

## Install / Build

```bash
go build -tags sqlite_fts5 -o ./dist/wacli-reader ./cmd/wacli
```

The source directory is still `./cmd/wacli` (unchanged from upstream for rebase ergonomics); the resulting binary is `wacli-reader`.

```bash
./dist/wacli-reader --help
```

## Quick start

Default store directory is the upstream writer's location: the XDG state dir on Linux (`~/.local/state/wacli`) and `~/.wacli` elsewhere. Existing Linux `~/.wacli` stores keep working. Override with `--store DIR` or `WACLI_STORE_DIR`.

```bash
# Diagnostics — store path, FTS flag, message/chat/contact/group counts
pnpm wacli-reader doctor

# Search messages (FTS5 if available, LIKE fallback)
pnpm wacli-reader messages search "meeting"

# List recent messages from a chat, oldest first
pnpm wacli-reader messages list --chat 1234567890@s.whatsapp.net --asc

# Show context around a message
pnpm wacli-reader messages context --chat 1234567890@s.whatsapp.net --id <message-id>

# Show one message
pnpm wacli-reader messages show --chat 1234567890@s.whatsapp.net --id <message-id>

# Chats
pnpm wacli-reader chats list
pnpm wacli-reader chats show --jid 1234567890@s.whatsapp.net

# Contacts
pnpm wacli-reader contacts search "alice"
pnpm wacli-reader contacts show --jid 1234567890@s.whatsapp.net

# Groups (read-only listing)
pnpm wacli-reader groups list

# Call events
pnpm wacli-reader calls list --limit 20

# Polls (stored locally; no live voting)
pnpm wacli-reader polls list
pnpm wacli-reader poll show --chat 1234567890@s.whatsapp.net --id <poll-msg-id>

# Starred messages, message export, and store row counts
pnpm wacli-reader messages starred --limit 50
pnpm wacli-reader messages export --chat 1234567890@s.whatsapp.net --output thread.json
pnpm wacli-reader store stats
```

## Command surface

- `wacli-reader chats list [--query TEXT] [--limit N]`
- `wacli-reader chats show --jid JID`
- `wacli-reader contacts search <query> [--limit N]`
- `wacli-reader contacts show --jid JID`
- `wacli-reader groups list [--query TEXT] [--limit N]`
- `wacli-reader messages list [--chat JID] [--sender JID] [--from-me|--from-them] [--asc] [--limit N] [--after DATE] [--before DATE] [--forwarded] [--starred]`
- `wacli-reader messages search <query> [--chat JID] [--from JID] [--has-media] [--type text|image|video|audio|document] [--forwarded] [--starred]`
- `wacli-reader messages starred [--chat JID] [--limit N] [--after DATE] [--before DATE] [--asc]`
- `wacli-reader messages show --chat JID --id MSG_ID`
- `wacli-reader messages context --chat JID --id MSG_ID [--before N] [--after N]`
- `wacli-reader messages export [--chat JID] [--limit N] [--after DATE] [--before DATE] [--output PATH]`
- `wacli-reader calls list [--chat JID] [--limit N] [--after DATE] [--before DATE] [--asc]`
- `wacli-reader polls list [--chat JID] [--limit N]`
- `wacli-reader poll show --chat JID --id MSG_ID`
- `wacli-reader store stats`
- `wacli-reader doctor`
- `wacli-reader version`
- `wacli-reader help`, `wacli-reader completion <shell>` (cobra built-ins)

## Storage and concurrency

By default `wacli-reader` resolves the same store directory as upstream `wacli` (`~/.local/state/wacli` on Linux, `~/.wacli` elsewhere), so it transparently reads whatever the writer has populated. It opens `wacli.db` with `mode=ro&_query_only=1` (and adds `immutable=1` only when no WAL/SHM sidecars exist next to the DB, e.g. on a truly read-only filesystem) and never acquires the upstream writer's `LOCK` file, so it is safe to run while `wacli sync --follow` is writing. SQLite's WAL mode (configured by the writer) lets readers see a consistent snapshot at every query without blocking the writer, and fresh WAL commits become visible on each new query.

`wacli-reader` does not create the store directory, run schema migrations, or chmod database files — those are the writer's responsibility.

## Global flags

- `--store DIR`: store directory.
- `--json`: JSON output.
- `--full`: disable table truncation.
- `--timeout DURATION`: timeout for read commands (default 5m).

## Environment overrides

- `WACLI_STORE_DIR`: override the default store directory.

## Prior art / credit

`wacli-reader` is a fork of [`wacli`](https://github.com/steipete/wacli) by [@steipete](https://github.com/steipete) and [@dinakars777](https://github.com/dinakars777), which is itself heavily inspired by Vicente Reig's [`whatsapp-cli`](https://github.com/vicentereig/whatsapp-cli). All the credit for the synced data model, FTS5 search infrastructure, and CLI structure goes upstream — this fork is purely a reductive variant.

## License

See `LICENSE`.
