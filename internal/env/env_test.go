package env

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tcdw/opencode-profile/internal/paths"
)

func toMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func TestBuildEnvOverridesOpencode(t *testing.T) {
	t.Setenv("OCP_SENTINEL", "keep-me")
	l := paths.Layout{Root: "/root"}
	got := toMap(BuildEnv(l, "work"))

	if got["OPENCODE_CONFIG_DIR"] != l.ProfileConfigOpencode("work") {
		t.Errorf("OPENCODE_CONFIG_DIR = %q, want %q", got["OPENCODE_CONFIG_DIR"], l.ProfileConfigOpencode("work"))
	}
	if got["OPENCODE_CONFIG"] != l.OpencodeConfig("work") {
		t.Errorf("OPENCODE_CONFIG = %q, want %q", got["OPENCODE_CONFIG"], l.OpencodeConfig("work"))
	}
	if got["OPENCODE_DB"] != l.ProfileDB("work") {
		t.Errorf("OPENCODE_DB = %q, want %q", got["OPENCODE_DB"], l.ProfileDB("work"))
	}
	if got["OCP_SENTINEL"] != "keep-me" {
		t.Error("existing env not preserved")
	}
}

func TestBuildEnvPreservesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/original/config")
	t.Setenv("XDG_DATA_HOME", "/original/data")
	l := paths.Layout{Root: "/root"}
	got := toMap(BuildEnv(l, "work"))

	if got["XDG_CONFIG_HOME"] != "/original/config" {
		t.Errorf("XDG_CONFIG_HOME = %q, want /original/config (should not be overridden)", got["XDG_CONFIG_HOME"])
	}
	if got["XDG_DATA_HOME"] != "/original/data" {
		t.Errorf("XDG_DATA_HOME = %q, want /original/data (should not be overridden)", got["XDG_DATA_HOME"])
	}
}

func TestBuildEnvUsesExistingJSONCConfig(t *testing.T) {
	root := t.TempDir()
	l := paths.Layout{Root: root}
	cfg := l.OpencodeJSONC("work")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := toMap(BuildEnv(l, "work"))
	if got["OPENCODE_CONFIG"] != cfg {
		t.Errorf("OPENCODE_CONFIG = %q, want %q", got["OPENCODE_CONFIG"], cfg)
	}
}

func TestBuildEnvNoDuplicateOpencode(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG", "/old/config")
	count := 0
	for _, kv := range BuildEnv(paths.Layout{Root: "/root"}, "x") {
		if strings.HasPrefix(kv, "OPENCODE_CONFIG=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("OPENCODE_CONFIG appears %d times, want 1 (pre-existing value must be stripped)", count)
	}
}

func TestMergeEnvWindowsKeysAreCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows environment keys are case-insensitive")
	}
	got := mergeEnv(
		[]string{"Path=keep", "opencode_config=C:\\old\\opencode.json"},
		map[string]string{"OPENCODE_CONFIG": `C:\new\opencode.json`},
	)
	count := 0
	for _, kv := range got {
		if strings.EqualFold(strings.SplitN(kv, "=", 2)[0], "OPENCODE_CONFIG") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("OPENCODE_CONFIG appears %d times, want 1", count)
	}
}
