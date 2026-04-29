package main

import "github.com/spf13/cobra"

func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List known groups (read-only)",
	}
	cmd.AddCommand(newGroupsListCmd(flags))
	return cmd
}
