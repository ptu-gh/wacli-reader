package app

import (
	"context"
	"fmt"
	"path/filepath"

	types "github.com/openclaw/wacli/internal/jid"
	"github.com/openclaw/wacli/internal/store"
)

type Options struct {
	StoreDir string
	Version  string
	JSON     bool
	// ReadOnly mirrors upstream wacli's opt-in read-only flag for interface
	// compatibility. wacli-reader requires it to be true — the binary has no
	// write path and refuses to construct an App that asks for one. Defence
	// in depth so future upstream merges that surface ReadOnly=false fail
	// loudly instead of silently regressing the no-writes invariant.
	ReadOnly bool
}

type App struct {
	opts Options
	db   *store.DB
}

// LocalResolver is upstream's LID↔phone-number resolver interface. Upstream
// implements it on top of session.db, which wacli-reader never opens, so the
// fork declares the interface (read helpers taken from upstream accept it as
// a parameter) but has no implementation: a LocalResolver here is always nil,
// and upstream's own nil-resolver fallback — the raw JID — is what runs.
type LocalResolver interface {
	ResolveChatName(context.Context, types.JID, string) string
	ResolveLIDToPN(context.Context, types.JID) types.JID
	ResolvePNToLID(context.Context, types.JID) types.JID
}

func New(opts Options) (*App, error) {
	if opts.StoreDir == "" {
		return nil, fmt.Errorf("store dir is required")
	}
	if !opts.ReadOnly {
		return nil, fmt.Errorf("wacli-reader only supports read-only opens; set Options.ReadOnly=true")
	}

	indexPath := filepath.Join(opts.StoreDir, "wacli.db")
	db, err := store.OpenReadOnly(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wacli.db read-only — has upstream wacli synced this store yet? %w", err)
	}

	return &App{opts: opts, db: db}, nil
}

func (a *App) Close() {
	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) DB() *store.DB    { return a.db }
func (a *App) StoreDir() string { return a.opts.StoreDir }
func (a *App) Version() string  { return a.opts.Version }
func (a *App) ReadOnly() bool   { return a.opts.ReadOnly }
