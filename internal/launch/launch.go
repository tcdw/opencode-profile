// Package launch turns a profile choice into an executable Plan. It never execs
// anything itself — main() does, so the irreversible syscall lives in one place
// and the launch path stays testable.
package launch

import (
	"os"
	"path/filepath"

	"github.com/tcdw/opencode-profile/internal/env"
	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

// Plan is everything needed to run opencode under a profile.
type Plan struct {
	Bin   string
	Argv  []string
	Env   []string
	Syncs []SyncPair
}

// SyncPair describes a post-exit JSON merge from the XDG default location
// (Src) into the profile's auth file (Dst). opencode writes credentials to
// the XDG data dir regardless of OPENCODE_CONFIG_DIR, so we reclaim them
// after the child exits.
type SyncPair struct {
	Src string
	Dst string
}

// BuildPlan resolves the opencode binary and assembles the child environment.
// The reserved "default" profile (or an empty name) runs against the live dirs
// with no override.
func BuildPlan(l paths.Layout, name string, extraArgs []string) (*Plan, error) {
	bin, err := paths.FindOpencode()
	if err != nil {
		return nil, err
	}
	var environ []string
	var syncs []SyncPair
	if name == "" || name == store.ReservedDefault {
		environ = os.Environ()
	} else {
		environ = env.BuildEnv(l, name)
		syncs = buildSyncs(l, name)
	}
	// argv[0] is the conventional command name so opencode's own usage prints
	// correctly; extra args are passed through verbatim.
	argv := append([]string{"opencode"}, extraArgs...)
	return &Plan{Bin: bin, Argv: argv, Env: environ, Syncs: syncs}, nil
}

// buildSyncs returns the auth/mcp-auth sync pairs for a named profile.
// Src is the XDG default location where opencode actually writes credentials;
// Dst is the profile's auth file (resolved through symlinks so linked mode
// writes back to the shared base).
func buildSyncs(l paths.Layout, name string) []SyncPair {
	var pairs []SyncPair
	for _, sp := range []struct{ src, dst string }{
		{l.LiveAuth(), l.ProfileAuth(name)},
		{l.LiveMCPAuth(), l.ProfileMCPAuth(name)},
	} {
		dst := sp.dst
		if resolved, err := filepath.EvalSymlinks(sp.dst); err == nil {
			dst = resolved
		}
		pairs = append(pairs, SyncPair{Src: sp.src, Dst: dst})
	}
	return pairs
}
