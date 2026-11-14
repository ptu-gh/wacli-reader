package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/openclaw/wacli/internal/app"
	types "github.com/openclaw/wacli/internal/jid"
	wa "github.com/openclaw/wacli/internal/jid"
	"github.com/openclaw/wacli/internal/store"
)

// messageChatJIDFilter is upstream's, minus the session.db lookup that adds
// the chat's LID (or phone-number) twin to the filter. wacli-reader never
// opens session.db, so a chat stored under the other half of a PN/LID pair is
// not matched — upstream's own behaviour when session.db is absent.
func messageChatJIDFilter(ctx context.Context, a *app.App, chat string) ([]string, error) {
	chat = strings.TrimSpace(chat)
	if chat == "" {
		return nil, nil
	}
	resolved, err := localMessageIdentifier(a.DB(), chat, true)
	if err != nil {
		return nil, err
	}
	jid, err := wa.ParseUserOrJID(resolved)
	if err != nil {
		return nil, err
	}
	jids := []types.JID{canonicalMessageFilterJID(jid)}
	return jidStrings(jids), nil
}

func canonicalMessageFilterJID(jid types.JID) types.JID {
	if jid.Server == types.DefaultUserServer {
		return jid.ToNonAD()
	}
	return jid
}

func jidStrings(jids []types.JID) []string {
	out := make([]string, 0, len(jids))
	seen := make(map[string]struct{}, len(jids))
	for _, jid := range jids {
		if jid.IsEmpty() {
			continue
		}
		s := jid.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func resolveMessageSenderNames(ctx context.Context, a *app.App, msgs []store.Message) []store.Message {
	return resolveMessageSenderNamesInView(ctx, a, a.DB(), msgs)
}

// resolveMessageSenderNamesInView keeps upstream's signature. Upstream names
// LID-only senders through session.db's LID→phone map; wacli-reader has no
// such map, so messages keep the sender name wacli.db stored — upstream's own
// result when session.db is absent.
func resolveMessageSenderNamesInView(ctx context.Context, a *app.App, db *store.DB, msgs []store.Message) []store.Message {
	return msgs
}

func getMessageByChatFilter(db *store.DB, chatJIDs []string, id string) (store.Message, error) {
	var notFound error
	for _, chatJID := range chatJIDs {
		m, err := db.GetMessage(chatJID, id)
		if err == nil {
			// GetMessage reads its own columns, so the delivery state has to be
			// filled in here for `show` to agree with `list`.
			if counts, cerr := db.MessageReceipts(m.ChatJID, m.MsgID); cerr == nil {
				m.DeliveredTo = counts.Delivered
				m.ReadBy = counts.Read
			}
			return m, nil
		}
		if !isNoRows(err) {
			return store.Message{}, err
		}
		notFound = err
	}
	if notFound != nil {
		return store.Message{}, notFound
	}
	return store.Message{}, sql.ErrNoRows
}

func getMessageContextByChatFilter(db *store.DB, chatJIDs []string, id string, before, after int) ([]store.Message, error) {
	var notFound error
	for _, chatJID := range chatJIDs {
		msgs, err := db.MessageContext(chatJID, id, before, after)
		if err == nil {
			return msgs, nil
		}
		if !isNoRows(err) {
			return nil, err
		}
		notFound = err
	}
	if notFound != nil {
		return nil, notFound
	}
	return nil, sql.ErrNoRows
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
