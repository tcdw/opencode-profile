// Package cli implements the non-TUI subcommands. Each returns either an error
// (for side-effecting commands) or a *launch.Plan (for `run`) that main execs.
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/tcdw/opencode-profile/internal/launch"
	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

const Version = "0.1.0"

func Init(l paths.Layout) error {
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	fmt.Printf("initialized ocp store at %s\n", l.Root)
	return nil
}

func List(l paths.Layout) error {
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODEL\tDOMAINS\tDESCRIPTION")
	fmt.Fprintln(w, "default\t(live config)\t-\tbuilt-in: current ~/.config/opencode")
	for _, p := range s.Profiles {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, store.ReadModel(l, p.Name), p.DomainBadges(), p.Description)
	}
	return w.Flush()
}

func Create(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	desc := fs.String("desc", "", "profile description")
	blank := fs.Bool("blank", false, "seed a minimal config instead of copying the current one")
	// Accept the name either before or after flags: stdlib flag stops parsing at
	// the first positional, so pull a leading bare token out as the name first.
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return fmt.Errorf("usage: ocp create <name> [-desc ...] [-blank]")
	}
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if !s.IsInitialized() {
		if err := s.Init(); err != nil {
			return err
		}
	}
	p, err := s.Create(name, store.CreateOpts{Description: *desc, Blank: *blank})
	if err != nil {
		return err
	}
	fmt.Printf("created profile %q at %s\n", p.Name, l.ProfileDir(p.Name))
	return nil
}

func Remove(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ocp rm <name>")
	}
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if err := s.Remove(args[0]); err != nil {
		return err
	}
	fmt.Printf("removed profile %q\n", args[0])
	return nil
}

// Run resolves a profile into a launch Plan for main to exec. Args are
// "<name> [-- opencode args]"; tokens after the name (optionally past a literal
// --) are passed through to opencode.
func Run(l paths.Layout, args []string) (*launch.Plan, error) {
	name, extra := splitArgs(args)
	if name == "" {
		return nil, fmt.Errorf("usage: ocp run <name> [-- opencode args]")
	}
	if name != store.ReservedDefault {
		s, err := store.Open(l)
		if err != nil {
			return nil, err
		}
		if _, err := s.Get(name); err != nil {
			return nil, err
		}
	}
	return launch.BuildPlan(l, name, extra)
}

// Path prints shell export lines so a user can `eval "$(ocp path x)"` and then
// run opencode manually.
func Path(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ocp path <name>")
	}
	name := args[0]
	if name == store.ReservedDefault {
		fmt.Println("# default profile uses live dirs; no XDG overrides needed")
		return nil
	}
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if _, err := s.Get(name); err != nil {
		return err
	}
	fmt.Printf("export XDG_CONFIG_HOME=%q\n", l.ProfileConfig(name))
	fmt.Printf("export XDG_DATA_HOME=%q\n", l.ProfileData(name))
	fmt.Printf("export XDG_STATE_HOME=%q\n", l.ProfileState(name))
	fmt.Printf("export XDG_CACHE_HOME=%q\n", l.ProfileCache(name))
	return nil
}

func PrintVersion() { fmt.Println("opencode-profile " + Version) }

func Usage() {
	fmt.Print(`opencode-profile (ocp) — isolated profiles for opencode

USAGE:
  ocp                      launch the TUI profile picker
  ocp run <name> [-- ...]  launch opencode under a profile (extra args pass through)
  ocp list | ls            list profiles
  ocp create <name>        create a profile (-desc, -blank)
  ocp rm <name>            delete a profile
  ocp path <name>          print export lines for the profile's XDG dirs
  ocp init                 initialize the store and seed the shared base
  ocp -v | --version       print version
  ocp -h | --help          show this help

The built-in "default" profile runs opencode against your live config.
`)
}

// --- helpers ---

func splitArgs(args []string) (name string, extra []string) {
	if len(args) == 0 {
		return "", nil
	}
	name = args[0]
	extra = args[1:]
	if len(extra) > 0 && extra[0] == "--" {
		extra = extra[1:]
	}
	return name, extra
}
