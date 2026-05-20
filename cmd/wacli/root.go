package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/openclaw/wacli/internal/app"
	"github.com/openclaw/wacli/internal/config"
	"github.com/openclaw/wacli/internal/out"
	"github.com/spf13/cobra"
)

var version = "0.0.2"

type rootFlags struct {
	storeDir   string
	asJSON     bool
	noJSON     bool
	fullOutput bool
	timeout    time.Duration
}

func execute(args []string) error {
	var flags rootFlags

	rootCmd := &cobra.Command{
		Use:           "wacli-reader",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("wacli-reader {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flags.storeDir, "store", "", "store directory (default: $WACLI_STORE_DIR, XDG state dir on Linux, or ~/.wacli)")
	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "output JSON (default: auto — JSON when stdout is not a TTY)")
	rootCmd.PersistentFlags().BoolVar(&flags.noJSON, "no-json", false, "force human-readable table output even when stdout is not a TTY")
	rootCmd.PersistentFlags().BoolVar(&flags.fullOutput, "full", false, "disable truncation in table output")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 5*time.Minute, "command timeout")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		switch {
		case cmd.Flags().Changed("no-json"):
			flags.asJSON = false
		case cmd.Flags().Changed("json"):
			// honour explicit value already in flags.asJSON
		default:
			flags.asJSON = !isTTY()
		}
	}

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newDoctorCmd(&flags))
	rootCmd.AddCommand(newMessagesCmd(&flags))
	rootCmd.AddCommand(newContactsCmd(&flags))
	rootCmd.AddCommand(newChatsCmd(&flags))
	rootCmd.AddCommand(newGroupsCmd(&flags))
	rootCmd.AddCommand(newCallsCmd(&flags))
	rootCmd.AddCommand(newPollsCmd(&flags))
	rootCmd.AddCommand(newPollCmd(&flags))
	rootCmd.AddCommand(newStoreCmd(&flags))

	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		_ = out.WriteError(os.Stderr, flags.asJSON, err)
		return err
	}
	return nil
}

func newApp(flags *rootFlags) (*app.App, error) {
	storeDir := flags.storeDir
	if storeDir == "" {
		storeDir = config.DefaultStoreDir()
	}
	storeDir, _ = filepath.Abs(storeDir)

	a, err := app.New(app.Options{
		StoreDir: storeDir,
		Version:  version,
		JSON:     flags.asJSON,
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func withTimeout(ctx context.Context, flags *rootFlags) (context.Context, context.CancelFunc) {
	if flags.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, flags.timeout)
}
