package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/wacli/internal/app"
	"github.com/openclaw/wacli/internal/config"
	"github.com/openclaw/wacli/internal/out"
	"github.com/spf13/cobra"
)

var version = "0.0.4"

// docsURL points at the fork, not upstream's https://wacli.sh — the read-only
// command surface and the read-only guarantee are documented here.
const docsURL = "https://github.com/ptu-gh/wacli-reader"

type rootFlags struct {
	storeDir   string
	account    string
	asJSON     bool
	noJSON     bool
	fullOutput bool
	timeout    time.Duration
	// readOnly is upstream's --read-only. The reader is always read-only, so
	// the flag defaults to true and exists only so that command lines written
	// for upstream (`wacli --read-only ...`) keep working; --read-only=false
	// is rejected rather than silently ignored.
	readOnly bool
}

func execute(args []string) error {
	var flags rootFlags
	rootCmd := newRootCmd(&flags)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		err = explainSchemaError(err)
		_ = out.WriteError(os.Stderr, flags.asJSON, err)
		return err
	}
	return nil
}

// explainSchemaError adds the likely cause to SQLite's "no such column/table":
// the reader never migrates, so a store last written by an older wacli lacks
// what this build's queries select.
func explainSchemaError(err error) error {
	msg := err.Error()
	if !strings.Contains(msg, "no such column") && !strings.Contains(msg, "no such table") {
		return err
	}
	return fmt.Errorf("%w (the store's schema is likely older than this wacli-reader expects; `wacli-reader doctor` shows both versions — let a current upstream wacli open the store once to migrate it)", err)
}

// newRootCmd builds the command tree. It is split out of execute (upstream
// builds the tree inline) so that tests can walk the tree and pin the read-only
// command surface; see TestCommandSurfaceIsReadOnly.
func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "wacli-reader",
		Short:         "Read-only reader for wacli's local WhatsApp archive",
		Long:          "wacli-reader reads the wacli.db archive that wacli writes. It never connects to WhatsApp and never writes the store.\n\nDocs: " + docsURL,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("wacli-reader {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flags.storeDir, "store", "", "store directory (default: $WACLI_STORE_DIR, XDG state dir on Linux, or ~/.wacli)")
	rootCmd.PersistentFlags().StringVar(&flags.account, "account", "", "named account from config.yaml")
	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "output JSON (default: auto — JSON when stdout is not a TTY)")
	rootCmd.PersistentFlags().BoolVar(&flags.noJSON, "no-json", false, "force human-readable table output even when stdout is not a TTY")
	rootCmd.PersistentFlags().BoolVar(&flags.fullOutput, "full", false, "disable truncation in table output")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 5*time.Minute, "command timeout")
	rootCmd.PersistentFlags().BoolVar(&flags.readOnly, "read-only", true, "always on: wacli-reader cannot write (accepted for compatibility with wacli)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		switch {
		case cmd.Flags().Changed("no-json"):
			flags.asJSON = false
		case cmd.Flags().Changed("json"):
			// honour explicit value already in flags.asJSON
		default:
			flags.asJSON = !isTTY()
		}
		if !flags.readOnly {
			return fmt.Errorf("--read-only=false is not supported: wacli-reader is always read-only")
		}
		return nil
	}

	// Registration order mirrors upstream's, minus the write commands, so that
	// commands upstream adds later land in an unambiguous place on rebase.
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAccountsCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newMessagesCmd(flags))
	rootCmd.AddCommand(newCallsCmd(flags))
	rootCmd.AddCommand(newPollCmd(flags))
	rootCmd.AddCommand(newPollsCmd(flags))
	rootCmd.AddCommand(newContactsCmd(flags))
	rootCmd.AddCommand(newChatsCmd(flags))
	rootCmd.AddCommand(newGroupsCmd(flags))
	rootCmd.AddCommand(newHistoryCmd(flags))
	rootCmd.AddCommand(newDocsCmd(flags))
	rootCmd.AddCommand(newStoreCmd(flags))
	return rootCmd
}

// readerLock stands in for upstream's *lock.Lock. wacli-reader never acquires
// the writer's LOCK file, so the value is always nil. It exists purely so that
// newApp and closeApp keep upstream's arity: that lets every command's RunE
// prologue stay byte-identical to upstream, which is the bulk of the fork's
// rebase cost. Nothing ever constructs one.
type readerLock struct{}

// newApp mirrors upstream's signature. ctx, needLock and allowUnauthed are
// accepted and (almost entirely) ignored: the reader takes no lock, and it has
// no auth state for allowUnauthed to gate. See AGENTS.md "Known divergences".
func newApp(ctx context.Context, flags *rootFlags, needLock bool, allowUnauthed bool) (*app.App, *readerLock, error) {
	// Every read command passes needLock=false. A true here means a write
	// command was pulled in from upstream by mistake — fail loudly rather than
	// silently running it unlocked against a store a writer owns.
	if needLock {
		return nil, nil, fmt.Errorf("internal: wacli-reader does not acquire the store lock; this command does not belong in the read-only fork")
	}

	storeDir, err := resolveStoreDir(flags)
	if err != nil {
		return nil, nil, err
	}

	a, err := app.New(app.Options{
		StoreDir: storeDir,
		Version:  version,
		JSON:     flags.asJSON,
		ReadOnly: true,
	})
	if err != nil {
		return nil, nil, err
	}
	return a, nil, nil
}

// resolveStoreDir is upstream's, verbatim. It is what makes wacli-reader and
// wacli agree on which store to read on a multi-account install: without it the
// reader ignores config.yaml's default_account and silently reads ~/.wacli.
func resolveStoreDir(flags *rootFlags) (string, error) {
	storeDir := ""
	account := ""
	if flags != nil {
		storeDir = flags.storeDir
		account = strings.TrimSpace(flags.account)
	}
	if storeDir != "" && account != "" {
		return "", fmt.Errorf("--store and --account cannot be combined")
	}
	switch {
	case storeDir != "":
	case account != "":
		resolved, _, err := config.ResolveAccountStore(config.DefaultConfigPath(), account)
		if err != nil {
			return "", err
		}
		storeDir = resolved
	case os.Getenv(config.EnvStoreDir) != "":
		storeDir = config.DefaultStoreDir()
	default:
		cfg, found, err := config.LoadAccountsConfigIfExists(config.DefaultConfigPath())
		if err != nil {
			return "", err
		}
		if found && strings.TrimSpace(cfg.DefaultAccount) != "" {
			resolved, _, err := config.ResolveAccountStore(config.DefaultConfigPath(), cfg.DefaultAccount)
			if err != nil {
				return "", err
			}
			storeDir = resolved
		} else {
			storeDir = config.DefaultStoreDir()
		}
	}
	storeDir, _ = filepath.Abs(storeDir)
	return storeDir, nil
}

func withTimeout(ctx context.Context, flags *rootFlags) (context.Context, context.CancelFunc) {
	if flags.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, flags.timeout)
}

// closeApp mirrors upstream's signature; the lock argument is always nil here.
func closeApp(a *app.App, lk *readerLock) {
	if a != nil {
		a.Close()
	}
	_ = lk
}
