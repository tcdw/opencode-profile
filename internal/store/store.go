package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tcdw/opencode-profile/internal/paths"
)

const storeVersion = 1

const blankConfig = `{
  "$schema": "https://opencode.ai/config.json"
}
`

// Store is the in-memory view of profiles.json plus the resolved layout. It is
// the single repository layer used by both the CLI and the TUI.
type Store struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`

	layout paths.Layout
}

// Open loads profiles.json. A missing file is not an error — it yields an empty,
// uninitialized store (see IsInitialized).
func Open(l paths.Layout) (*Store, error) {
	s := &Store{Version: storeVersion, layout: l}
	data, err := os.ReadFile(l.ProfilesJSON())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", l.ProfilesJSON(), err)
	}
	s.layout = l
	s.reconcile()
	return s, nil
}

// reconcile makes recorded modes match what's actually on disk — disk is the
// source of truth, so a symlink reads as linked and a real file/dir as owned.
// Missing/broken targets keep their recorded mode (surfaced elsewhere).
func (s *Store) reconcile() {
	for i := range s.Profiles {
		p := &s.Profiles[i]
		if p.Modes == nil {
			p.Modes = defaultModes()
		}
		for _, d := range AllDomains {
			fi, err := os.Lstat(s.domainTarget(p.Name, d))
			if err != nil {
				continue
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				p.Modes[d] = ModeLinked
			} else {
				p.Modes[d] = ModeOwned
			}
		}
	}
}

func (s *Store) Layout() paths.Layout { return s.layout }

func (s *Store) IsInitialized() bool {
	_, err := os.Stat(s.layout.ProfilesJSON())
	return err == nil
}

// Save atomically rewrites profiles.json.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeAtomic(s.layout.ProfilesJSON(), data, 0o644)
}

func (s *Store) Get(name string) (*Profile, error) {
	for i := range s.Profiles {
		if s.Profiles[i].Name == name {
			return &s.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("profile %q not found", name)
}

// Init creates the store skeleton and seeds the shared base from the live
// opencode dirs. It is idempotent and never overwrites an existing shared file.
func (s *Store) Init() error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	l := s.layout
	if err := s.ensureSkeleton(); err != nil {
		return err
	}
	if src := l.LiveAuth(); fileExists(src) && !fileExists(l.SharedAuth()) {
		if err := copyFile(src, l.SharedAuth(), 0o600); err != nil {
			return err
		}
	}
	if src := l.LiveMCPAuth(); fileExists(src) && !fileExists(l.SharedMCPAuth()) {
		if err := copyFile(src, l.SharedMCPAuth(), 0o600); err != nil {
			return err
		}
	}
	if src := l.LiveSkills(); dirExists(src) {
		if err := replicateDir(src, l.SharedSkills()); err != nil {
			return err
		}
	}
	if !s.IsInitialized() {
		return s.Save()
	}
	return nil
}

// ensureSkeleton creates the store's directory skeleton. It does NOT seed from
// the live opencode dirs and does NOT take the lock — callers that already hold
// it (Init) use this directly.
func (s *Store) ensureSkeleton() error {
	for _, d := range []string{s.layout.Shared(), s.layout.SharedSkills(), s.layout.ProfilesDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// EnsureSkeleton creates the store skeleton without seeding from the live
// opencode dirs. Import uses this so a fresh machine isn't polluted by any
// pre-existing live config — only the bundle's contents land in the store.
func (s *Store) EnsureSkeleton() error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return s.ensureSkeleton()
}

// CreateOpts tunes profile creation.
type CreateOpts struct {
	Description string
	Blank       bool // seed a minimal config instead of copying the live one
	Modes       map[Domain]DomainMode
}

// Create builds a new profile directory tree, seeds its config + AGENTS.md, and
// materializes each shareable domain as a link or copy.
func (s *Store) Create(name string, opts CreateOpts) (*Profile, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if _, err := s.Get(name); err == nil {
		return nil, fmt.Errorf("profile %q already exists", name)
	}
	l := s.layout
	if dirExists(l.ProfileDir(name)) {
		return nil, fmt.Errorf("profile dir already exists: %s", l.ProfileDir(name))
	}

	modes := defaultModes()
	for k, v := range opts.Modes {
		modes[k] = v
	}

	for _, d := range []string{
		l.ProfileConfigOpencode(name),
		l.ProfileDataOpencode(name),
		l.ProfileState(name),
		l.ProfileCache(name),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	// opencode.json: copy the live config unless blank/absent.
	switch {
	case opts.Blank:
		if err := writeAtomic(l.OpencodeJSON(name), []byte(blankConfig), 0o600); err != nil {
			return nil, err
		}
	case fileExists(l.LiveOpencodeJSON()):
		if err := copyFile(l.LiveOpencodeJSON(), l.OpencodeJSON(name), 0o600); err != nil {
			return nil, err
		}
	default:
		if err := writeAtomic(l.OpencodeJSON(name), []byte(blankConfig), 0o600); err != nil {
			return nil, err
		}
	}

	// AGENTS.md: copy the live prompt if present, else start empty.
	if !opts.Blank && fileExists(l.LiveAgentsMD()) {
		if err := copyFile(l.LiveAgentsMD(), l.AgentsMD(name), 0o644); err != nil {
			return nil, err
		}
	}
	if !fileExists(l.AgentsMD(name)) {
		if err := writeAtomic(l.AgentsMD(name), []byte{}, 0o644); err != nil {
			return nil, err
		}
	}

	for _, d := range AllDomains {
		eff, err := s.materializeDomain(name, d, modes[d])
		if err != nil {
			return nil, err
		}
		modes[d] = eff // a linked domain may degrade to owned on symlink-hostile FS
	}
	p := &Profile{Name: name, Description: opts.Description, Modes: modes, CreatedAt: time.Now()}

	s.Profiles = append(s.Profiles, *p)
	if err := s.Save(); err != nil {
		return nil, err
	}
	return p, nil
}

// Remove deletes a profile's directory tree and its store entry. Symlinks
// inside the tree are removed as links, never followed into the shared base.
func (s *Store) Remove(name string) error {
	if name == ReservedDefault {
		return errors.New("cannot remove the built-in default profile")
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	idx := -1
	for i := range s.Profiles {
		if s.Profiles[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("profile %q not found", name)
	}
	pdir := s.layout.ProfileDir(name)
	if err := mustBeUnderRoot(s.layout.Root, pdir); err != nil {
		return err
	}
	if err := os.RemoveAll(pdir); err != nil {
		return err
	}
	s.Profiles = append(s.Profiles[:idx], s.Profiles[idx+1:]...)
	return s.Save()
}

// SetMode switches a domain between sharing the base (linked) and owning a copy
// (owned). Going owned→linked backs up real data rather than deleting it; going
// linked→owned replaces the symlink with a self-contained copy.
func (s *Store) SetMode(name string, d Domain, to DomainMode) error {
	p, err := s.Get(name)
	if err != nil {
		return err
	}
	if p.Modes[d] == to {
		return nil
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	target := s.domainTarget(name, d)
	source := s.domainShared(d)
	if err := mustBeUnderRoot(s.layout.Root, target); err != nil {
		return err
	}

	switch to {
	case ModeOwned:
		if err := removeIfExists(target); err != nil { // drop the symlink
			return err
		}
		if err := copyShareInto(d, source, target); err != nil {
			return err
		}
	case ModeLinked:
		if err := backupIfReal(target); err != nil {
			return err
		}
		if err := ensureSharedExists(d, source); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown domain mode %q", to)
	}

	p.Modes[d] = to
	return s.Save()
}

func (s *Store) domainTarget(name string, d Domain) string {
	switch d {
	case DomainAuth:
		return s.layout.ProfileAuth(name)
	case DomainMCPAuth:
		return s.layout.ProfileMCPAuth(name)
	case DomainSkills:
		return s.layout.ProfileSkills(name)
	}
	return ""
}

func (s *Store) domainShared(d Domain) string {
	switch d {
	case DomainAuth:
		return s.layout.SharedAuth()
	case DomainMCPAuth:
		return s.layout.SharedMCPAuth()
	case DomainSkills:
		return s.layout.SharedSkills()
	}
	return ""
}

// materializeDomain (re)creates a domain's on-disk form: a symlink into the
// shared base (linked) or a self-contained copy (owned). It returns the
// effective mode, which differs from the requested one only when a linked
// domain has to degrade to owned because the filesystem refuses symlinks.
func (s *Store) materializeDomain(name string, d Domain, mode DomainMode) (DomainMode, error) {
	target := s.domainTarget(name, d)
	source := s.domainShared(d)
	if err := mustBeUnderRoot(s.layout.Root, target); err != nil {
		return mode, err
	}
	if err := removeIfExists(target); err != nil {
		return mode, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return mode, err
	}

	switch mode {
	case ModeLinked:
		if err := ensureSharedExists(d, source); err != nil {
			return mode, err
		}
		if err := os.Symlink(source, target); err != nil {
			// A restricted filesystem (notably Windows without the symlink
			// privilege) refuses links; fall back to a self-contained copy so
			// the profile still works, and report the downgraded mode.
			if cErr := copyShareInto(d, source, target); cErr != nil {
				return mode, fmt.Errorf("symlink %s failed (%v) and copy fallback failed: %w", target, err, cErr)
			}
			return ModeOwned, nil
		}
		return ModeLinked, nil
	case ModeOwned:
		if err := copyShareInto(d, source, target); err != nil {
			return mode, err
		}
		return ModeOwned, nil
	}
	return mode, fmt.Errorf("unknown domain mode %q", mode)
}

// copyShareInto writes a self-contained (owned) copy of a domain's shared base
// into target: a deep copy for skills, the shared file (or an empty JSON object
// when absent) for auth/mcp_auth.
func copyShareInto(d Domain, source, target string) error {
	if d == DomainSkills {
		if dirExists(source) {
			return copyTree(source, target)
		}
		return os.MkdirAll(target, 0o755)
	}
	if fileExists(source) {
		return copyFile(source, target, 0o600)
	}
	return writeAtomic(target, []byte("{}\n"), 0o600)
}

// MaterializeDomain (re)creates a domain on disk for an imported profile and
// returns the effective mode. Unlike the internal helper it takes the store
// lock, so import (a separate package) can call it safely.
func (s *Store) MaterializeDomain(name string, d Domain, want DomainMode) (DomainMode, error) {
	unlock, err := s.lock()
	if err != nil {
		return want, err
	}
	defer unlock()
	return s.materializeDomain(name, d, want)
}

// AddProfile appends an externally-constructed profile (e.g. assembled by
// import) and persists it. It refuses a duplicate name; a caller wanting to
// replace an existing profile must Remove it first.
func (s *Store) AddProfile(p Profile) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := s.Get(p.Name); err == nil {
		return fmt.Errorf("profile %q already exists", p.Name)
	}
	if p.Modes == nil {
		p.Modes = defaultModes()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.Profiles = append(s.Profiles, p)
	return s.Save()
}

// ensureSharedExists makes sure a linked symlink won't dangle.
func ensureSharedExists(d Domain, source string) error {
	if d == DomainSkills {
		return os.MkdirAll(source, 0o755)
	}
	if !fileExists(source) {
		return writeAtomic(source, []byte("{}\n"), 0o600)
	}
	return nil
}
