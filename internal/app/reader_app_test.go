package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/wacli/internal/store"
)

func TestNewRejectsWritableOpen(t *testing.T) {
	if _, err := New(Options{StoreDir: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("New(ReadOnly=false) error = %v", err)
	}
}

func TestNewDoesNotCreateAStore(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(Options{StoreDir: dir, ReadOnly: true}); err == nil {
		t.Fatal("New on an empty store dir succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("New created files in an empty store dir: %v", entries)
	}
}

func TestNewOpensStoreReadOnly(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "wacli.db"))
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	a, err := New(Options{StoreDir: dir, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.DB().UpsertChat("1@s.whatsapp.net", "dm", "x", time.Now()); err == nil || !strings.Contains(err.Error(), "readonly") {
		t.Fatalf("write through App's DB error = %v, want readonly rejection", err)
	}
}
