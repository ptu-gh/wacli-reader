package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openclaw/wacli/internal/out"
	"github.com/spf13/cobra"
)

func newContactsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "Search synced contact metadata",
	}
	cmd.AddCommand(newContactsSearchCmd(flags))
	cmd.AddCommand(newContactsShowCmd(flags))
	return cmd
}

func newContactsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search contacts (from synced metadata)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

			cs, err := a.DB().SearchContacts(args[0], limit)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, cs)
			}

			fullOutput := fullTableOutput(flags.fullOutput)
			w := newTableWriter(os.Stdout)
			fmt.Fprintln(w, "ALIAS\tNAME\tPHONE\tJID")
			for _, c := range cs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					tableCell(c.Alias, 18, fullOutput),
					tableCell(c.Name, 24, fullOutput),
					tableCell(c.Phone, 14, fullOutput),
					c.JID,
				)
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "limit results")
	return cmd
}

func newContactsShowCmd(flags *rootFlags) *cobra.Command {
	var jid string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one contact",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jid) == "" {
				return fmt.Errorf("--jid is required")
			}
			_, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, err := newApp(flags)
			if err != nil {
				return err
			}
			defer a.Close()

			c, err := a.DB().GetContact(jid)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, c)
			}

			fmt.Fprintf(os.Stdout, "JID: %s\n", c.JID)
			if c.Phone != "" {
				fmt.Fprintf(os.Stdout, "Phone: %s\n", c.Phone)
			}
			if c.Name != "" {
				fmt.Fprintf(os.Stdout, "Name: %s\n", c.Name)
			}
			if c.Alias != "" {
				fmt.Fprintf(os.Stdout, "Alias: %s\n", c.Alias)
			}
			if len(c.Tags) > 0 {
				fmt.Fprintf(os.Stdout, "Tags: %s\n", strings.Join(c.Tags, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&jid, "jid", "", "contact JID")
	return cmd
}
