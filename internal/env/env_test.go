package env

import (
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

func TestBuildEnvOverridesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/old/config")
	t.Setenv("OCP_SENTINEL", "keep-me")
	l := paths.Layout{Root: "/root"}
	got := toMap(BuildEnv(l, "work"))

	if got["XDG_CONFIG_HOME"] != l.ProfileConfig("work") {
		t.Errorf("XDG_CONFIG_HOME = %q, want %q", got["XDG_CONFIG_HOME"], l.ProfileConfig("work"))
	}
	if got["XDG_DATA_HOME"] != l.ProfileData("work") {
		t.Errorf("XDG_DATA_HOME = %q, want %q", got["XDG_DATA_HOME"], l.ProfileData("work"))
	}
	if got["XDG_STATE_HOME"] != l.ProfileState("work") {
		t.Errorf("XDG_STATE_HOME = %q", got["XDG_STATE_HOME"])
	}
	if got["XDG_CACHE_HOME"] != l.ProfileCache("work") {
		t.Errorf("XDG_CACHE_HOME = %q", got["XDG_CACHE_HOME"])
	}
	if got["OCP_SENTINEL"] != "keep-me" {
		t.Error("non-XDG env not preserved")
	}
}

func TestBuildEnvNoDuplicateXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/old/config")
	count := 0
	for _, kv := range BuildEnv(paths.Layout{Root: "/root"}, "x") {
		if strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("XDG_CONFIG_HOME appears %d times, want 1 (pre-existing value must be stripped)", count)
	}
}
