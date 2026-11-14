package main

// Read-only slice of upstream's cmd/wacli/history.go. Upstream's file also
// defines `history fill` and `history backfill`, which reach the network and
// write to the store; those are deliberately not shipped here. Everything
// below is copied verbatim (modulo the fork's newApp signature) so a future
// rebase diffs cleanly against upstream.
//
// writeHistoryCoverageTable keeps its includeSelected parameter even though
// only `history fill` passes true, for the same reason internal/store keeps
// upstream's write helpers: staying byte-identical to upstream is what makes
// rebases cheap.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History coverage",
	}
	cmd.AddCommand(newHistoryCoverageCmd(flags))
	return cmd
}

func newHistoryCoverageCmd(flags *rootFlags) *cobra.Command {
	var chats []string
	var query string
	var kind string
	var limit int
	var includeBlocked bool
	var onlyActionable bool

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Show local archive coverage by chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(cmd.Context(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			coverage, err := a.DB().ListHistoryCoverage(store.ListHistoryCoverageParams{
				ChatJIDs:       chats,
				Query:          query,
				Kind:           kind,
				Limit:          limit,
				IncludeBlocked: includeBlocked,
				OnlyActionable: onlyActionable,
			})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"coverage": coverage})
			}
			return writeHistoryCoverageTable(os.Stdout, coverage, fullTableOutput(flags.fullOutput), false)
		},
	}
	cmd.Flags().StringSliceVar(&chats, "chat", nil, "chat JID to inspect (repeatable)")
	cmd.Flags().StringVar(&query, "query", "", "filter chats by local name or JID")
	cmd.Flags().StringVar(&kind, "kind", "", "chat kind filter (dm|group|broadcast|newsletter|unknown)")
	cmd.Flags().IntVar(&limit, "limit", 100, "limit rows")
	cmd.Flags().BoolVar(&includeBlocked, "include-blocked", false, "include chats without a local message anchor")
	cmd.Flags().BoolVar(&onlyActionable, "only-actionable", false, "show only chats with a local message anchor")
	return cmd
}

func writeHistoryCoverageTable(dst io.Writer, coverage []store.HistoryCoverage, fullOutput, includeSelected bool) error {
	w := newTableWriter(dst)
	if includeSelected {
		fmt.Fprintln(w, "SELECTED\tCHAT\tKIND\tMESSAGES\tOLDEST\tNEWEST\tSTATUS\tDETAIL")
	} else {
		fmt.Fprintln(w, "CHAT\tKIND\tMESSAGES\tOLDEST\tNEWEST\tSTATUS\tDETAIL")
	}
	for _, c := range coverage {
		name := c.Name
		if strings.TrimSpace(name) == "" {
			name = c.ChatJID
		}
		detail := historyCoverageDetail(c)
		selected := ""
		if includeSelected {
			if c.Status == store.HistoryCoverageStatusReady {
				selected = "yes"
			} else {
				selected = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				selected,
				tableCell(name, 32, fullOutput),
				sanitize(c.Kind),
				c.MessageCount,
				formatHistoryDate(c.OldestTS),
				formatHistoryDate(c.NewestTS),
				c.Status,
				tableCell(detail, 36, fullOutput),
			)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			tableCell(name, 32, fullOutput),
			c.Kind,
			c.MessageCount,
			formatHistoryDate(c.OldestTS),
			formatHistoryDate(c.NewestTS),
			c.Status,
			tableCell(detail, 36, fullOutput),
		)
	}
	_ = w.Flush()
	return nil
}

func historyCoverageDetail(c store.HistoryCoverage) string {
	if c.BlockedReason != "" {
		return c.BlockedReason
	}
	return c.ChatJID
}

func formatHistoryDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02")
}
