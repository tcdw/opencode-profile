// Package transfer moves profiles between machines: Export writes a portable
// .zip bundle (plaintext config + an encrypted secrets blob), and Import
// reconstructs the store from one. It is the groundwork for cross-platform use
// (e.g. carrying a macOS store to Windows), so it deliberately stores no
// symlinks and rewrites absolute path references on import.
package transfer

import (
	"time"

	"github.com/tcdw/opencode-profile/internal/store"
)

// bundleSchema is the on-disk format version of a bundle. Import refuses a
// bundle whose schema it doesn't understand.
const bundleSchema = 1

// Bundle entry names (zip uses forward slashes regardless of OS).
const (
	manifestName = "manifest.json"
	secretsName  = "secrets.enc"
	sharedPrefix = "shared/"   // shared/skills/... travels in plaintext
	profPrefix   = "profiles/" // profiles/<name>/{opencode.json[c],AGENTS.md,skills/...}
)

// secretsMode values for Manifest.Secrets.Mode.
const secretsEncrypted = "encrypted"

// Manifest is the bundle's table of contents and the metadata import needs to
// rebuild the store faithfully on a different machine.
type Manifest struct {
	Schema      int            `json:"schema"`
	Tool        string         `json:"tool"`
	ToolVersion string         `json:"tool_version"`
	CreatedAt   time.Time      `json:"created_at"`
	SourceOS    string         `json:"source_os"`   // runtime.GOOS at export time
	SourceRoot  string         `json:"source_root"` // absolute OCP_HOME; import rewrites {file:} refs from this
	Secrets     SecretsInfo    `json:"secrets"`
	Profiles    []ProfileEntry `json:"profiles"`
}

// SecretsInfo describes how the encrypted secrets blob was produced and which
// logical files it carries.
type SecretsInfo struct {
	Mode  string   `json:"mode"`
	KDF   string   `json:"kdf,omitempty"`
	Iter  int      `json:"iter,omitempty"`
	Files []string `json:"files,omitempty"` // logical paths packed inside secrets.enc
}

// ProfileEntry is one profile's portable metadata. The recorded Modes drive
// re-materialization on import (linked → symlink-or-copy, owned → real copy).
type ProfileEntry struct {
	Name        string                            `json:"name"`
	Description string                            `json:"description,omitempty"`
	Modes       map[store.Domain]store.DomainMode `json:"modes"`
	CreatedAt   time.Time                         `json:"created_at"`
	Model       string                            `json:"model,omitempty"` // informational only
}
