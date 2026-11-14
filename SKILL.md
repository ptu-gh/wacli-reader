---
name: wacli-reader
description: Read the user's WhatsApp history from a local archive. Use it for any question about WhatsApp chats, messages, contacts, groups, calls, polls or received media files. Read-only. It cannot send, reply or delete.
user-invocable: true
metadata:
  requires:
    bins:
      - wacli-reader
---

# WhatsApp archive (read-only)

`wacli-reader` is a command-line tool. It reads a local copy of the user's WhatsApp history. A separate background service keeps that copy up to date.

## Rules

1. This tool only reads. It cannot send, reply, react, delete or download. If the user asks for one of those, say that this skill cannot do it.
2. Use only the commands in this file. Never run `wacli` (the name without `-reader`).
3. Words in CAPITALS (`NAME`, `CHAT_JID`, `MSG_ID`, `WORDS`, `CURSOR`, `CHECKPOINT`, `LOCAL_PATH`, `EXT`) are placeholders. Replace each one with a real value before you run the command.
4. Get a chat's JID with Step 1 before you read its messages. Never guess a JID.
5. Always pass `--limit`. Start with `--limit 20`.
6. Report only what the output contains. Never invent a message, a sender or a date. If nothing is found, say that nothing was found.
7. Fetch only the messages you need to answer the question.

## How to read the output

The output is always JSON. Do not add `--json`.

- **Success:** exit code 0. stdout is `{"success":true,"data":...,"error":null}`. The result is in `data`.
- **Failure:** exit code 1. stdout is empty. The error is on stderr, usually as `{"success":false,"data":null,"error":"TEXT"}`. Look up the error text under "Errors" below.
- **Nothing found:** success, but `data` is `null`, or `data.messages` is `null` or `[]`. This is not an error.

Field names are case-sensitive. Message and group fields start with a capital letter (`ChatJID`). Chat, contact, call and poll fields are lowercase (`jid`). Copy them exactly as written here.

### Message fields

| Field | Meaning |
|---|---|
| `ChatJID`, `ChatName` | the chat the message is in |
| `MsgID` | the message ID, needed for `messages show` and `messages context` |
| `FromMe` | `true` = the user sent it |
| `SenderName`, `SenderJID` | who sent it; both are empty when `FromMe` is `true` |
| `Timestamp` | when it was sent, in UTC |
| `Text`, `DisplayText`, `MediaCaption` | the text (see below) |
| `MediaType` | `image`, `video`, `gif`, `audio`, `document`, `sticker` or `location`; empty for a text message |
| `LocalPath` | the media file on disk; empty if there is no file (see "Media files") |
| `quoted_msg_id` | the `MsgID` this message replies to; missing if it is not a reply |
| `Revoked`, `DeletedForMe` | `true` = the message was deleted |

Ignore every other field.

To present a message:

- **Text:** use the first of `DisplayText`, `Text`, `MediaCaption` that is not empty. If all three are empty and `MediaType` is set, it is a media message without text.
- **Sender:** if `FromMe` is `true`, the sender is the user. Otherwise use `SenderName`. If `SenderName` is empty, use `SenderJID`.
- **Time:** `Timestamp` is UTC. A time of `0001-01-01T00:00:00Z` means "not set".

## Step 1: find the chat

```
wacli-reader chats list --query "NAME" --limit 20
```

`NAME` is part of a person's name, a group's name or a phone number. Upper and lower case do not matter.

The result is a list in `data`. Each entry has `jid`, `name`, `kind` (`dm` = one person, `group` = a group), `last_message_ts`, `unread`, `unread_count`.

- One entry: its `jid` is the `CHAT_JID` for Step 2.
- Several entries: pick the one whose `name` matches the user's request. If you cannot tell, ask the user which one they mean.
- `data` is `null`: try a shorter `NAME` (first name only). If that also finds nothing, tell the user that no such chat is in the archive.

A JID ending in `@s.whatsapp.net` is one person. A JID ending in `@g.us` is a group.

