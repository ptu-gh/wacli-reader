---
name: wacli-reader
description: Read the user's WhatsApp history from a local archive. Use it for any question about WhatsApp chats, messages, contacts and their phone numbers, groups and members, calls, polls, and photos or files sent on WhatsApp. Read-only. It cannot send, reply or delete.
user-invocable: true
metadata:
  requires:
    bins:
      - wacli-reader
---

# WhatsApp archive (read-only)

`wacli-reader` reads a local copy of the user's WhatsApp history. It cannot send, reply, react, delete or download. If asked for one of those, say that this skill cannot do it. Never run `wacli`.

Output is JSON: `{"success":true,"data":...}`. On failure the exit code is 1 and the error is on stderr. `data` that is `null` or an empty list means nothing was found. That is an answer, not an error.

Report only what the output contains. Never invent a message, sender or date.

## Step 1: find the chat

```
wacli-reader chats list --query "NAME" --limit 20
```

`NAME` is a first name, a group name or a phone number. Each entry has `jid`, `name` and `kind` (`dm` = a person, `group`).

- One entry: its `jid` is `CHAT_JID`.
- Several entries: take the one whose `name` fits the request. If several fit, list their names and ask the user which one. Do not pick one yourself.
- None: try a shorter `NAME`. Still none: say the chat is not in the archive.

Take `CHAT_JID` only from `chats list`. Never guess it, and never use a `jid` from `contacts search`.

## Step 2: one command for the task

Words in CAPITALS are placeholders. Replace them with real values.

| Task | Command | Result |
|---|---|---|
| last message from a person | `wacli-reader messages list --chat CHAT_JID --from-them --limit 5` | `data.messages` |
| last photo or file from a person | `wacli-reader messages list --chat CHAT_JID --from-them --limit 20` | the first entry with `MediaType` set |
| latest messages of a chat | `wacli-reader messages list --chat CHAT_JID --limit 20` | `data.messages` |
| messages on given days | `wacli-reader messages list --chat CHAT_JID --after YYYY-MM-DD --before YYYY-MM-DD --limit 100` | `data.messages` |
| oldest messages of a chat | `wacli-reader messages list --chat CHAT_JID --asc --limit 5` | `data.messages` |
| a topic, all chats | `wacli-reader messages search "WORDS" --limit 20` | `data.messages` |
| a topic, one chat | `wacli-reader messages search "WORDS" --chat CHAT_JID --limit 20` | `data.messages` |
| a topic, with the replies around it | `wacli-reader messages search "WORDS" --chat CHAT_JID --context-after 3 --limit 10` | `data.context[].messages` |
| messages around one message | `wacli-reader messages context --chat CHAT_JID --id MSG_ID --before 5 --after 5` | `data` |
| one message in full | `wacli-reader messages show --chat CHAT_JID --id MSG_ID` | `data` |
| chats with unread messages | `wacli-reader chats list --unread --no-archived --limit 20` | `data` |
| starred messages | `wacli-reader messages starred --limit 20` | `data.messages` |
| a contact's phone number | `wacli-reader contacts search "NAME" --limit 20` | `data[].phone` |
| groups | `wacli-reader groups list --limit 50` | `data[].Name`, `data[].JID` |
| members of a group | `wacli-reader groups participants list --jid CHAT_JID` | `data[]` (`user_jid`, `role`) |
| calls | `wacli-reader calls list --limit 20` | `data.calls` (`chat_name`, `timestamp`, `direction`, `outcome`) |
| polls of a chat | `wacli-reader polls list --chat CHAT_JID --limit 20` | `data.polls` (`msg_id`, `question`) |
| votes of one poll | `wacli-reader poll show --chat CHAT_JID --id MSG_ID` | `data.aggregates` (option: votes) |
| does the archive work | `wacli-reader doctor` | `data` |

Always pass `--limit` where the table shows it.

Options: `--after`/`--before` take `YYYY-MM-DD` in UTC. `--after 2026-11-01 --before 2026-11-02` is 1 November. `--from-me` = only the user's own messages. `messages search` also takes `--has-media` and `--type image|video|audio|document`, but only together with search words.

## Reading messages

Results are newest first, except with `--asc`.

- **Text:** `DisplayText`. For media it is a description such as "Sent image"; the caption is in `MediaCaption`.
- **Sender:** `FromMe: true` = the user. Otherwise `SenderName`, or `SenderJID` if the name is empty.
- **Time:** `Timestamp` is UTC. Convert it to the user's time zone in your answer.
- **Reactions:** a message with `ReactionToID` set is a reaction ("Reacted 👍 to …"), not a message. Skip it when asked for messages.
- **Deleted:** `Revoked` or `DeletedForMe` is `true`.
- `MsgID` is the `MSG_ID` for `messages show` and `messages context`. Ignore fields not named here.

## Search

Search matches whole words, and every word must occur. Use 1 to 3 key words, not a sentence. If nothing is found, retry with fewer words or another form (`invoice`, `invoices`, `bill`). Still nothing: say nothing was found.

## More results

If `data.page.has_more` is `true` and you need more, repeat the same command with `--cursor` plus the value of `data.page.next_cursor`. The `More local messages:` line on stderr is a hint, not an error.

## Errors

| Error contains | Do this |
|---|---|
| `ambiguous local name` | pick the right JID from the error, or ask the user |
| `no local exact name match`, `sql: no rows` | do Step 1 and use the `jid` it returns |
| `cursor` or `checkpoint` | run the command again without `--cursor`; for `--since`, get a new checkpoint |
| `unknown command`, `unknown flag` | use only the commands in this file |
| `no such column`, `no such table`, `failed to open wacli.db` | Stop. Say the WhatsApp archive is unavailable or its sync service needs an upgrade |

## Media files

A media message has `MediaType` and a file path in `LocalPath`. `LocalPath` empty (always for `gif`): say the media was not archived locally, and stop.

1. Copy the file with `cat`, never `cp`. Use `jpg` as `EXT` for images, otherwise the extension of `LocalPath`. This also removes earlier copies:
   `rm -rf /tmp/openclaw/wa && mkdir -p /tmp/openclaw/wa && cat "LOCAL_PATH" > /tmp/openclaw/wa/MSG_ID.EXT`
2. Use the copy, never `LocalPath` itself:
   - **To send it to the user:** put `MEDIA:/tmp/openclaw/wa/MSG_ID.EXT` on its own line in your reply. Do not use the `message` tool.
   - **To look at a photo:** call `dispatch_action` with `{"id": "openclaw:core:view_image", "args": {"path": "/tmp/openclaw/wa/MSG_ID.jpg", "prompt": "Describe this photo."}}`, or the `view_image` tool directly if you have it. No `tool_search` needed. The `read` tool cannot show images.

## Rarely needed

- Save a chat to a file: `wacli-reader messages export --chat CHAT_JID --limit 5000 --output /tmp/thread.json` (oldest first).
- Changes since an earlier point: `wacli-reader messages changes --checkpoint` gives `data.next_checkpoint`. Later, `wacli-reader messages changes --since CHECKPOINT --limit 100` lists `data.changes` (`operation`, `chat_jid`, `msg_id`, no text).
