package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

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

// doctorReport is a strict subset of upstream's: the lock, auth, connection
// and session fields need internal/lock and session.db, which the reader does
// not have. The schema fields are fork-local — the reader never migrates, so a
// store last written by an older wacli is its most likely failure.
type doctorReport struct {
	StoreDir              string            `json:"store_dir"`
	FTSEnabled            bool              `json:"fts_enabled"`
	SchemaVersion         int               `json:"schema_version,omitempty"`
	ExpectedSchemaVersion int               `json:"expected_schema_version"`
	SchemaWarning         string            `json:"schema_warning,omitempty"`
	Store                 *doctorStoreStats `json:"store,omitempty"`
	StoreError            string            `json:"store_error,omitempty"`
}

func doctorStoreStatsFromStoreStats(stats store.StoreStats) doctorStoreStats {
	out := doctorStoreStats{
		Messages: stats.Messages,
		Chats:    stats.Chats,
		Contacts: stats.Contacts,
		Groups:   stats.Groups,
	}

	return out
}

func doctorSchemaWarning(have, want int) string {
	if have >= want {
		return ""
	}
	return fmt.Sprintf("store schema v%d is older than the v%d this wacli-reader reads; some commands will fail with \"no such column\" until a current upstream wacli opens the store once and migrates it", have, want)
}

func writeDoctorReport(w io.Writer, rep doctorReport) {
	tw := newTableWriter(w)
	fmt.Fprintf(tw, "STORE\t%s\n", sanitize(rep.StoreDir))
	fmt.Fprintf(tw, "FTS5\t%v\n", rep.FTSEnabled)
	if rep.SchemaVersion > 0 {
		fmt.Fprintf(tw, "SCHEMA\tv%d (reader expects v%d)\n", rep.SchemaVersion, rep.ExpectedSchemaVersion)
	}
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
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			storeDir, err := resolveStoreDir(flags)
			if err != nil {
				return err
			}

			var storeErr string
			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				storeErr = err.Error()
			} else {
				defer closeApp(a, lk)
			}

			var stats *doctorStoreStats
			var ftsEnabled bool
			var schemaVersion int
			if a != nil {
				db := a.DB()
				if v, err := db.SchemaVersion(); err == nil {
					schemaVersion = v
				} else if storeErr == "" {
					storeErr = err.Error()
				}
				if raw, err := db.Stats(); err == nil {
					converted := doctorStoreStatsFromStoreStats(raw)
					stats = &converted
					if synced, err := db.LastSuccessfulSync(); err != nil {
						storeErr = err.Error()
					} else if synced != nil {
						stats.LastSyncAt = synced.UTC().Format(time.RFC3339)
					}
				} else if storeErr == "" {
					// A readable but corrupt store must not report a clean bill
					// of health just because it opened.
					storeErr = err.Error()
				}
				ftsEnabled = db.HasFTS()
			}

			rep := doctorReport{
				StoreDir:              storeDir,
				FTSEnabled:            ftsEnabled,
				SchemaVersion:         schemaVersion,
				ExpectedSchemaVersion: store.LatestSchemaVersion(),
				Store:                 stats,
				StoreError:            storeErr,
			}
			if a != nil {
				rep.SchemaWarning = doctorSchemaWarning(schemaVersion, rep.ExpectedSchemaVersion)
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, rep)
			}

			writeDoctorReport(os.Stdout, rep)

			if rep.SchemaWarning != "" {
				fmt.Fprintf(os.Stdout, "\nWARNING: %s.\n", rep.SchemaWarning)
			}
			if rep.StoreError != "" {
				fmt.Fprintf(os.Stdout, "\nERROR: store could not be opened: %s\n", rep.StoreError)
				fmt.Fprintln(os.Stdout, "Tip: wacli-reader needs a wacli.db that upstream wacli has already synced. See https://github.com/openclaw/wacli.")
			}
			return nil
		},
	}

	return cmd
}