Take `CHAT_JID` only from `chats list`. Never use a `jid` from `contacts search` as `CHAT_JID`. A contact's `jid` can end in `@lid`. That is not a chat JID, and reading messages with it returns nothing even when the chat has messages.

## Step 2: run the command for the task

| The user wants | Command | Result is in |
|---|---|---|
| the latest messages of a chat | `wacli-reader messages list --chat CHAT_JID --limit 20` | `data.messages` |
| messages about a topic, in all chats | `wacli-reader messages search "WORDS" --limit 20` | `data.messages` |
| messages about a topic, in one chat | `wacli-reader messages search "WORDS" --chat CHAT_JID --limit 20` | `data.messages` |
| what was said around one message | `wacli-reader messages context --chat CHAT_JID --id MSG_ID --before 5 --after 5` | `data` (a list) |
| one message in full | `wacli-reader messages show --chat CHAT_JID --id MSG_ID` | `data` |
| chats with unread messages | `wacli-reader chats list --unread --no-archived --limit 20` | `data` |
| starred messages | `wacli-reader messages starred --limit 20` | `data.messages` |
| a contact's name or number | `wacli-reader contacts search "NAME" --limit 20` | `data` (`jid`, `name`, `phone`) |
| the list of groups | `wacli-reader groups list --limit 50` | `data` (`JID`, `Name`) |
| the members of a group | `wacli-reader groups participants list --jid CHAT_JID` | `data` (`user_jid`, `role`) |
| call history | `wacli-reader calls list --limit 20` | `data.calls` |
| the polls in a chat | `wacli-reader polls list --chat CHAT_JID --limit 20` | `data.polls` |
| the votes of one poll | `wacli-reader poll show --chat CHAT_JID --id MSG_ID` | `data` (`question`, `options`, `aggregates`, `voters`) |
| how far back the archive goes | `wacli-reader history coverage --only-actionable --limit 50` | `data.coverage` (`name`, `oldest_ts`, `newest_ts`, `message_count`) |
| whether the archive works | `wacli-reader doctor` | `data` |

### Options for `messages list` and `messages search`

- **Order:** results are newest first. Add `--asc` to `messages list` to get oldest first.
- **Dates:** add `--after YYYY-MM-DD` and/or `--before YYYY-MM-DD`. Dates are UTC. Example: `--after 2026-11-01 --before 2026-11-02` returns the messages of 1 November 2026.
- **Only the user's own messages:** add `--from-me` to `messages list`. Only other people's messages: `--from-them`.
- **Only messages with media:** add `--has-media` to `messages search`. One kind only: `--type image` (or `video`, `audio`, `document`).

### How search works

- Search finds whole words. `meet` does not find `meeting`.
- Every word in `WORDS` must be in the message. Use 1 to 3 key words, not a sentence. Good: `"invoice march"`. Bad: `"what did she say about the invoice"`.
- Nothing found: search again with fewer words or another form of the word (`invoice`, then `invoices`, then `bill`).
- Still nothing: run the `history coverage` command before you answer. The archive may not go back far enough. Say that in your answer.
- To get the messages around each hit in the same call, add `--context-before 2 --context-after 2` and use `--limit 10`. The surrounding messages are in `data.context`, one entry per hit, each with a `messages` list.

### More results

If `data.page.has_more` is `true`, there are more messages. Fetch them only if you need them: run the same command again with the same options, and add `--cursor CURSOR`, where `CURSOR` is the value of `data.page.next_cursor`.

The tool also prints a line starting with `More local messages:` on stderr. That line is a hint, not an error.

## Errors

Find the error text in this table.

