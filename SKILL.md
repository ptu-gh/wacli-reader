---
name: wacli-reader
description: Search and read a local WhatsApp archive.
user-invocable: true
metadata:
  requires:
    bins:
      - wacli-reader
---

# WhatsApp History & Media Archive Skill

`wacli-reader` reads the local WhatsApp archive (`wacli.db`) maintained by a background `wacli sync` service. Read-only.

When you invoke `wacli-reader` it automatically returns JSON (no `--json` flag needed — JSON is the default when stdout is not a TTY).

## Prerequisites

- `wacli-reader` on `$PATH`
- `$WACLI_STORE_DIR` set to the `wacli` store dir

## JSON output — what you'll see

Every response is wrapped: `{"success": bool, "data": ..., "error": null|string}`. Real payload is at `.data`.

| Command | Payload path | Field naming |
|---|---|---|
| `chats list/show`, `contacts search/show`, `groups list` | `.data` (raw array or object) | snake_case (`jid`, `kind`, `name`, …) |
| `messages list/search/show/context/starred/export` | `.data.messages` (or `.data` for `show`) | **PascalCase** (`ChatJID`, `MsgID`, `SenderName`, `Timestamp`, `Text`, `DisplayText`, `LocalPath`, `MediaType`, `IsForwarded`, `Starred`, …) |
| `calls list` | `.data.calls` | snake_case |
| `polls list` / `poll show` | `.data.polls` / `.data` | snake_case |
| `store stats`, `doctor` | `.data` | snake_case |

Empty result = `null` *or* `[]`. Treat both as "no results".

## Tools

### Find a chat or contact by name
```
wacli-reader chats list --query "{{name}}" --limit 20
wacli-reader contacts search "{{name}}" --limit 20
```
Resolves names to JIDs. JIDs look like `12345@s.whatsapp.net` (a person), `groupid@g.us` (a group), or `12345@lid` (a privacy ID — see gotcha below).

### Search messages
```
wacli-reader messages search "{{query}}" --limit 20
wacli-reader messages search "{{query}}" --chat {{jid}}
wacli-reader messages search "{{query}}" --after YYYY-MM-DD
```
Add `--forwarded` or `--starred` to narrow. Same flags work on `messages list`.

### Read a chat thread
```
wacli-reader messages list --chat {{jid}} --limit 20
```

### One message + its context
```
wacli-reader messages show --chat {{jid}} --id {{msg_id}}
wacli-reader messages context --chat {{jid}} --id {{msg_id}} --before 5 --after 5
```

### Starred messages
```
wacli-reader messages starred --limit 50
```
Add `--chat {{jid}}` to scope to one conversation.

### Export a thread as JSON
```
wacli-reader messages export --chat {{jid}} --limit 5000 --output /tmp/thread.json
```
Oldest first. Omit `--output` to stream to stdout.

### Call history
```
wacli-reader calls list --limit 20
```
Optional: `--chat`, `--after`, `--before`.

### Polls
```
wacli-reader polls list
wacli-reader poll show --chat {{jid}} --id {{msg_id}}
```
`show` returns question, options, per-option vote counts, per-voter selections. Cannot vote.

### Store stats (heartbeat)
```
wacli-reader store stats
```

## Gotchas (read these)

### LID vs phone JID — `chats show` may return nothing

`contacts search` can return a `…@lid` JID (a WhatsApp privacy identity). The `chats` table is keyed by phone JID (`…@s.whatsapp.net`), so `chats show --jid <lid>` will give `sql: no rows`.

**Rule:** when starting from a contact name, use `chats list --query "Name"` — not `chats show --jid <lid from contacts>`. `chats list` matches by chat name and returns the phone JID directly.

### FTS5 warning is fine

If you see `Note: FTS5 not enabled`, search just uses `LIKE` instead (slower, same results). Don't mention it to the user.

## Media

**Never run `wacli media download`** — wrong binary, no permissions.

**Never copy media files.** Read them in place.

1. Get a message via any read command. Look at `LocalPath` (PascalCase — Go struct field).
2. If `LocalPath` is a path like `$WACLI_STORE_DIR/media/...`, the file is **already on disk and readable**. Pass the path directly to your file-attach / file-read tool.
3. **Do NOT `cp` the file first.** On macOS `cp` tries to copy extended attributes and exits with "could not copy extended attributes ... Permission denied". That error is cosmetic — the file *data* is readable, `cp` is the problem. Skip the copy entirely.
4. If your tool somehow cannot read the path and you must duplicate the bytes, use `cat "$src" > "$dst"`. Never `cp`, `rsync`, `ditto`, etc.
5. If `LocalPath` is empty: tell the user *"The media for this message has not been archived locally."*
6. GIFs currently fail to archive — treat as missing.

## Display extraction

For a message, use:
- `SenderName` for who said it
- `DisplayText` for the text (already rendered for reactions, edits, etc.). Fall back to `Text` if empty.
- `Timestamp` for when

## Privacy

Pull only the specific messages needed to answer the prompt.
