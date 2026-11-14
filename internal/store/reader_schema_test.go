package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSchemaVersionOfMigratedStoreIsLatest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wacli.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	v, err := ro.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v == 0 || v != LatestSchemaVersion() {
		t.Fatalf("SchemaVersion = %d, LatestSchemaVersion = %d", v, LatestSchemaVersion())
	}
}

func TestSchemaVersionOfUnmigratedStoreIsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wacli.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE unrelated (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if v, err := ro.SchemaVersion(); err != nil || v != 0 {
		t.Fatalf("SchemaVersion = %d, %v; want 0", v, err)
	}
}
