package transfer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

// ImportOpts configures Import.
type ImportOpts struct {
	Bundle     string    // path to the .zip bundle
	Passphrase string    // needed to decrypt the secrets blob
	Overwrite  bool      // replace existing profiles / shared secrets instead of skipping
	Now        time.Time // timestamp for .bak names (injectable for tests)
	Log        io.Writer // progress/notes; nil means discard
}

// Import reconstructs profiles from a bundle into the current store (l). It
// never reads or writes the live opencode dirs, rebuilds linked domains from
// recorded modes (degrading to copies where symlinks are unavailable), and
// rewrites absolute {file:} references from the source root to this machine's.
func Import(l paths.Layout, opts ImportOpts) error {
	logw := opts.Log
	if logw == nil {
		logw = io.Discard
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	zr, err := zip.OpenReader(opts.Bundle)
	if err != nil {
		return err
	}
	defer zr.Close()
	index := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		index[f.Name] = f
	}

	mf := index[manifestName]
	if mf == nil {
		return fmt.Errorf("not an ocp bundle: missing %s", manifestName)
	}
	mb, err := readZip(mf)
	if err != nil {
		return err
	}
	var man Manifest
	if err := json.Unmarshal(mb, &man); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if man.Schema != bundleSchema {
		return fmt.Errorf("unsupported bundle schema %d (this ocp understands %d)", man.Schema, bundleSchema)
	}

	s, err := store.Open(l)
	if err != nil {
		return err
	}
	if err := s.EnsureSkeleton(); err != nil {
		return err
	}

	// Decrypt secrets (if the bundle carries any).
	var secrets map[string][]byte
	if sf := index[secretsName]; sf != nil {
		if opts.Passphrase == "" {
			return fmt.Errorf("bundle contains encrypted secrets; a passphrase is required")
		}
		blob, err := readZip(sf)
		if err != nil {
			return err
		}
		plain, err := Open(blob, opts.Passphrase)
		if err != nil {
			return err
		}
		secrets, err = untar(plain)
		if err != nil {
			return fmt.Errorf("read secrets archive: %w", err)
		}
	}

	dstRoot := absRoot(l)

	// Shared base: plaintext skills, then the shared secret files.
	if err := extractPrefix(index, sharedPrefix+"skills/", l.SharedSkills()); err != nil {
		return err
	}
	for logical, data := range secrets {
		rel := strings.TrimPrefix(logical, sharedPrefix)
		if !strings.HasPrefix(logical, sharedPrefix) || strings.Contains(rel, "/") {
			continue // not a top-level shared secret (owned profile secrets handled below)
		}
		dest, err := safeJoin(l.Shared(), rel)
		if err != nil {
			return err
		}
		if err := placeShared(dest, data, opts.Overwrite, now, logw); err != nil {
			return err
		}
	}

	// Global config/data: restored into the global/ symlinks (i.e. the live
	// opencode dirs). Existing files are kept unless --force is set.
	if err := extractGlobal(index, l, opts.Overwrite, now, logw); err != nil {
		return err
	}

	for _, pe := range man.Profiles {
		if err := importProfile(l, s, pe, index, secrets, man.SourceRoot, dstRoot, opts.Overwrite, logw); err != nil {
			return err
		}
	}
	return nil
}

