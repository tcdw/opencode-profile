// Package paths resolves every on-disk location ocp cares about: the profile
// store under ~/.opencode-profiles, the per-profile XDG targets, the live
// opencode dirs (read-only, for seeding), and the opencode binary itself.
package paths

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// Layout is rooted at the ocp store directory (default ~/.opencode-profiles,
// overridable via $OCP_HOME). All other paths derive from Root.
type Layout struct {
	Root string
}

// Default returns the Layout rooted at $OCP_HOME, falling back to
// ~/.opencode-profiles.
func Default() (Layout, error) {
	if v := os.Getenv("OCP_HOME"); v != "" {
		return Layout{Root: v}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Layout{}, err
	}
	return Layout{Root: filepath.Join(home, ".opencode-profiles")}, nil
}

// --- ocp store layout ---

func (l Layout) ProfilesJSON() string  { return filepath.Join(l.Root, "profiles.json") }
func (l Layout) Lock() string          { return filepath.Join(l.Root, ".lock") }
func (l Layout) Shared() string        { return filepath.Join(l.Root, "shared") }
func (l Layout) SharedAuth() string    { return filepath.Join(l.Shared(), "auth.json") }
func (l Layout) SharedMCPAuth() string { return filepath.Join(l.Shared(), "mcp-auth.json") }
func (l Layout) SharedSkills() string  { return filepath.Join(l.Shared(), "skills") }
func (l Layout) ProfilesDir() string   { return filepath.Join(l.Root, "profiles") }

// --- per-profile layout ---

func (l Layout) ProfileDir(name string) string { return filepath.Join(l.ProfilesDir(), name) }

// ProfileConfig is the value handed to opencode as XDG_CONFIG_HOME; opencode
// then resolves its config dir to <here>/opencode.
func (l Layout) ProfileConfig(name string) string { return filepath.Join(l.ProfileDir(name), "config") }

// ProfileData is the value handed to opencode as XDG_DATA_HOME.
func (l Layout) ProfileData(name string) string  { return filepath.Join(l.ProfileDir(name), "data") }
func (l Layout) ProfileState(name string) string { return filepath.Join(l.ProfileDir(name), "state") }
func (l Layout) ProfileCache(name string) string { return filepath.Join(l.ProfileDir(name), "cache") }

func (l Layout) ProfileConfigOpencode(name string) string {
	return filepath.Join(l.ProfileConfig(name), "opencode")
}
func (l Layout) ProfileDataOpencode(name string) string {
	return filepath.Join(l.ProfileData(name), "opencode")
}
func (l Layout) OpencodeJSON(name string) string {
	return filepath.Join(l.ProfileConfigOpencode(name), "opencode.json")
}
func (l Layout) AgentsMD(name string) string {
	return filepath.Join(l.ProfileConfigOpencode(name), "AGENTS.md")
}
func (l Layout) ProfileSkills(name string) string {
	return filepath.Join(l.ProfileConfigOpencode(name), "skills")
}
func (l Layout) ProfileAuth(name string) string {
	return filepath.Join(l.ProfileDataOpencode(name), "auth.json")
}
func (l Layout) ProfileMCPAuth(name string) string {
	return filepath.Join(l.ProfileDataOpencode(name), "mcp-auth.json")
}

// --- live opencode dirs (read-only; used only for seeding) ---
//
// opencode follows XDG even on macOS, so we must NOT use os.UserConfigDir
// (which returns ~/Library/Application Support on darwin).

func xdgConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func xdgDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func (l Layout) LiveConfigOpencode() string { return filepath.Join(xdgConfigHome(), "opencode") }
func (l Layout) LiveDataOpencode() string   { return filepath.Join(xdgDataHome(), "opencode") }
func (l Layout) LiveOpencodeJSON() string {
	return filepath.Join(l.LiveConfigOpencode(), "opencode.json")
}
func (l Layout) LiveAgentsMD() string { return filepath.Join(l.LiveConfigOpencode(), "AGENTS.md") }
func (l Layout) LiveSkills() string   { return filepath.Join(l.LiveConfigOpencode(), "skills") }
func (l Layout) LiveAuth() string     { return filepath.Join(l.LiveDataOpencode(), "auth.json") }
func (l Layout) LiveMCPAuth() string  { return filepath.Join(l.LiveDataOpencode(), "mcp-auth.json") }

// FindOpencode locates the opencode binary: PATH first, then the well-known
// install location, returning an absolute path suitable for syscall.Exec.
func FindOpencode() (string, error) {
	if p, err := exec.LookPath("opencode"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs, nil
		}
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		fallback := filepath.Join(home, ".opencode", "bin", "opencode")
		if fi, err := os.Stat(fallback); err == nil && !fi.IsDir() {
			return fallback, nil
		}
	}
	return "", errors.New("opencode not found on PATH or at ~/.opencode/bin/opencode")
}
