// Package env builds the child environment that makes profile isolation work.
package env

import (
	"os"
	"runtime"
	"strings"

	"github.com/tcdw/opencode-profile/internal/paths"
)

// BuildEnv returns os.Environ() with opencode-specific paths overridden to
// point at the named profile. XDG variables are intentionally left untouched so
// that tools launched from within opencode (glab, gh, etc.) can still find
// their own tokens and configuration in the standard XDG directories.
func BuildEnv(l paths.Layout, name string) []string {
	overrides := map[string]string{
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
