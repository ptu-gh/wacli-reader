package app

import (
	"fmt"
	"path/filepath"

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