func importProfile(l paths.Layout, s *store.Store, pe ProfileEntry, index map[string]*zip.File, secrets map[string][]byte, srcRoot, dstRoot string, overwrite bool, logw io.Writer) error {
	name := pe.Name
	if err := store.ValidateName(name); err != nil {
		fmt.Fprintf(logw, "skip %q: %v\n", name, err)
		return nil
	}
	if _, err := s.Get(name); err == nil {
		if !overwrite {
			fmt.Fprintf(logw, "skip existing profile %q (use --force to overwrite)\n", name)
			return nil
		}
		if err := s.Remove(name); err != nil {
			return err
		}
	}

	for _, d := range []string{
		l.ProfileConfigOpencode(name), l.ProfileDataOpencode(name),
		l.ProfileState(name), l.ProfileCache(name),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	base := profPrefix + name + "/"
	// opencode config with absolute path refs rewritten to this machine's root.
	if f, cfgName := bundleConfig(index, base); f != nil {
		raw, err := readZip(f)
		if err != nil {
			return err
		}
		dst := filepath.Join(l.ProfileConfigOpencode(name), cfgName)
		if err := writeFile(dst, rewriteRefs(raw, srcRoot, dstRoot), 0o600); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("bundle profile %q is missing opencode.json or opencode.jsonc", name)
	}
	// AGENTS.md
	if f := index[base+"AGENTS.md"]; f != nil {
		raw, err := readZip(f)
		if err != nil {
			return err
		}
		if err := writeFile(l.AgentsMD(name), raw, 0o644); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("bundle profile %q is missing AGENTS.md", name)
	}

	// owned domain payloads placed before materialization.
	if pe.Modes[store.DomainSkills] == store.ModeOwned {
		if err := os.MkdirAll(l.ProfileSkills(name), 0o755); err != nil {
			return err
		}
		if err := extractPrefix(index, base+"skills/", l.ProfileSkills(name)); err != nil {
			return err
		}
	}
	if pe.Modes[store.DomainAuth] == store.ModeOwned {
		if b, ok := secrets[base+"data/opencode/auth.json"]; ok {
			if err := writeFile(l.ProfileAuth(name), b, 0o600); err != nil {
				return err
			}
		}
	}
	if pe.Modes[store.DomainMCPAuth] == store.ModeOwned {
		if b, ok := secrets[base+"data/opencode/mcp-auth.json"]; ok {
			if err := writeFile(l.ProfileMCPAuth(name), b, 0o600); err != nil {
				return err
			}
		}
	}

	// Materialize each domain, recording the effective (possibly degraded) mode.
	eff := make(map[store.Domain]store.DomainMode, len(store.AllDomains))
	for _, d := range store.AllDomains {
		want := pe.Modes[d]
		if want == "" {
			want = store.ModeLinked
		}
		if want == store.ModeOwned && ownedPresent(l, name, d) {
			eff[d] = store.ModeOwned
			continue
		}
		m, err := s.MaterializeDomain(name, d, want)
		if err != nil {
			return err
		}
		if want == store.ModeLinked && m != store.ModeLinked {
			fmt.Fprintf(logw, "note: %s/%s degraded linked→owned (symlinks unavailable here)\n", name, d)
		}
		eff[d] = m
	}

	if err := s.AddProfile(store.Profile{
		Name: name, Description: pe.Description, Modes: eff, CreatedAt: pe.CreatedAt,
	}); err != nil {
		return err
	}
	fmt.Fprintf(logw, "imported profile %q\n", name)
	return nil
}

func ownedPresent(l paths.Layout, name string, d store.Domain) bool {
	switch d {
	case store.DomainAuth:
		return isFile(l.ProfileAuth(name))
	case store.DomainMCPAuth:
		return isFile(l.ProfileMCPAuth(name))
	case store.DomainSkills:
		return isDir(l.ProfileSkills(name))
	}
	return false
}

func bundleConfig(index map[string]*zip.File, base string) (*zip.File, string) {
	for _, name := range []string{paths.OpencodeJSONCName, paths.OpencodeJSONName} {
		if f := index[base+name]; f != nil {
			return f, name
		}
	}
	return nil, ""
}

// rewriteRefs swaps the source machine's OCP_HOME prefix for this machine's,
// fixing {file:<root>/...} references in opencode config. The replacement uses
// forward slashes, which Go's path resolution accepts on every OS.
func rewriteRefs(data []byte, srcRoot, dstRoot string) []byte {
	if srcRoot == "" {
		return data
	}
	return bytes.ReplaceAll(data, []byte(srcRoot), []byte(filepath.ToSlash(dstRoot)))
}

// placeShared writes a shared secret, keeping any existing file unless overwrite
// is set (in which case the old one is moved aside to a timestamped .bak).
func placeShared(path string, data []byte, overwrite bool, now time.Time, logw io.Writer) error {
	if lexists(path) {
		if !overwrite {
			fmt.Fprintf(logw, "kept existing %s (use --force to overwrite)\n", filepath.Base(path))
			return nil
		}
	}
	return placeFile(path, data, 0o600, overwrite, now)
}

// placeFile writes data to path, backing up any existing file when overwrite is
// true and leaving it untouched when overwrite is false.
func placeFile(path string, data []byte, perm os.FileMode, overwrite bool, now time.Time) error {
	if lexists(path) {
		if !overwrite {
			return nil
		}
		if err := os.Rename(path, fmt.Sprintf("%s.bak-%d", path, now.Unix())); err != nil {
			return err
		}
	}
	return writeFile(path, data, perm)
}

// extractGlobal restores the live opencode config/data pointed at by the
// global/ symlinks. Existing files are kept unless overwrite is set.
func extractGlobal(index map[string]*zip.File, l paths.Layout, overwrite bool, now time.Time, logw io.Writer) error {
	if err := extractPrefixWithOverwrite(index, globalPrefix+"config/opencode/", l.GlobalConfigDir(), 0o644, overwrite, now, logw); err != nil {
		return err
	}
	if err := extractPrefixWithOverwrite(index, globalPrefix+"data/opencode/", l.GlobalDataDir(), 0o600, overwrite, now, logw); err != nil {
		return err
	}
	return nil
}

// extractPrefixWithOverwrite writes every non-directory zip entry under prefix
// into destBase, preserving the sub-path and guarding against zip-slip. Existing
// files are kept unless overwrite is set, in which case they are backed up.
func extractPrefixWithOverwrite(index map[string]*zip.File, prefix, destBase string, perm os.FileMode, overwrite bool, now time.Time, logw io.Writer) error {
	for name, f := range index {
		if !strings.HasPrefix(name, prefix) || strings.HasSuffix(name, "/") {
			continue
		}
		rel := name[len(prefix):]
		if rel == "" {
			continue
		}
		dest, err := safeJoin(destBase, rel)
		if err != nil {
			return err
		}
		if lexists(dest) && !overwrite {
			fmt.Fprintf(logw, "kept existing global %s (use --force to overwrite)\n", rel)
			continue
		}
		data, err := readZip(f)
		if err != nil {
			return err
		}
		if err := placeFile(dest, data, perm, overwrite, now); err != nil {
			return err
		}
	}
	return nil
}

// extractPrefix writes every non-directory zip entry under prefix into destBase,
// preserving the sub-path and guarding against zip-slip.
func extractPrefix(index map[string]*zip.File, prefix, destBase string) error {
	for name, f := range index {
		if !strings.HasPrefix(name, prefix) || strings.HasSuffix(name, "/") {
			continue
		}
		rel := name[len(prefix):]
		if rel == "" {
			continue
		}
		dest, err := safeJoin(destBase, rel)
		if err != nil {
			return err
		}
		data, err := readZip(f)
		if err != nil {
			return err
		}
		if err := writeFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func untar(b []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(b))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[hdr.Name] = data
	}
	return out, nil
}

func readZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// safeJoin joins rel under base, rejecting any result that escapes base.
func safeJoin(base, rel string) (string, error) {
	dest := filepath.Join(base, filepath.FromSlash(rel))
	rp, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	dp, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	r, err := filepath.Rel(rp, dp)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in bundle: %s", rel)
	}
	return dest, nil
}

func writeFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	return os.Chmod(path, perm) // enforce perm even if the file pre-existed
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func lexists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
