package main

import "github.com/spf13/cobra"

func newStoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Inspect the local data store",
	}
	cmd.AddCommand(newStoreStatsCmd(flags))
	return cmd
}
