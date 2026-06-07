//go:build unix

package launch

import "syscall"

// Exec hands off to the planned command by replacing the current process. On
// success it never returns — opencode inherits ocp's terminal and pid, which is
// the clean handoff ocp relies on. Only a failed exec returns an error.
func Exec(p *Plan) error {
	return syscall.Exec(p.Bin, p.Argv, p.Env)
}
