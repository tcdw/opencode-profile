// Package launch turns a profile choice into an executable Plan. It never execs
// anything itself — main() does, so the irreversible syscall lives in one place
// and the launch path stays testable.
package launch

import (
	"os"

	"github.com/tcdw/opencode-profile/internal/env"
	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

// Plan is everything needed for syscall.Exec.
type Plan struct {
	Bin  string
	Argv []string
	Env  []string
}

// BuildPlan resolves the opencode binary and assembles the child environment.
// The reserved "default" profile (or an empty name) runs against the live dirs
// with no XDG override.
func BuildPlan(l paths.Layout, name string, extraArgs []string) (*Plan, error) {
	bin, err := paths.FindOpencode()
	if err != nil {
		return nil, err
	}
	var environ []string
	if name == "" || name == store.ReservedDefault {
		environ = os.Environ()
	} else {
		environ = env.BuildEnv(l, name)
	}
	// argv[0] is the conventional command name so opencode's own usage prints
	// correctly; extra args are passed through verbatim.
	argv := append([]string{"opencode"}, extraArgs...)
	return &Plan{Bin: bin, Argv: argv, Env: environ}, nil
}
