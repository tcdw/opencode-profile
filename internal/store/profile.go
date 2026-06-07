package store

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// DomainMode says whether a shareable domain points at the shared base
// (a symlink) or is a self-contained copy owned by the profile.
type DomainMode string

const (
	ModeLinked DomainMode = "linked"
	ModeOwned  DomainMode = "owned"
)

// Domain is one of the three things a profile may share with the base. The
// system prompt (AGENTS.md), opencode.json, and the session DB are always
// per-profile and never appear here.
type Domain string

const (
	DomainAuth    Domain = "auth"     // data/opencode/auth.json
	DomainMCPAuth Domain = "mcp_auth" // data/opencode/mcp-auth.json
	DomainSkills  Domain = "skills"   // config/opencode/skills/
)

// AllDomains is the canonical iteration order for display and materialization.
var AllDomains = []Domain{DomainAuth, DomainMCPAuth, DomainSkills}

// ReservedDefault is the synthetic profile that runs opencode against the live
// dirs with no XDG override. It is never stored in profiles.json.
const ReservedDefault = "default"

// Profile is one isolated opencode environment. Modes is the source of truth
// recorded in profiles.json; it is reconciled against disk on load.
type Profile struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Modes       map[Domain]DomainMode `json:"modes"`
	CreatedAt   time.Time             `json:"created_at"`
}

func defaultModes() map[Domain]DomainMode {
	return map[Domain]DomainMode{
		DomainAuth:    ModeLinked,
		DomainMCPAuth: ModeLinked,
		DomainSkills:  ModeLinked,
	}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// ValidateName rejects the reserved name, path-traversal, and shell-hostile
// characters before any name is turned into a filesystem path.
func ValidateName(name string) error {
	if name == ReservedDefault {
		return errors.New(`"default" is reserved for the built-in profile`)
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid profile name %q (use letters, digits, _.- ; 1-64 chars; must start alphanumeric)", name)
	}
	return nil
}
