// Package env builds the child environment that makes profile isolation work:
// the current environment with the four XDG_*_HOME vars pointed inside a profile.
package env

import (
	"os"
	"strings"

	"github.com/tcdw/opencode-profile/internal/paths"
)

// BuildEnv returns os.Environ() with XDG_{CONFIG,DATA,STATE,CACHE}_HOME
// overridden to point at the named profile. opencode resolves its config to
// <XDG_CONFIG_HOME>/opencode and its data (auth.json, db, ...) to
// <XDG_DATA_HOME>/opencode, giving full isolation without touching live dirs.
func BuildEnv(l paths.Layout, name string) []string {
	overrides := map[string]string{
		"XDG_CONFIG_HOME": l.ProfileConfig(name),
		"XDG_DATA_HOME":   l.ProfileData(name),
		"XDG_STATE_HOME":  l.ProfileState(name),
		"XDG_CACHE_HOME":  l.ProfileCache(name),
	}
	return mergeEnv(os.Environ(), overrides)
}

// mergeEnv strips any pre-existing entries for the override keys (so a parent
// shell that already set XDG_* can't leak through) and appends the new values.
func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if _, shadowed := overrides[k]; shadowed {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}
