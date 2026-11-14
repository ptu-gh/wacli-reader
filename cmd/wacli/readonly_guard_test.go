package main

// Fork-only tests: the guarantees that make wacli-reader what it is. Upstream
// has no equivalent file, so rebases never conflict here — but a rebase that
// pulls in a write command, a WhatsApp client, a session.db reference or a
// lock acquisition fails one of these tests instead of shipping silently.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openclaw/wacli/internal/store"
	"github.com/spf13/cobra"
)

// readOnlyCommands is the complete command surface. Adding a command means
// adding it here, which is the point: every new entry is a deliberate claim
// that the command only reads wacli.db.
var readOnlyCommands = []string{
	"accounts", "accounts list", "accounts show",
	"calls", "calls list",
	"chats", "chats list", "chats show",
	"contacts", "contacts search", "contacts show",
	"docs",
	"doctor",
	"groups", "groups list", "groups participants", "groups participants list",
	"history", "history coverage",
	"messages", "messages changes", "messages context", "messages export", "messages list",
	"messages search", "messages show", "messages starred",
	"poll", "poll show",
	"polls", "polls list",
	"store", "store stats",
	"version",
}

func TestCommandSurfaceIsReadOnly(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	var got []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			got = append(got, strings.TrimPrefix(sub.CommandPath(), root.Name()+" "))
			walk(sub)
		}
	}
	walk(root)
	slices.Sort(got)
	want := slices.Clone(readOnlyCommands)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("command surface changed.\n got: %q\nwant: %q\nA new command must be verified to only read wacli.db before it is added to readOnlyCommands.", got, want)
	}
}

// forbiddenImports are packages that would give the binary a way to talk to
// WhatsApp, open session.db, take the writer's lock or hand work to a running
// writer.
var forbiddenImports = []string{
	"go.mau.fi/",
	"github.com/openclaw/wacli/internal/wa",
	"github.com/openclaw/wacli/internal/lock",
	"github.com/openclaw/wacli/internal/delegate",
	"github.com/openclaw/wacli/internal/linkpreview",
	"github.com/openclaw/wacli/internal/syscontacts",
	"net/http",
	"os/exec",
}

func TestSourceHasNoWriteOrNetworkPaths(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		checked++
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbiddenImports {
				if p == bad || strings.HasPrefix(p, strings.TrimSuffix(bad, "/")+"/") {
					t.Errorf("%s imports %s", path, p)
				}
			}
		}
		// Comments may mention session.db; code may not.
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "session.db") {
				t.Errorf("%s: string literal references session.db: %s", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 20 {
		t.Fatalf("only %d source files checked; walk root is wrong", checked)
	}
}

func TestReadOnlyFlag(t *testing.T) {
	dir := seedReaderStore(t)
	isolateAccountConfigHome(t)

	stdout := captureRootStdout(t, func() {
		if err := execute([]string{"--store", dir, "--read-only", "--json", "store", "stats"}); err != nil {
			t.Errorf("--read-only rejected: %v", err)
		}
	})
	if !strings.Contains(stdout, `"messages"`) {
		t.Fatalf("store stats output = %q", stdout)
	}

	stderr := captureRootStderr(t, func() {
		if err := execute([]string{"--store", dir, "--read-only=false", "store", "stats"}); err == nil {
			t.Error("--read-only=false accepted")
		}
	})
	if !strings.Contains(stderr, "always read-only") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestNewAppRefusesLock(t *testing.T) {
	dir := seedReaderStore(t)
	if _, _, err := newApp(t.Context(), &rootFlags{storeDir: dir}, true, false); err == nil || !strings.Contains(err.Error(), "does not acquire the store lock") {
		t.Fatalf("newApp(needLock=true) error = %v", err)
	}
}

// Seeded identities, shared by the store fixture and the command sweep.
const (
	seedDM    = "15551234567@s.whatsapp.net"
	seedGroup = "120363000000000001@g.us"
	seedMsgID = "MSG-1"
	seedPoll  = "POLL-1"
)

// seedReaderStore writes a small store through upstream's writable store
// package — the same path upstream's writer takes — and closes it, so that the
// reader under test is the only party touching the file.
func seedReaderStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "wacli.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.UpsertContact(seedDM, "15551234567", "Alice", "Alice Example", "Alice", ""))
	must(db.UpsertChat(seedDM, "dm", "Alice", now))
	must(db.UpsertChat(seedGroup, "group", "Book Club", now))
	must(db.UpsertGroup(seedGroup, "Book Club", seedDM, now))
	must(db.ReplaceGroupParticipants(seedGroup, []store.GroupParticipant{{GroupJID: seedGroup, UserJID: seedDM, Role: "admin"}}))
	for i := range 5 {
		must(db.UpsertMessage(store.UpsertMessageParams{
			ChatJID:    seedDM,
			ChatName:   "Alice",
			MsgID:      fmt.Sprintf("MSG-%d", i+1),
			SenderJID:  seedDM,
			SenderName: "Alice",
			Timestamp:  now.Add(time.Duration(i) * time.Minute),
			Text:       fmt.Sprintf("hello reader %d", i+1),
		}))
	}
	must(db.SetStarred(store.SetStarredParams{ChatJID: seedDM, MsgID: seedMsgID, SenderJID: seedDM, Starred: true, StarredAt: now}))
	must(db.UpsertPoll(store.Poll{ChatJID: seedGroup, MsgID: seedPoll, SenderJID: seedDM, Question: "Next book?", Options: []string{"A", "B"}, SelectableCount: 1, CreatedAt: now}))
	must(db.UpsertCallEvent(store.UpsertCallEventParams{ChatJID: seedDM, SenderJID: seedDM, CallID: "CALL-1", EventType: "offer", Direction: "incoming", Media: "audio", Timestamp: now}))
	return dir
}

// readSweep runs every read command once. Each is expected to succeed.
func readSweep(dir string) [][]string {
	base := []string{"--store", dir, "--json"}
	cmds := [][]string{
		{"doctor"},
		{"store", "stats"},
		{"chats", "list"},
		{"chats", "show", "--jid", seedDM},
		{"contacts", "search", "Alice"},
		{"contacts", "show", "--jid", seedDM},
		{"groups", "list"},
		{"groups", "participants", "list", "--jid", seedGroup},
		{"messages", "list"},
		{"messages", "list", "--chat", "Alice"},
		{"messages", "search", "reader"},
		{"messages", "starred"},
		{"messages", "show", "--chat", seedDM, "--id", seedMsgID},
		{"messages", "context", "--chat", seedDM, "--id", "MSG-3"},
		{"messages", "export"},
		{"messages", "changes", "--checkpoint"},
		{"messages", "changes", "--since", "beginning"},
		{"calls", "list"},
		{"polls", "list"},
		{"poll", "show", "--chat", seedGroup, "--id", seedPoll},
		{"history", "coverage"},
		{"accounts", "list"},
	}
	for i := range cmds {
		cmds[i] = append(slices.Clone(base), cmds[i]...)
	}
	return cmds
}

func storeFingerprint(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %v %x\n", e.Name(), info.Mode(), sha256.Sum256(data))
	}
	return b.String()
}

