package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

func TestZedSettings(t *testing.T) {
	got := zedSettings("/usr/local/bin/ocp", []string{"default", "work"})
	if len(got.AgentServers) != 2 {
		t.Fatalf("got %d servers, want 2", len(got.AgentServers))
	}
	server, ok := got.AgentServers["OpenCode (work)"]
	if !ok {
		t.Fatalf("missing OpenCode (work) server: %#v", got.AgentServers)
	}
	if server.Type != "custom" {
		t.Errorf("type = %q, want custom", server.Type)
	}
	if server.Command != "/usr/local/bin/ocp" {
		t.Errorf("command = %q", server.Command)
	}
	if len(server.Args) != 2 || server.Args[0] != "acp" || server.Args[1] != "work" {
		t.Errorf("args = %#v, want [acp work]", server.Args)
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatal(err)
	}
}

func TestZedProfileNamesIncludesDefaultAndStoredProfiles(t *testing.T) {
	l, s := setupCLIStore(t)
	if _, err := s.Create("work", store.CreateOpts{Blank: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("personal", store.CreateOpts{Blank: true}); err != nil {
		t.Fatal(err)
	}

	got, err := zedProfileNames(l)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"default", "work", "personal"}
	if len(got) != len(want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %#v, want %#v", got, want)
		}
	}
}

func TestSplitArgsKeepsACPTail(t *testing.T) {
	name, extra := splitArgs([]string{"work", "--", "--foo"})
	if name != "work" {
		t.Fatalf("name = %q, want work", name)
	}
	if len(extra) != 1 || extra[0] != "--foo" {
		t.Fatalf("extra = %#v, want [--foo]", extra)
	}
}

func setupCLIStore(t *testing.T) (paths.Layout, *store.Store) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "live-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "live-data"))
	l := paths.Layout{Root: filepath.Join(tmp, "store")}
	s, err := store.Open(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return l, s
}
