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
	"regexp"
	"runtime"
	"time"

	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

// ToolVersion is stamped into a bundle's manifest. main wires it to the build
// version so a bundle records which ocp produced it.
var ToolVersion = "dev"

// ExportOpts configures Export.
type ExportOpts struct {
	Names      []string  // profiles to export; empty means all
	Out        string    // output .zip path; empty derives ocp-export-<ts>.zip
	Passphrase string    // required: secrets are always encrypted
	Now        time.Time // timestamp for the manifest and zip entries (injectable for tests)
	Log        io.Writer // warnings/progress; nil means discard
}

type secretItem struct {
	logical string // path inside secrets.enc (and the manifest)
	disk    string // source path on disk
}

// Export writes a portable bundle: plaintext config (AGENTS.md,
// opencode.json/jsonc, shared & owned skills) plus an encrypted blob holding every secret file.
// Symlinks are never stored — domain modes in the manifest let import rebuild
// them — and skills symlinks are dereferenced so the bundle is self-contained.
func Export(l paths.Layout, opts ExportOpts) error {
	logw := opts.Log
	if logw == nil {
		logw = io.Discard
	}
	if opts.Passphrase == "" {
		return fmt.Errorf("a passphrase is required (secrets are encrypted)")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	s, err := store.Open(l)
	if err != nil {
		return err
	}

	selected, err := selectProfiles(s, opts.Names)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return fmt.Errorf("no profiles to export")
	}

	out := opts.Out
	if out == "" {
		out = fmt.Sprintf("ocp-export-%s.zip", now.Format("20060102-150405"))
	}

	// Collect secret files first so the manifest can list them.
	items := collectSecrets(l, selected)

	man := Manifest{
		Schema:      bundleSchema,
		Tool:        "opencode-profile",
		ToolVersion: ToolVersion,
		CreatedAt:   now,
		SourceOS:    runtime.GOOS,
		SourceRoot:  absRoot(l),
		Secrets: SecretsInfo{
			Mode:  secretsEncrypted,
			KDF:   kdfName,
			Iter:  kdfIter,
			Files: logicalPaths(items),
		},
	}
	for _, p := range selected {
		man.Profiles = append(man.Profiles, ProfileEntry{
			Name:        p.Name,
			Description: p.Description,
			Modes:       p.Modes,
			CreatedAt:   p.CreatedAt,
			Model:       store.ReadModel(l, p.Name),
		})
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	if err := writeBundle(zw, l, man, selected, items, opts.Passphrase, now, logw); err != nil {
		zw.Close()
		f.Close()
		os.Remove(out)
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(out)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(out)
		return err
	}
	fmt.Fprintf(logw, "exported %d profile(s) to %s\n", len(selected), out)
	return nil
}

func writeBundle(zw *zip.Writer, l paths.Layout, man Manifest, selected []store.Profile, items []secretItem, pass string, now time.Time, logw io.Writer) error {
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	if err := zipBytes(zw, manifestName, append(manBytes, '\n'), 0o644, now); err != nil {
		return err
	}

	// Plaintext: shared skills (dereferenced) and each profile's config.
	if err := addTreeDeref(zw, l.SharedSkills(), sharedPrefix+"skills/", now); err != nil {
		return err
	}
	for _, p := range selected {
		base := profPrefix + p.Name + "/"
		cfg := l.OpencodeConfig(p.Name)
		data, err := os.ReadFile(cfg)
		if err != nil {
			return fmt.Errorf("profile %q missing opencode config at %s: %w", p.Name, cfg, err)
		}
		for _, w := range scanInlineSecrets(data) {
			fmt.Fprintf(logw, "warning: %s %s: %s\n", p.Name, filepath.Base(cfg), w)
		}
		if err := zipBytes(zw, base+filepath.Base(cfg), data, 0o600, now); err != nil {
			return err
		}
		if data, err := os.ReadFile(l.AgentsMD(p.Name)); err == nil {
			if err := zipBytes(zw, base+"AGENTS.md", data, 0o644, now); err != nil {
				return err
			}
		}
		if p.Modes[store.DomainSkills] == store.ModeOwned {
			if err := addTreeDeref(zw, l.ProfileSkills(p.Name), base+"skills/", now); err != nil {
				return err
			}
		}
	}

	// Encrypted: tar of every secret file, sealed under the passphrase.
	tarBytes, err := buildSecretsTar(items)
	if err != nil {
		return err
	}
	blob, err := Seal(tarBytes, pass)
	if err != nil {
		return err
	}
	return zipBytes(zw, secretsName, blob, 0o600, now)
}

// selectProfiles resolves names to profiles, or returns all when names is empty.
func selectProfiles(s *store.Store, names []string) ([]store.Profile, error) {
	if len(names) == 0 {
		return s.Profiles, nil
	}
	out := make([]store.Profile, 0, len(names))
	for _, n := range names {
		p, err := s.Get(n)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

// collectSecrets gathers every secret file: all non-dir files directly under
// shared/ (auth.json, mcp-auth.json, *.key) plus the owned data secrets of each
// selected profile.
func collectSecrets(l paths.Layout, selected []store.Profile) []secretItem {
	var items []secretItem
	if entries, err := os.ReadDir(l.Shared()); err == nil {
		for _, e := range entries {
			disk := filepath.Join(l.Shared(), e.Name())
			fi, err := os.Stat(disk) // follow a possible symlink
			if err != nil || fi.IsDir() {
				continue // skills/ is the only dir; it travels in plaintext
			}
			items = append(items, secretItem{logical: sharedPrefix + e.Name(), disk: disk})
		}
	}
	for _, p := range selected {
		if p.Modes[store.DomainAuth] == store.ModeOwned && isFile(l.ProfileAuth(p.Name)) {
			items = append(items, secretItem{
				logical: profPrefix + p.Name + "/data/opencode/auth.json",
				disk:    l.ProfileAuth(p.Name),
			})
		}
		if p.Modes[store.DomainMCPAuth] == store.ModeOwned && isFile(l.ProfileMCPAuth(p.Name)) {
			items = append(items, secretItem{
				logical: profPrefix + p.Name + "/data/opencode/mcp-auth.json",
				disk:    l.ProfileMCPAuth(p.Name),
			})
		}
	}
	return items
}

func logicalPaths(items []secretItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.logical
	}
	return out
}

func buildSecretsTar(items []secretItem) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, it := range items {
		data, err := os.ReadFile(it.disk)
		if err != nil {
			return nil, err
		}
		hdr := &tar.Header{Name: it.logical, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// addTreeDeref copies a directory tree into the zip, following symlinks so the
// result is self-contained (skill entries are often symlinks into a store).
// A missing root or a dangling link is skipped, not an error.
func addTreeDeref(zw *zip.Writer, diskDir, zipPrefix string, now time.Time) error {
	entries, err := os.ReadDir(diskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		disk := filepath.Join(diskDir, e.Name())
		name := zipPrefix + e.Name()
		fi, err := os.Stat(disk) // follow symlinks
		if err != nil {
			continue // dangling link or vanished entry
		}
		if fi.IsDir() {
			if err := addTreeDeref(zw, disk, name+"/", now); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(disk)
		if err != nil {
			return err
		}
		if err := zipBytes(zw, name, data, 0o644, now); err != nil {
			return err
		}
	}
	return nil
}

func zipBytes(zw *zip.Writer, name string, data []byte, mode os.FileMode, now time.Time) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now}
	hdr.SetMode(mode)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func absRoot(l paths.Layout) string {
	if abs, err := filepath.Abs(l.Root); err == nil {
		return abs
	}
	return l.Root
}

// scanInlineSecrets warns about values that look like literal secrets in
// opencode.json, which is stored in the bundle's plaintext region. References
// via {file:...} or {env:...} are fine; a raw key is not.
func scanInlineSecrets(data []byte) []string {
	var warns []string
	for _, m := range secretFieldRe.FindAllSubmatch(data, -1) {
		field, val := string(m[1]), string(m[2])
		if bytes.Contains(m[2], []byte("{file:")) || bytes.Contains(m[2], []byte("{env:")) {
			continue
		}
		warns = append(warns, fmt.Sprintf("inline %q looks like a literal secret (prefer {file:} or {env:}): %s", field, mask(val)))
	}
	if literalKeyRe.Match(data) {
		warns = append(warns, "contains a value matching a literal API-key pattern (e.g. sk-...)")
	}
	return warns
}

var (
	secretFieldRe = regexp.MustCompile(`(?i)"(api[_-]?key|apikey|token|secret|authorization|password)"\s*:\s*"([^"]+)"`)
	literalKeyRe  = regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`)
)

func mask(s string) string {
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "…" + "******"
}
