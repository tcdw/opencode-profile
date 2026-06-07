package ocfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `{
  "$schema": "https://opencode.ai/config.json",
  "model": "prov/old-model",
  "mcp": {
    "figma": { "type": "remote", "enabled": true },
    "chrome": { "type": "local" }
  },
  "keep": "untouched"
}
`

func writeSample(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(p, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSetGetModel(t *testing.T) {
	p := writeSample(t)
	if got := GetModel(p); got != "prov/old-model" {
		t.Fatalf("GetModel = %q", got)
	}
	if err := SetModel(p, "prov/new"); err != nil {
		t.Fatal(err)
	}
	if got := GetModel(p); got != "prov/new" {
		t.Fatalf("after SetModel, GetModel = %q", got)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"keep": "untouched"`) {
		t.Error("unrelated key was disturbed by SetModel")
	}
}

func TestListAndToggleMCP(t *testing.T) {
	p := writeSample(t)
	entries, err := ListMCP(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 MCP entries, got %d", len(entries))
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name] = e.Enabled
	}
	if !got["chrome"] {
		t.Error("a server with no enabled field should default to enabled")
	}
	if err := SetMCPEnabled(p, "chrome", false); err != nil {
		t.Fatal(err)
	}
	entries, _ = ListMCP(p)
	for _, e := range entries {
		if e.Name == "chrome" && e.Enabled {
			t.Error("chrome still enabled after toggle")
		}
		if e.Name == "figma" && !e.Enabled {
			t.Error("figma should be unaffected by toggling chrome")
		}
	}
}

func TestListProviders(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "auth.json")
	os.WriteFile(p, []byte(`{"openai":{"type":"oauth"},"deepseek":{"type":"api"}}`), 0o600)
	ps, err := ListProviders(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("want 2 providers, got %d", len(ps))
	}
	ps, err = ListProviders(filepath.Join(dir, "missing.json"))
	if err != nil || len(ps) != 0 {
		t.Errorf("missing auth file should yield (nil, nil); got ps=%v err=%v", ps, err)
	}
}
