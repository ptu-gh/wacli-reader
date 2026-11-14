package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/wacli/internal/store"
)

func TestDoctorStoreStatsFromStoreStats(t *testing.T) {
	when := time.Date(2024, 4, 1, 12, 30, 0, 0, time.FixedZone("offset", 2*60*60))
	got := doctorStoreStatsFromStoreStats(store.StoreStats{
		Messages:      4,
		Chats:         3,
		Contacts:      2,
		Groups:        1,
		LastMessageTS: when.Unix(),
	})
	if got.Messages != 4 || got.Chats != 3 || got.Contacts != 2 || got.Groups != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	// Like upstream, the last sync time comes from the writer's recorded sync,
	// not from the newest message.
	if got.LastSyncAt != "" {
		t.Fatalf("LastSyncAt = %q", got.LastSyncAt)
	}
}

func TestWriteDoctorReportIncludesStats(t *testing.T) {
	var b bytes.Buffer
	writeDoctorReport(&b, doctorReport{
		StoreDir:              "/tmp/wacli",
		FTSEnabled:            true,
		SchemaVersion:         33,
		ExpectedSchemaVersion: 33,
		Store: &doctorStoreStats{
			Messages:   9,
			Chats:      8,
			Contacts:   7,
			Groups:     6,
			LastSyncAt: "2024-04-01T10:30:00Z",
		},
	})

	out := b.String()
	for _, want := range []string{
		"STORE",
		"/tmp/wacli",
		"FTS5",
		"true",
		"SCHEMA",
		"v33 (reader expects v33)",
		"MESSAGES",
		"9",
		"CHATS",
		"8",
		"CONTACTS",
		"7",
		"GROUPS",
		"6",
		"LAST_SYNC",
		"2024-04-01T10:30:00Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

// Upstream's test, verbatim.
func TestDoctorReportsCorruptStore(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "wacli.db"), []byte("not sqlite"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout := captureRootStdout(t, func() {
		if err := execute([]string{"--store", storeDir, "--read-only", "--json", "doctor"}); err != nil {
			t.Fatalf("execute doctor: %v", err)
		}
	})
	var got struct {
		Data doctorReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if got.Data.StoreError == "" {
		t.Fatalf("store_error is empty for corrupt store: %s", stdout)
	}
}

func runDoctorJSON(t *testing.T, storeDir string) doctorReport {
	t.Helper()
	isolateAccountConfigHome(t)
	stdout := captureRootStdout(t, func() {
		if err := execute([]string{"--store", storeDir, "--json", "doctor"}); err != nil {
			t.Fatalf("execute doctor: %v", err)
		}
	})
	var got struct {
		Data doctorReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	return got.Data
}

func TestDoctorCurrentSchemaHasNoWarning(t *testing.T) {
	rep := runDoctorJSON(t, seedReaderStore(t))
	if rep.SchemaVersion != store.LatestSchemaVersion() || rep.ExpectedSchemaVersion != store.LatestSchemaVersion() {
		t.Fatalf("schema = v%d, expected v%d, latest v%d", rep.SchemaVersion, rep.ExpectedSchemaVersion, store.LatestSchemaVersion())
	}
	if rep.SchemaWarning != "" || rep.StoreError != "" {
		t.Fatalf("unexpected warning/error: %q / %q", rep.SchemaWarning, rep.StoreError)
	}
}

func TestDoctorWarnsAboutOlderWriterSchema(t *testing.T) {
	dir := seedReaderStore(t)
	// Make the store look as if an older wacli last migrated it.
	raw, err := sql.Open("sqlite3", filepath.Join(dir, "wacli.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM schema_migrations WHERE version > 20`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	rep := runDoctorJSON(t, dir)
	if rep.SchemaVersion != 20 {
		t.Fatalf("schema_version = %d, want 20", rep.SchemaVersion)
	}
	if !strings.Contains(rep.SchemaWarning, "v20 is older than") {
		t.Fatalf("schema_warning = %q", rep.SchemaWarning)
	}
}
