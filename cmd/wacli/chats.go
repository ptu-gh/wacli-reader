package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/app"
	types "github.com/openclaw/wacli/internal/jid"
	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

func newChatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chats",
		Short: "List chats from the local DB",
	}
	cmd.AddCommand(newChatsListCmd(flags))
	cmd.AddCommand(newChatsShowCmd(flags))
	return cmd
}

func newChatsListCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	var archived, noArchived bool
	var pinned, noPinned bool
	var muted, noMuted bool
	var unread, noUnread bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List chats",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateBoolFilter("archived", archived, noArchived); err != nil {
				return err
			}
			if err := validateBoolFilter("pinned", pinned, noPinned); err != nil {
				return err
			}
			if err := validateBoolFilter("muted", muted, noMuted); err != nil {
				return err
			}
			if err := validateBoolFilter("unread", unread, noUnread); err != nil {
				return err
			}

			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			filter := store.ChatListFilter{
				Query:    query,
				Limit:    limit,
				Archived: boolFilter(archived, noArchived),
				Pinned:   boolFilter(pinned, noPinned),
				Muted:    boolFilter(muted, noMuted),
				Unread:   boolFilter(unread, noUnread),
			}
			chats, err := a.DB().ListChatsFiltered(filter)
			if err != nil {
				return err
			}
			chats = resolveStoredChats(ctx, a, chats)
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, chats)
			}

			fullOutput := fullTableOutput(flags.fullOutput)
			w := newTableWriter(os.Stdout)
			fmt.Fprintln(w, "KIND\tNAME\tJID\tLAST\tFLAGS")
			for _, c := range chats {
				name := c.Name
				if name == "" {
					name = c.JID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", sanitize(c.Kind), tableCell(name, 28, fullOutput), sanitize(c.JID), c.LastMessageTS.Local().Format("2006-01-02 15:04:05"), chatFlagsString(c))
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search query")
	cmd.Flags().IntVar(&limit, "limit", 50, "limit")
	cmd.Flags().BoolVar(&archived, "archived", false, "show only archived chats")
	cmd.Flags().BoolVar(&noArchived, "no-archived", false, "exclude archived chats")
	cmd.Flags().BoolVar(&pinned, "pinned", false, "show only pinned chats")
	cmd.Flags().BoolVar(&noPinned, "no-pinned", false, "exclude pinned chats")
	cmd.Flags().BoolVar(&muted, "muted", false, "show only muted chats")
	cmd.Flags().BoolVar(&noMuted, "no-muted", false, "exclude muted chats")
	cmd.Flags().BoolVar(&unread, "unread", false, "show only unread chats")
	cmd.Flags().BoolVar(&noUnread, "no-unread", false, "exclude unread chats")
	return cmd
}

func newChatsShowCmd(flags *rootFlags) *cobra.Command {
	var jid string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jid == "" {
				return fmt.Errorf("--jid is required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			c, err := getChatForDisplay(ctx, a, jid)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, c)
			}
			fmt.Fprintf(os.Stdout, "JID: %s\nKind: %s\nName: %s\nLast: %s\nArchived: %t\nPinned: %t\nMuted: %t\nMuted until: %s\nUnread: %t\nUnread count: %d\n",
				sanitize(c.JID), sanitize(c.Kind), sanitize(c.Name), c.LastMessageTS.Local().Format(time.RFC3339), c.Archived, c.Pinned, c.Muted(), formatMutedUntil(c.MutedUntil), c.Unread, c.UnreadCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&jid, "jid", "", "chat JID")
	return cmd
}

// resolveStoredChats keeps upstream's signature. Upstream rewrites LID-keyed
// chats to their phone-number JID and name through session.db; wacli-reader
// never opens session.db, so chats are shown as wacli.db stores them —
// upstream's own result when session.db is absent.
func resolveStoredChats(ctx context.Context, a *app.App, chats []store.Chat) []store.Chat {
	return chats
}

func getChatForDisplay(ctx context.Context, a *app.App, rawJID string) (store.Chat, error) {
	chat, err := a.DB().GetChat(rawJID)
	if err == nil {
		return resolveStoredChatForDisplay(ctx, a, chat), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.Chat{}, err
	}

	chatJIDs := mappedChatJIDs(ctx, a, rawJID)
	for _, chatJID := range chatJIDs {
		if chatJID == rawJID {
			continue
		}
		chat, err = a.DB().GetChat(chatJID)
		if err == nil {
			return resolveStoredChatForDisplay(ctx, a, chat), nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return store.Chat{}, err
		}
	}
	return store.Chat{}, sql.ErrNoRows
}

func resolveStoredChatForDisplay(ctx context.Context, a *app.App, chat store.Chat) store.Chat {
	return resolveStoredChats(ctx, a, []store.Chat{chat})[0]
}

func mappedChatJIDs(ctx context.Context, a *app.App, rawJID string) []string {
	jid, err := types.ParseJID(strings.TrimSpace(rawJID))
	if err != nil {
		return []string{rawJID}
	}
	// Upstream adds the PN/LID twin from session.db here; the fork cannot.
	jids := []types.JID{jid}
	return jidStrings(jids)
}
