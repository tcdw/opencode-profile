package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tcdw/opencode-profile/internal/paths"
)

// setup builds a fully hermetic store: live opencode dirs are redirected to the
// temp dir via XDG so Init seeds from controlled fixtures, never real config.
func setup(t *testing.T) (paths.Layout, *Store) {
	t.Helper()
	tmp := t.TempDir()
	liveCfg := filepath.Join(tmp, "live-config")
	liveData := filepath.Join(tmp, "live-data")
	t.Setenv("XDG_CONFIG_HOME", liveCfg)
	t.Setenv("XDG_DATA_HOME", liveData)
	mustMkdir(t, filepath.Join(liveCfg, "opencode"))
	mustMkdir(t, filepath.Join(liveData, "opencode"))
	mustWrite(t, filepath.Join(liveCfg, "opencode", "opencode.json"), `{"model":"live/model","mcp":{}}`)
	mustWrite(t, filepath.Join(liveData, "opencode", "auth.json"), `{"openai":{"type":"oauth"}}`)

	l := paths.Layout{Root: filepath.Join(tmp, "store")}
	s, err := Open(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return l, s
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateLinksAndReconcile(t *testing.T) {
	l, s := setup(t)
	p, err := s.Create("work", CreateOpts{Description: "w"})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(l.ProfileAuth("work"))
	if err != nil {
		t.Fatal(err)
	}
	switch p.Modes[DomainAuth] {
	case ModeLinked:
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Error("linked auth.json should be a symlink")
		}
	case ModeOwned:
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Error("owned auth.json should be a real file")
		}
	default:
		t.Fatalf("new profile auth mode = %s, want linked or owned fallback", p.Modes[DomainAuth])
	}
	// reopening must reconcile to the same on-disk truth
	s2, _ := Open(l)
	pp, err := s2.Get("work")
	if err != nil {
		t.Fatal(err)
	}
	if pp.Modes[DomainAuth] != p.Modes[DomainAuth] {
		t.Errorf("reconcile auth mode = %s, want %s", pp.Modes[DomainAuth], p.Modes[DomainAuth])
	}
}

func TestSetModeRoundTrip(t *testing.T) {
	l, s := setup(t)
	if _, err := s.Create("work", CreateOpts{}); err != nil {
		t.Fatal(err)
	}

	if err := s.SetMode("work", DomainAuth, ModeOwned); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Lstat(l.ProfileAuth("work"))
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("owned auth.json should be a real file, not a symlink")
	}
	if _, err := os.Stat(l.SharedAuth()); err != nil {
		t.Error("shared auth must survive linked->owned")
	}

	if err := s.SetMode("work", DomainAuth, ModeLinked); err != nil {
		t.Fatal(err)
	}
	fi, _ = os.Lstat(l.ProfileAuth("work"))
	p, err := s.Get("work")
	if err != nil {
		t.Fatal(err)
	}
	switch p.Modes[DomainAuth] {
	case ModeLinked:
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Error("re-linked auth.json should be a symlink again")
		}
	case ModeOwned:
		if runtime.GOOS != "windows" {
			t.Error("linked auth should only degrade to owned on symlink-hostile platforms")
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			t.Error("fallback auth.json should be a real file")
		}
	default:
		t.Fatalf("auth mode after relink = %s, want linked or owned fallback", p.Modes[DomainAuth])
	}
	if m, _ := filepath.Glob(l.ProfileAuth("work") + ".bak-*"); len(m) == 0 {
		t.Error("owned->linked should back up the previous copy, not delete it")
	}
}

func TestRemoveKeepsShared(t *testing.T) {
	l, s := setup(t)
	if _, err := s.Create("work", CreateOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(l.SharedAuth()); err != nil {
		t.Error("Remove must not follow the symlink into shared")
	}
	if _, err := os.Stat(l.ProfileDir("work")); !os.IsNotExist(err) {
		t.Error("profile dir should be gone after Remove")
	}
}

func TestGlobalLinksCreatedAtInit(t *testing.T) {
	l, _ := setup(t)
	if runtime.GOOS == "windows" {
		t.Skip("symlink assumptions skipped on windows")
	}
	for _, target := range []string{l.GlobalConfigDir(), l.GlobalDataDir()} {
		fi, err := os.Lstat(target)
		if err != nil {
			t.Fatalf("global link %s missing: %v", target, err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink", target)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, n := range []string{"default", "..", ".", "a/b", "", ".hidden", "-x", "a b"} {
		if err := ValidateName(n); err == nil {
			t.Errorf("expected %q to be rejected", n)
		}
	}
	for _, n := range []string{"work", "claude-1", "a.b_c", "GLM"} {
		if err := ValidateName(n); err != nil {
			t.Errorf("expected %q to be valid: %v", n, err)
		}
	}
}
