package main

import "github.com/spf13/cobra"

func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List known groups and their participants (read-only)",
	}
	cmd.AddCommand(newGroupsListCmd(flags))
	cmd.AddCommand(newGroupsParticipantsCmd(flags))
	return cmd
}
