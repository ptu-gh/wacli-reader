package main

// Read-only slice of upstream's cmd/wacli/poll_cmd.go. Upstream's file also
// defines `poll vote` and the recipient-resolution helpers (`resolveRecipient`,
// `pollChatJIDCandidates`), which reach the network and whatsmeow; those are
// deliberately not shipped here. Function order, signatures, and section
// comments follow upstream so a future rebase diffs cleanly — the surviving
// code should read as upstream's with the write half cut out.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/app"
	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

func newPollCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Inspect a single poll",
	}
	cmd.AddCommand(newPollShowCmd(flags))
	return cmd
}

func newPollsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polls",
		Short: "List polls",
	}
	cmd.AddCommand(newPollsListCmd(flags))
	return cmd
}

// ---- show -----------------------------------------------------------------

func newPollShowCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var msgID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a poll's question, options, and votes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chat) == "" || strings.TrimSpace(msgID) == "" {
				return fmt.Errorf("--chat and --id are required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			poll, err := a.DB().GetPoll(chat, msgID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("poll %s not found in local store for chat %s", msgID, chat)
				}
				return err
			}
			votes, err := a.DB().ListPollVotes(poll.ChatJID, msgID)
			if err != nil {
				return err
			}
			aggregates := aggregatePollVotes(poll.Options, votes)

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, pollShowPayload(a, ctx, poll, votes, aggregates))
			}
			renderPollShow(os.Stdout, a, ctx, poll, votes, aggregates)
			return nil
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID")
	cmd.Flags().StringVar(&msgID, "id", "", "poll message ID")
	return cmd
}

func aggregatePollVotes(options []string, votes []store.PollVote) map[string]int {
	out := make(map[string]int, len(options))
	for _, o := range options {
		out[o] = 0
	}
	for _, v := range votes {
		for _, sel := range v.Selected {
			out[sel]++
		}
	}
	return out
}

type pollVoterPayload struct {
	JID           string   `json:"jid"`
	Name          string   `json:"name,omitempty"`
	Selected      []string `json:"selected"`
	UnknownHashes []string `json:"unknown_hashes,omitempty"`
	VotedAt       string   `json:"voted_at"`
}

type pollShowJSON struct {
	Question        string             `json:"question"`
	Options         []string           `json:"options"`
	SelectableCount uint32             `json:"selectable_count"`
	ChatJID         string             `json:"chat_jid"`
	MsgID           string             `json:"msg_id"`
	SenderJID       string             `json:"sender_jid,omitempty"`
	CreatedAt       string             `json:"created_at"`
	Aggregates      map[string]int     `json:"aggregates"`
	Voters          []pollVoterPayload `json:"voters"`
}

func pollShowPayload(a *app.App, ctx context.Context, poll store.Poll, votes []store.PollVote, aggregates map[string]int) pollShowJSON {
	out := pollShowJSON{
		Question:        poll.Question,
		Options:         poll.Options,
		SelectableCount: poll.SelectableCount,
		ChatJID:         poll.ChatJID,
		MsgID:           poll.MsgID,
		SenderJID:       poll.SenderJID,
		CreatedAt:       poll.CreatedAt.Format(time.RFC3339),
		Aggregates:      aggregates,
		Voters:          make([]pollVoterPayload, 0, len(votes)),
	}
	for _, v := range votes {
		name := resolveVoterName(a, ctx, v.VoterJID)
		out.Voters = append(out.Voters, pollVoterPayload{
			JID:           v.VoterJID,
			Name:          name,
			Selected:      v.Selected,
			UnknownHashes: v.UnknownHashes,
			VotedAt:       v.VotedAt.Format(time.RFC3339),
		})
	}
	return out
}

// resolveVoterName keeps upstream's local-contact lookup and drops its tail,
// which parses the JID with whatsmeow and falls back to the session.db-backed
// LocalResolver. A voter with no local contact row stays a bare JID here.
func resolveVoterName(a *app.App, ctx context.Context, jid string) string {
	jid = strings.TrimSpace(jid)
	if jid == "" {
		return ""
	}
	if a != nil && a.DB() != nil {
		if c, err := a.DB().GetContact(jid); err == nil {
			if name := strings.TrimSpace(c.Name); name != "" {
				return name
			}
		}
	}
	return ""
}

func renderPollShow(w *os.File, a *app.App, ctx context.Context, poll store.Poll, votes []store.PollVote, aggregates map[string]int) {
	fmt.Fprintf(w, "Poll: %s\n", sanitize(poll.Question))
	fmt.Fprintf(w, "Chat: %s   Msg: %s   Selectable: %d\n", sanitize(poll.ChatJID), sanitize(poll.MsgID), poll.SelectableCount)
	fmt.Fprintf(w, "\nResults (%d voter(s)):\n", len(votes))
	for _, opt := range poll.Options {
		fmt.Fprintf(w, "  %3d  %s\n", aggregates[opt], sanitize(opt))
	}
	if len(votes) == 0 {
		fmt.Fprintln(w, "\nNo votes yet.")
		return
	}
	fmt.Fprintln(w, "\nVoters:")
	for _, v := range votes {
		name := resolveVoterName(a, ctx, v.VoterJID)
		who := v.VoterJID
		if name != "" && name != v.VoterJID {
			who = fmt.Sprintf("%s (%s)", name, v.VoterJID)
		}
		selected := make([]string, 0, len(v.Selected))
		for _, option := range v.Selected {
			selected = append(selected, sanitize(option))
		}
		fmt.Fprintf(w, "  %s — %s [%s]\n", sanitize(who), strings.Join(selected, ", "), v.VotedAt.Format(time.RFC3339))
	}
}

// ---- list -----------------------------------------------------------------

func newPollsListCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List polls stored locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			filter := store.PollListFilter{Limit: limit}
			if strings.TrimSpace(chat) != "" {
				filter.ChatJID = strings.TrimSpace(chat)
			}
			polls, err := a.DB().ListPolls(filter)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"polls": pollsToJSON(polls)})
			}
			renderPollsList(os.Stdout, polls)
			return nil
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "filter by chat JID")
	cmd.Flags().IntVar(&limit, "limit", 50, "max polls to return (0 = unlimited)")
	return cmd
}

type pollListItemJSON struct {
	ChatJID         string   `json:"chat_jid"`
	MsgID           string   `json:"msg_id"`
	Question        string   `json:"question"`
	Options         []string `json:"options"`
	SelectableCount uint32   `json:"selectable_count"`
	SenderJID       string   `json:"sender_jid,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

func pollsToJSON(polls []store.Poll) []pollListItemJSON {
	out := make([]pollListItemJSON, 0, len(polls))
	for _, p := range polls {
		out = append(out, pollListItemJSON{
			ChatJID:         p.ChatJID,
			MsgID:           p.MsgID,
			Question:        p.Question,
			Options:         p.Options,
			SelectableCount: p.SelectableCount,
			SenderJID:       p.SenderJID,
			CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		})
	}
	return out
}

func renderPollsList(w *os.File, polls []store.Poll) {
	if len(polls) == 0 {
		fmt.Fprintln(w, "No polls.")
		return
	}
	for _, p := range polls {
		fmt.Fprintf(w, "%s  %s\n  chat=%s  options=%d  selectable=%d  id=%s\n",
			p.CreatedAt.Format(time.RFC3339),
			sanitize(p.Question),
			sanitize(p.ChatJID),
			len(p.Options),
			p.SelectableCount,
			sanitize(p.MsgID),
		)
	}
}