func TestReadCommandsLeaveStoreUntouched(t *testing.T) {
	dir := seedReaderStore(t)
	isolateAccountConfigHome(t)
	before := storeFingerprint(t, dir)

	for _, args := range readSweep(dir) {
		var runErr error
		stdout := captureRootStdout(t, func() { runErr = execute(args) })
		if runErr != nil {
			t.Errorf("%v: %v", args[3:], runErr)
			continue
		}
		if !json.Valid([]byte(stdout)) {
			t.Errorf("%v: output is not JSON: %q", args[3:], stdout)
		}
	}

	// Byte-identical, and no -wal, -shm, -journal or session.db appeared.
	if after := storeFingerprint(t, dir); after != before {
		t.Fatalf("store directory changed.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestReaderProcessHelper is the child side of TestConcurrentReaderProcesses.
func TestReaderProcessHelper(t *testing.T) {
	raw := os.Getenv("WACLI_READER_TEST_ARGS")
	if raw == "" {
		t.Skip("subprocess helper")
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"wacli-reader"}, args...)
	main()
	os.Exit(0)
}

// TestConcurrentReaderProcesses runs several reader processes at once against
// a store whose writer is live and writing, as `wacli sync --follow` would be.
// Readers must neither block the writer nor fail while it writes.
func TestConcurrentReaderProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns processes")
	}
	dir := seedReaderStore(t)
	isolateAccountConfigHome(t)

	writer, err := store.Open(filepath.Join(dir, "wacli.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	stop := make(chan struct{})
	var writes int
	var writeErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if err := writer.UpsertMessage(store.UpsertMessageParams{
				ChatJID:   seedDM,
				MsgID:     fmt.Sprintf("LIVE-%d", i),
				SenderJID: seedDM,
				Timestamp: time.Now().UTC(),
				Text:      fmt.Sprintf("live write %d", i),
			}); err != nil {
				writeErr = err
				return
			}
			writes++
		}
	})

	commands := [][]string{
		{"--store", dir, "--json", "messages", "list", "--limit", "20"},
		{"--store", dir, "--json", "messages", "search", "live"},
		{"--store", dir, "--json", "chats", "list"},
		{"--store", dir, "--json", "messages", "changes", "--since", "beginning", "--limit", "1000"},
		{"--store", dir, "--json", "doctor"},
	}
	rounds := 4
	if n, err := strconv.Atoi(os.Getenv("WACLI_READER_STRESS_ROUNDS")); err == nil && n > 0 {
		rounds = n
	}
	var mu sync.Mutex
	var failures []string
	sawLiveWrite := false
	for range rounds {
		var readers sync.WaitGroup
		for _, args := range commands {
			readers.Go(func() {
				encoded, _ := json.Marshal(args)
				cmd := exec.Command(os.Args[0], "-test.run=^TestReaderProcessHelper$")
				cmd.Env = append(os.Environ(), "WACLI_READER_TEST_ARGS="+string(encoded))
				out, err := cmd.Output()
				if err == nil && !json.Valid(out) {
					err = fmt.Errorf("invalid JSON %q", out)
				}
				if err == nil && strings.Contains(string(out), "live write") {
					mu.Lock()
					sawLiveWrite = true
					mu.Unlock()
				}
				if err != nil {
					var stderr string
					if ee, ok := err.(*exec.ExitError); ok {
						stderr = string(ee.Stderr)
					}
					mu.Lock()
					failures = append(failures, fmt.Sprintf("%v: %v %s", args[3:], err, stderr))
					mu.Unlock()
				}
			})
		}
		readers.Wait()
	}
	close(stop)
	wg.Wait()

	if writeErr != nil {
		t.Fatalf("writer failed while readers ran: %v", writeErr)
	}
	if len(failures) > 0 {
		t.Fatalf("%d reader processes failed:\n%s", len(failures), strings.Join(failures, "\n"))
	}
	if writes == 0 {
		t.Fatal("writer made no progress while readers ran")
	}
	if !sawLiveWrite {
		t.Fatal("no reader process saw a row the live writer committed")
	}
	t.Logf("%d reader processes ran alongside %d writer commits", rounds*len(commands), writes)
}
