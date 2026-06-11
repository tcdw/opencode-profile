// Command opencode-profile (ocp) manages isolated opencode profiles. It is the
// single place that performs the exit-and-exec handoff into opencode.
package main

import (
	"fmt"
	"os"

	"github.com/tcdw/opencode-profile/internal/cli"
	"github.com/tcdw/opencode-profile/internal/launch"
	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
	"github.com/tcdw/opencode-profile/internal/tui"
)

func main() {
	l, err := paths.Default()
	if err != nil {
		fatal(err)
	}

	plan, err := dispatch(os.Args[1:], l)
	if err != nil {
		fatal(err)
	}
	if plan == nil {
		return // a non-launch subcommand ran, or the user quit the picker
	}

	// The terminal is in a clean state here (Bubble Tea restores it before
	// Run() returns), so opencode initializes its own TUI from scratch. On unix
	// this replaces the ocp process; on Windows it runs opencode as a child and
	// exits with its status.
	if err := launch.Exec(plan); err != nil {
		fmt.Fprintf(os.Stderr, "ocp: exec %s: %v\n", plan.Bin, err)
		os.Exit(127)
	}
}

func dispatch(args []string, l paths.Layout) (*launch.Plan, error) {
	if len(args) == 0 {
		s, err := store.Open(l)
		if err != nil {
			return nil, err
		}
		if !s.IsInitialized() {
			if err := s.Init(); err != nil {
				return nil, err
			}
		}
		return tui.Run(l, s)
	}
	switch args[0] {
	case "init":
		return nil, cli.Init(l)
	case "list", "ls":
		return nil, cli.List(l)
	case "create", "new":
		return nil, cli.Create(l, args[1:])
	case "rm", "remove":
		return nil, cli.Remove(l, args[1:])
	case "export":
		return nil, cli.Export(l, args[1:])
	case "import":
		return nil, cli.Import(l, args[1:])
	case "path":
		return nil, cli.Path(l, args[1:])
	case "run":
		return cli.Run(l, args[1:])
	case "acp":
		return cli.ACP(l, args[1:])
	case "zed":
		return nil, cli.Zed(l, args[1:])
	case "-h", "--help", "help":
		cli.Usage()
		return nil, nil
	case "-v", "--version":
		cli.PrintVersion()
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown command %q (try `ocp help`)", args[0])
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ocp:", err)
	os.Exit(1)
}
