//go:build unix

package launch

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Exec runs opencode as a child process, forwarding signals and stdio. After
// the child exits it syncs any credentials that opencode wrote to the XDG
// default data dir back into the profile's auth files, then mirrors the
// child's exit code.
func Exec(p *Plan) error {
	cmd := exec.Command(p.Bin, p.Argv[1:]...)
	cmd.Env = p.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runStartupSyncs(p.Syncs)

	if err := cmd.Start(); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	go func(pid int) {
		for sig := range sigCh {
			_ = syscall.Kill(pid, sig.(syscall.Signal))
		}
	}(cmd.Process.Pid)

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	runSyncs(p.Syncs)

	if err == nil {
		os.Exit(0)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}
