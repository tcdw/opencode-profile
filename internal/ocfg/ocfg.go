// Package ocfg does surgical, non-destructive reads and writes of opencode.json.
// gjson/sjson edit the document at the byte level, so toggling one MCP server or
// changing the model leaves the rest of the (possibly large) file untouched.
package ocfg

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// GetModel returns the "model" field, or "" if unset/unreadable.
func GetModel(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return gjson.GetBytes(data, "model").String()
}

// SetModel sets (or clears, when model is "") the "model" field in place.
func SetModel(path, model string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var out []byte
	if model == "" {
		out, err = sjson.DeleteBytes(data, "model")
	} else {
		out, err = sjson.SetBytes(data, "model", model)
	}
	if err != nil {
		return err
	}
	return writeAtomic(path, out, 0o600)
}

// MCPEntry is one configured MCP server. A missing "enabled" field defaults to
// enabled, matching opencode's behavior.
type MCPEntry struct {
	Name    string
	Type    string
	Enabled bool
}

func ListMCP(path string) ([]MCPEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	res := gjson.GetBytes(data, "mcp")
	if !res.Exists() {
		return nil, nil
	}
	var entries []MCPEntry
	res.ForEach(func(k, v gjson.Result) bool {
		enabled := true
		if e := v.Get("enabled"); e.Exists() {
			enabled = e.Bool()
		}
		entries = append(entries, MCPEntry{
			Name:    k.String(),
			Type:    v.Get("type").String(),
			Enabled: enabled,
		})
		return true
	})
	return entries, nil
}

// SetMCPEnabled flips one server's enabled flag, preserving the rest of the file.
func SetMCPEnabled(path, name string, enabled bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := sjson.SetBytes(data, "mcp."+escapeKey(name)+".enabled", enabled)
	if err != nil {
		return err
	}
	return writeAtomic(path, out, 0o600)
}

// Provider is one entry in auth.json (the credential store). Values are never
// read or surfaced — only the id and credential type.
type Provider struct {
	ID   string
	Type string
}

func ListProviders(authPath string) ([]Provider, error) {
	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ps []Provider
	gjson.ParseBytes(data).ForEach(func(k, v gjson.Result) bool {
		ps = append(ps, Provider{ID: k.String(), Type: v.Get("type").String()})
		return true
	})
	return ps, nil
}

// escapeKey escapes the gjson/sjson path separator so server names containing a
// dot are addressed as a single key.
func escapeKey(k string) string {
	return strings.ReplaceAll(k, ".", `\.`)
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
