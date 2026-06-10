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
	"github.com/tcdw/opencode-profile/internal/transfer"
	"golang.org/x/term"
)

var Version = "dev"

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
	blank := fs.Bool("blank", false, "seed a minimal config and empty AGENTS.md instead of copying the current config")
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
	if !*blank && nonEmptyFile(l.LiveAgentsMD()) {
		fmt.Fprintf(os.Stderr, "warning: copied non-empty system prompt from %s into this profile's AGENTS.md; use -blank for an empty prompt\n", l.LiveAgentsMD())
	}
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

// Export writes a portable, encrypted bundle of one or more profiles.
// Names may appear before and/or after flags; with none, all profiles export.
func Export(l paths.Layout, args []string) error {
	transfer.ToolVersion = Version
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	out := fs.String("o", "", "output .zip path (default ocp-export-<timestamp>.zip)")
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "-") {
		i++
	}
	names := append([]string{}, args[:i]...)
	if err := fs.Parse(args[i:]); err != nil {
		return err
	}
	names = append(names, fs.Args()...)

	pass, err := exportPassphrase()
	if err != nil {
		return err
	}
	return transfer.Export(l, transfer.ExportOpts{
		Names: names, Out: *out, Passphrase: pass, Log: os.Stderr,
	})
}

// Import reconstructs profiles from a bundle into the current store.
func Import(l paths.Layout, args []string) error {
	transfer.ToolVersion = Version
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite existing profiles and shared secrets")
	var bundle string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		bundle, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if bundle == "" {
		bundle = fs.Arg(0)
	}
	if bundle == "" {
		return fmt.Errorf("usage: ocp import <bundle.zip> [--force]")
	}
	pass, err := readPassphrase("passphrase for the bundle: ")
	if err != nil {
		return err
	}
	return transfer.Import(l, transfer.ImportOpts{
		Bundle: bundle, Passphrase: pass, Overwrite: *force, Log: os.Stderr,
	})
}

// readPassphrase returns $OCP_PASSPHRASE if set, else prompts without echo.
func readPassphrase(prompt string) (string, error) {
	if v := os.Getenv("OCP_PASSPHRASE"); v != "" {
		return v, nil
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// exportPassphrase prompts twice (and confirms) unless $OCP_PASSPHRASE is set.
func exportPassphrase() (string, error) {
	if v := os.Getenv("OCP_PASSPHRASE"); v != "" {
		return v, nil
	}
	p1, err := readPassphrase("passphrase to encrypt the bundle: ")
	if err != nil {
		return "", err
	}
	if p1 == "" {
		return "", fmt.Errorf("empty passphrase")
	}
	p2, err := readPassphrase("confirm passphrase: ")
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", fmt.Errorf("passphrases do not match")
	}
	return p1, nil
}

// Path prints shell export lines so a user can `eval "$(ocp path x)"` and then
// run opencode manually.
func Path(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ocp path <name>")
	}
	name := args[0]
	if name == store.ReservedDefault {
		fmt.Println("# default profile uses live dirs; no overrides needed")
		return nil
	}
	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if _, err := s.Get(name); err != nil {
		return err
	}
	fmt.Printf("export OPENCODE_CONFIG_DIR=%q\n", l.ProfileConfigOpencode(name))
	fmt.Printf("export OPENCODE_CONFIG=%q\n", l.OpencodeConfig(name))
	fmt.Printf("export OPENCODE_DB=%q\n", l.ProfileDB(name))
	return nil
}

func PrintVersion() { fmt.Println("opencode-profile " + Version) }

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func Usage() {
	fmt.Print(`opencode-profile (ocp) — isolated profiles for opencode

USAGE:
  ocp                      launch the TUI profile picker
  ocp run <name> [-- ...]  launch opencode under a profile (extra args pass through)
  ocp list | ls            list profiles
  ocp create <name>        create a profile (-desc, -blank)
  ocp rm <name>            delete a profile
  ocp export [names...]    write an encrypted .zip bundle (-o out.zip; all if no names)
  ocp import <bundle.zip>  restore profiles from a bundle (--force to overwrite)
  ocp path <name>          print export lines for the profile's opencode dirs
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
