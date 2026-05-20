package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/openclaw/wacli/internal/config"
	"github.com/openclaw/wacli/internal/out"
	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

type doctorStoreStats struct {
	Messages   int64  `json:"messages"`
	Chats      int64  `json:"chats"`
	Contacts   int64  `json:"contacts"`
	Groups     int64  `json:"groups"`
	LastSyncAt string `json:"last_sync_at,omitempty"`
}

type doctorReport struct {
	StoreDir   string            `json:"store_dir"`
	FTSEnabled bool              `json:"fts_enabled"`
	Store      *doctorStoreStats `json:"store,omitempty"`
	StoreError string            `json:"store_error,omitempty"`
}

func doctorStoreStatsFromStoreStats(stats store.StoreStats) doctorStoreStats {
	out := doctorStoreStats{
		Messages: stats.Messages,
		Chats:    stats.Chats,
		Contacts: stats.Contacts,
		Groups:   stats.Groups,
	}
	if stats.LastMessageTS > 0 {
		out.LastSyncAt = time.Unix(stats.LastMessageTS, 0).UTC().Format(time.RFC3339)
	}
	return out
}

func writeDoctorReport(w io.Writer, rep doctorReport) {
	tw := newTableWriter(w)
	fmt.Fprintf(tw, "STORE\t%s\n", rep.StoreDir)
	fmt.Fprintf(tw, "FTS5\t%v\n", rep.FTSEnabled)
	if rep.Store != nil {
		fmt.Fprintf(tw, "MESSAGES\t%d\n", rep.Store.Messages)
		fmt.Fprintf(tw, "CHATS\t%d\n", rep.Store.Chats)
		fmt.Fprintf(tw, "CONTACTS\t%d\n", rep.Store.Contacts)
		fmt.Fprintf(tw, "GROUPS\t%d\n", rep.Store.Groups)
		if rep.Store.LastSyncAt != "" {
			fmt.Fprintf(tw, "LAST_SYNC\t%s\n", rep.Store.LastSyncAt)
		}
	}
	_ = tw.Flush()
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnostics for store/search",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			storeDir := flags.storeDir
			if storeDir == "" {
				storeDir = config.DefaultStoreDir()
			}
			storeDir, _ = filepath.Abs(storeDir)

			var storeErr string
			a, err := newApp(flags)
			if err != nil {
				storeErr = err.Error()
			} else {
				defer a.Close()
			}

			var stats *doctorStoreStats
			var ftsEnabled bool
			if a != nil {
				if raw, err := a.DB().Stats(); err == nil {
					converted := doctorStoreStatsFromStoreStats(raw)
					stats = &converted
				}
				ftsEnabled = a.DB().HasFTS()
			}

			rep := doctorReport{
				StoreDir:   storeDir,
				FTSEnabled: ftsEnabled,
				Store:      stats,
				StoreError: storeErr,
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, rep)
			}

			writeDoctorReport(os.Stdout, rep)

			if rep.StoreError != "" {
				fmt.Fprintf(os.Stdout, "\nERROR: store could not be opened: %s\n", rep.StoreError)
				fmt.Fprintln(os.Stdout, "Tip: wacli-reader needs a wacli.db that upstream wacli has already synced. See https://github.com/openclaw/wacli.")
			}
			return nil
		},
	}

	return cmd
}
