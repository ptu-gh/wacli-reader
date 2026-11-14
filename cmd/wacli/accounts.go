package main

// Read-only slice of upstream's cmd/wacli/accounts.go. Upstream's file also
// defines `accounts add` (which runs a full auth flow), `accounts use`, and
// `accounts remove` — all of which write config.yaml and are deliberately not
// shipped here. Everything below is copied verbatim so a future rebase diffs
// cleanly. These commands read config.yaml only; they never open a store.

import (
	"fmt"
	"os"
	"sort"

	"github.com/openclaw/wacli/internal/config"
	"github.com/openclaw/wacli/internal/out"
	"github.com/spf13/cobra"
)

type accountPayload struct {
	Name            string `json:"name"`
	Label           string `json:"label,omitempty"`
	ConfiguredStore string `json:"configured_store"`
	StoreDir        string `json:"store_dir"`
	Default         bool   `json:"default"`
}

func newAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Inspect named WhatsApp accounts",
	}
	cmd.AddCommand(newAccountsListCmd(flags))
	cmd.AddCommand(newAccountsShowCmd(flags))
	return cmd
}

func newAccountsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.DefaultConfigPath()
			cfg, _, err := config.LoadAccountsConfigIfExists(path)
			if err != nil {
				return err
			}
			accounts := sortedAccounts(path, cfg)
			payloads := accountPayloads(accounts)
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"config_path":     path,
					"default_account": cfg.DefaultAccount,
					"accounts":        payloads,
				})
			}
			if len(accounts) == 0 {
				fmt.Fprintln(os.Stdout, "No accounts configured. Upstream wacli creates them with `wacli accounts add`.")
				return nil
			}
			w := newTableWriter(os.Stdout)
			fmt.Fprintln(w, "DEFAULT\tNAME\tSTORE")
			for _, account := range accounts {
				mark := ""
				if account.Default {
					mark = "*"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", mark, sanitize(account.Name), sanitize(account.StoreDir))
			}
			_ = w.Flush()
			return nil
		},
	}
}

func newAccountsShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show one configured account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, account, err := config.ResolveAccountStore(config.DefaultConfigPath(), args[0])
			if err != nil {
				return err
			}
			payload := accountPayloadFromAccount(account)
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, payload)
			}
			fmt.Fprintf(os.Stdout, "Name: %s\nStore: %s\nDefault: %t\n", sanitize(payload.Name), sanitize(payload.StoreDir), payload.Default)
			if payload.Label != "" {
				fmt.Fprintf(os.Stdout, "Label: %s\n", sanitize(payload.Label))
			}
			return nil
		},
	}
}

func sortedAccounts(path string, cfg *config.AccountsConfig) []config.Account {
	accounts := config.ListAccounts(path, cfg)
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Name < accounts[j].Name
	})
	return accounts
}

func accountPayloads(accounts []config.Account) []accountPayload {
	payloads := make([]accountPayload, 0, len(accounts))
	for _, account := range accounts {
		payloads = append(payloads, accountPayloadFromAccount(account))
	}
	return payloads
}

func accountPayloadFromAccount(account config.Account) accountPayload {
	return accountPayload{
		Name:            account.Name,
		Label:           account.Label,
		ConfiguredStore: account.ConfiguredStore,
		StoreDir:        account.StoreDir,
		Default:         account.Default,
	}
}
