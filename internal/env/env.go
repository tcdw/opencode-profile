// Package env builds the child environment that makes profile isolation work.
package env

import (
	"os"
	"runtime"
	"strings"

	"github.com/tcdw/opencode-profile/internal/paths"
)

// BuildEnv returns os.Environ() with XDG and opencode-specific paths overridden
// to point at the named profile. The explicit OPENCODE_* values make config and
// database selection robust across platforms and opencode runtime changes.
func BuildEnv(l paths.Layout, name string) []string {
	overrides := map[string]string{
		"XDG_CONFIG_HOME":     l.ProfileConfig(name),
		"XDG_DATA_HOME":       l.ProfileData(name),
		"XDG_STATE_HOME":      l.ProfileState(name),
		"XDG_CACHE_HOME":      l.ProfileCache(name),
		"OPENCODE_CONFIG_DIR": l.ProfileConfigOpencode(name),
		"OPENCODE_CONFIG":     l.OpencodeConfig(name),
		"OPENCODE_DB":         l.ProfileDB(name),
	}
	return mergeEnv(os.Environ(), overrides)
}

// mergeEnv strips any pre-existing entries for the override keys so a parent
// shell cannot leak stale profile paths through, then appends the new values.
func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	shadowedKeys := make(map[string]struct{}, len(overrides))
	for k := range overrides {
		shadowedKeys[envKey(k)] = struct{}{}
	}
	for _, kv := range base {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if _, shadowed := shadowedKeys[envKey(k)]; shadowed {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func envKey(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}
