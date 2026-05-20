package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

func newPollsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polls",
		Short: "List polls stored locally",
	}
	cmd.AddCommand(newPollsListCmd(flags))
	return cmd
}

func newPollCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Inspect a single poll",
	}
	cmd.AddCommand(newPollShowCmd(flags))
	return cmd
}

func newPollsListCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List polls stored locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

			polls, err := a.DB().ListPolls(store.PollListFilter{
				ChatJID: strings.TrimSpace(chat),
				Limit:   limit,
			})
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
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

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
				return out.WriteJSON(os.Stdout, pollShowPayload(poll, votes, aggregates))
			}
			renderPollShow(os.Stdout, poll, votes, aggregates)
			return nil
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID")
	cmd.Flags().StringVar(&msgID, "id", "", "poll message ID")
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

func renderPollsList(w io.Writer, polls []store.Poll) {
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

func pollShowPayload(poll store.Poll, votes []store.PollVote, aggregates map[string]int) pollShowJSON {
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
		out.Voters = append(out.Voters, pollVoterPayload{
			JID:           v.VoterJID,
			Selected:      v.Selected,
			UnknownHashes: v.UnknownHashes,
			VotedAt:       v.VotedAt.Format(time.RFC3339),
		})
	}
	return out
}

func renderPollShow(w io.Writer, poll store.Poll, votes []store.PollVote, aggregates map[string]int) {
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
		selected := make([]string, 0, len(v.Selected))
		for _, option := range v.Selected {
			selected = append(selected, sanitize(option))
		}
		fmt.Fprintf(w, "  %s — %s [%s]\n", sanitize(v.VoterJID), strings.Join(selected, ", "), v.VotedAt.Format(time.RFC3339))
	}
}
