package main

import (
	"context"
	"fmt"
	"os"

	"github.com/openclaw/wacli/internal/out"
	"github.com/spf13/cobra"
)

func newStoreStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show store row counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

			chats, err := a.DB().CountChats()
			if err != nil {
				return err
			}
			groups, err := a.DB().CountGroups()
			if err != nil {
				return err
			}
			leftGroups, err := a.DB().CountLeftGroups()
			if err != nil {
				return err
			}
			messages, err := a.DB().CountMessages()
			if err != nil {
				return err
			}

			stats := map[string]any{
				"chats":       chats,
				"groups":      groups,
				"left_groups": leftGroups,
				"messages":    messages,
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, stats)
			}

			fmt.Fprintf(os.Stdout, "Store Statistics:\n")
			fmt.Fprintf(os.Stdout, "  Chats:       %d\n", chats)
			fmt.Fprintf(os.Stdout, "  Groups:      %d\n", groups)
			fmt.Fprintf(os.Stdout, "  Left Groups: %d\n", leftGroups)
			fmt.Fprintf(os.Stdout, "  Messages:    %d\n", messages)
			return nil
		},
	}
	return cmd
}
