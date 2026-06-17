package launch

import (
	"errors"
	"os"
	"os/exec"
)

// Exec runs the planned command as a child process and exits with its status.
// Windows has no exec()-style process replacement, so instead of handing off the
// pid ocp forwards stdio and the environment, waits, syncs credentials, then
// mirrors the child's exit code. It only returns an error when the child fails
// to start.
func Exec(p *Plan) error {
	cmd := exec.Command(p.Bin, p.Argv[1:]...)
	cmd.Env = p.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runStartupSyncs(p.Syncs)
	err := cmd.Run()

	runSyncs(p.Syncs)

	if err == nil {
		os.Exit(0)
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		os.Exit(ee.ExitCode())
	}
	return err
}
