package launch

import (
	"errors"
	"os"
	"os/exec"
)

// Exec runs the planned command as a child process and exits with its status.
// Windows has no exec()-style process replacement, so instead of handing off the
// pid ocp forwards stdio and the environment, waits, then mirrors the child's
// exit code. It only returns an error when the child fails to start.
func Exec(p *Plan) error {
	// p.Argv[0] is the conventional command name ("opencode"); the real binary
	// is p.Bin and the actual arguments are p.Argv[1:].
	cmd := exec.Command(p.Bin, p.Argv[1:]...)
	cmd.Env = p.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		os.Exit(ee.ExitCode())
	}
	return err // failed to start (e.g. binary not found)
}
