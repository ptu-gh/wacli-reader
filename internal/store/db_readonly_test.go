package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpenReadOnlyRejectsWrites is a fork-specific defence-in-depth test for
// wacli-reader: it verifies that the production opener (OpenReadOnly) physically
// rejects writes at the SQLite-driver layer. If somebody later points App.New
// at the writable Open by accident, this test catches it.
func TestOpenReadOnlyRejectsWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wacli.db")

	// Create + migrate the DB through the writable upstream path so there is
	// something for OpenReadOnly to open.
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open (writable, for setup): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer r.Close()

	_, err = r.sql.Exec(`INSERT INTO chats(jid, kind) VALUES('test@s.whatsapp.net', 'dm')`)
	if err == nil {
		t.Fatal("INSERT into a read-only DB succeeded; expected SQLITE_READONLY")
	}
	if !strings.Contains(err.Error(), "readonly") {
		t.Fatalf("unexpected error from write attempt: %v", err)
	}
}

// TestOpenReadOnlyRejectsURIInjection mirrors Open's URI-injection guard.
func TestOpenReadOnlyRejectsURIInjection(t *testing.T) {
	for _, p := range []string{
		"/tmp/wacli.db?mode=rw",
		"/tmp/wacli.db#frag",
		"",
	} {
		if _, err := OpenReadOnly(p); err == nil {
			t.Errorf("OpenReadOnly(%q) = nil error, want error", p)
		}
	}
}

// TestOpenReadOnlyOnReadOnlyFilesystem verifies the headline guarantee of
// wacli-reader: opening wacli.db must succeed even when the process has only
// read permissions on the file AND on its containing directory. SQLite in WAL
// mode normally tries to create a `<db>-shm` lock file in that directory, which
// fails on a read-only filesystem with "unable to open database file" or
// "attempt to write a readonly database". OpenReadOnly must work around that.
func TestOpenReadOnlyOnReadOnlyFilesystem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wacli.db")

	// Populate the DB through the writable upstream path so OpenReadOnly has
	// real data to read. Open() also enables WAL journal mode, which is the
	// mode that triggers the bug — without WAL, SQLite would happily read the
	// file even with mode=ro on a read-only directory.
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.UpsertChat("test@s.whatsapp.net", "dm", "Test", time.Unix(0, 0)); err != nil {
		t.Fatalf("UpsertChat: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Strip write access from every file in the dir AND from the dir itself,
	// reproducing the user's "user only has read filesystem permissions" setup.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if err := os.Chmod(p, 0o444); err != nil {
			t.Fatalf("chmod file %s: %v", p, err)
		}
		// Restore for cleanup — t.TempDir's RemoveAll needs write access.
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	r, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly on read-only directory: %v", err)
	}
	defer r.Close()

	chats, err := r.ListChats("", 10)
	if err != nil {
		t.Fatalf("ListChats on read-only DB: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("ListChats returned %d rows, want 1", len(chats))
	}
}
