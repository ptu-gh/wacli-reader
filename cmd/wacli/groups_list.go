package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
)

func newGroupsListCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List known groups (from local DB; populated by upstream wacli sync)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

			gs, err := a.DB().ListGroups(query, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, gs)
			}

			fullOutput := fullTableOutput(flags.fullOutput)
			w := newTableWriter(os.Stdout)
			fmt.Fprintln(w, "NAME\tJID\tCREATED")
			for _, g := range gs {
				name := g.Name
				if name == "" {
					name = g.JID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", tableCell(name, 40, fullOutput), g.JID, g.CreatedAt.Local().Format("2006-01-02"))
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search query")
	cmd.Flags().IntVar(&limit, "limit", 50, "limit")
	return cmd
}
