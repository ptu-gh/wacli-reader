package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/steipete/wacli/internal/store"
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
	if got.LastSyncAt != "2024-04-01T10:30:00Z" {
		t.Fatalf("LastSyncAt = %q", got.LastSyncAt)
	}
}

func TestWriteDoctorReportIncludesStats(t *testing.T) {
	var b bytes.Buffer
	writeDoctorReport(&b, doctorReport{
		StoreDir:   "/tmp/wacli",
		FTSEnabled: true,
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