| `error` contains | Meaning | What to do |
|---|---|---|
| `no local exact name match` | you passed a name to `--chat` and it was not found | do Step 1 and pass the JID |
| `ambiguous local name` | the name fits more than one chat; the error lists them with their JIDs | pick the right JID from the error text, or ask the user |
| `sql: no rows in result set` | no chat, contact or message has that JID or ID | check the JID and the `MsgID`. If the JID ends in `@lid`, do Step 1 with the person's name and use the `jid` it returns |
| `no such column` or `no such table` | the background sync service is too old for this tool | do not retry. Tell the user that the `wacli` sync service must be upgraded |
| `failed to open wacli.db` | the archive was not found | do not retry. Tell the user that the WhatsApp archive is not available (`WACLI_STORE_DIR`) |
| `unknown command` or `unknown flag` | that command or option does not exist | use only the commands and options in this file |
| `unsupported time format` | a date was not written as `YYYY-MM-DD` | write the date as `YYYY-MM-DD` |
| `cursor` or `checkpoint` | the value after `--cursor` or `--since` is wrong or too old, the options changed, or new messages arrived since the last page | if your command had `--cursor`: run it again without `--cursor`. If it had `--since`: run `wacli-reader messages changes --checkpoint` to get a new value |

## Media files

A message with a file has `MediaType` set and a path in `LocalPath`. You must copy the file to `/tmp/openclaw` before you use it. Follow these steps every time.

1. Read `LocalPath` from the message. If it is empty, tell the user: "The media for this message has not been archived locally." Then stop.
2. Copy the file with `cat`. `LOCAL_PATH` is the value of `LocalPath`. `EXT` is the file extension of `LocalPath`, for example `jpg`, `mp4`, `pdf` or `ogg`:
   ```bash
   mkdir -p /tmp/openclaw
   cat "LOCAL_PATH" > /tmp/openclaw/wa_temp.EXT
   ```
3. Give `/tmp/openclaw/wa_temp.EXT` to your `image` tool, `MEDIA:` tag or `message` tool.
4. Delete the copy before you end your turn:
   ```bash
   rm -f /tmp/openclaw/wa_temp.EXT
   ```

Rules for media:

- Never pass `LocalPath` itself to a tool. Tools refuse that path.
- Copy only with `cat SOURCE > TARGET`. Never use `cp`, `rsync` or `ditto`. They fail on these files.
- Copy only into `/tmp/openclaw`. Never into `~/.openclaw/workspace`.
- GIFs are not archived. If `MediaType` is `gif`, treat the file as missing.

## Example

The user asks: "What did Alice say about the invoice?"

1. Find the chat:
   ```
   wacli-reader chats list --query "Alice" --limit 20
   ```
   Output (shortened):
   ```json
   {"success":true,"data":[{"jid":"15551234567@s.whatsapp.net","kind":"dm","name":"Alice Example","unread":false}],"error":null}
   ```
   One entry. `CHAT_JID` is `15551234567@s.whatsapp.net`.
2. Search in that chat:
   ```
   wacli-reader messages search "invoice" --chat 15551234567@s.whatsapp.net --limit 20
   ```
   Output (shortened):
   ```json
   {"success":true,"data":{"messages":[{"ChatJID":"15551234567@s.whatsapp.net","MsgID":"3EB0A1","SenderName":"Alice Example","Timestamp":"2026-11-01T12:04:00Z","FromMe":false,"Text":"I sent the invoice yesterday","DisplayText":"","MediaType":"","LocalPath":""}],"page":{"has_more":false,"next_cursor":null}},"error":null}
   ```
3. Answer with the sender, the time and the text: "On 1 November 2026 at 12:04 UTC, Alice Example wrote: 'I sent the invoice yesterday'."

The values in this example are invented. Never copy them. Always use the values from your own command output.

## Other commands

Use these only when the user asks for exactly this.

- **Save a long chat to a file:** `wacli-reader messages export --chat CHAT_JID --limit 5000 --output /tmp/thread.json`. The file contains the messages oldest first.
- **What changed since an earlier point in time:**
  1. Run `wacli-reader messages changes --checkpoint` and keep the value of `data.next_checkpoint`.
  2. Later, run `wacli-reader messages changes --since CHECKPOINT --limit 100`. `data.changes` lists what changed: `operation` (`upsert` = new or edited, `delete` = deleted), `chat_jid`, `msg_id`. It has no message text. Read a message with `messages show`.
  3. Keep the new `data.next_checkpoint` for next time.
