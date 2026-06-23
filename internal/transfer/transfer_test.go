package transfer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

func mustMkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRoundTrip exports a store with one all-linked and one owned profile, then
// imports it into a second root and verifies the reconstruction is faithful and
// portable (modes, secret contents/perms, path-ref rewriting, symlink vs copy).
func TestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	liveCfgA := filepath.Join(tmp, "live-config-a")
	liveDataA := filepath.Join(tmp, "live-data-a")
	liveCfgB := filepath.Join(tmp, "live-config-b")
	liveDataB := filepath.Join(tmp, "live-data-b")
	t.Setenv("XDG_CONFIG_HOME", liveCfgA)
	t.Setenv("XDG_DATA_HOME", liveDataA)
	mustMkdirAll(t, filepath.Join(liveCfgA, "opencode"))
	mustMkdirAll(t, filepath.Join(liveDataA, "opencode"))

	rootA := filepath.Join(tmp, "store-a")
	rootB := filepath.Join(tmp, "store-b")
	absA, _ := filepath.Abs(rootA)
	absB, _ := filepath.Abs(rootB)

	// Live opencode.json carries an absolute {file:} ref into store-a's key —
	// import must rewrite this to store-b's root.
	ref := "{file:" + absA + "/shared/rightcapital.key}"
	liveJSON := `{"model":"prov/m","provider":{"x":{"options":{"apiKey":"` + ref +
		`"}}},"mcp":{"figma":{"type":"remote","headers":{"Authorization":"Bearer ` + ref + `"}}}}` + "\n"
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "opencode.json"), liveJSON)
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "AGENTS.md"), "jirai prompt\n")
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "tui.json"), `{"theme":"dark"}`)
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "commands", "hello.json"), `{"cmd":"hi"}`)
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "plugins", "local.ts"), "export default async () => ({})\n")
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "plugins", "node_modules", "dep", "index.js"), "module.exports = {}\n")
	mustWrite(t, filepath.Join(liveCfgA, "opencode", "opencode.json.bak-1"), `{"old":true}`)
	mustWrite(t, filepath.Join(liveDataA, "opencode", "auth.json"), `{"openai":{"type":"oauth"}}`)
	mustWrite(t, filepath.Join(liveDataA, "opencode", "mcp-auth.json"), `{"notion":{"token":"x"}}`)
	mustWrite(t, filepath.Join(liveDataA, "opencode", "opencode.db"), "db")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "opencode.db-wal"), "wal")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "extensions", "foo.json"), `{"enabled":true}`)
	mustWrite(t, filepath.Join(liveDataA, "opencode", "snapshot", "project", "blob"), "snapshot")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "storage", "message", "msg.json"), `{"large":true}`)
	mustWrite(t, filepath.Join(liveDataA, "opencode", "tool-output", "tool_1"), "output")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "log", "app.log"), "log")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "bin", "tool"), "bin")
	mustWrite(t, filepath.Join(liveDataA, "opencode", "auth.json.bak-1"), `{"old":true}`)

	lA := paths.Layout{Root: rootA}
	sA, err := store.Open(lA)
	if err != nil {
		t.Fatal(err)
	}
	if err := sA.Init(); err != nil {
		t.Fatal(err)
	}

	// A loose shared secret + a shared skill.
	mustWrite(t, filepath.Join(lA.Shared(), "rightcapital.key"), "sk-secret-key-value")
	mustWrite(t, filepath.Join(lA.SharedSkills(), "demo", "SKILL.md"), "# demo skill\n")

	if _, err := sA.Create("alpha", store.CreateOpts{Description: "linked one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := sA.Create("beta", store.CreateOpts{
		Description: "owned one",
		Modes: map[store.Domain]store.DomainMode{
			store.DomainAuth:   store.ModeOwned,
			store.DomainSkills: store.ModeOwned,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// Give beta a distinct owned auth so we can prove owned content travels.
	mustWrite(t, lA.ProfileAuth("beta"), `{"beta":"owned"}`)

	bundle := filepath.Join(tmp, "b.zip")
	now := time.Unix(1_700_000_000, 0)
	if err := Export(lA, ExportOpts{Out: bundle, Passphrase: "pw", Now: now}); err != nil {
		t.Fatal(err)
	}

	lB := paths.Layout{Root: rootB}
	// Point the destination global symlinks at a fresh pair of live dirs.
	t.Setenv("XDG_CONFIG_HOME", liveCfgB)
	t.Setenv("XDG_DATA_HOME", liveDataB)
	if err := Import(lB, ImportOpts{Bundle: bundle, Passphrase: "pw", Now: now}); err != nil {
		t.Fatal(err)
	}

	// Wrong passphrase must be rejected.
	if err := Import(paths.Layout{Root: filepath.Join(tmp, "store-c")},
		ImportOpts{Bundle: bundle, Passphrase: "nope", Now: now}); err == nil {
		t.Error("import with wrong passphrase should fail")
	}

	sB, err := store.Open(lB)
	if err != nil {
		t.Fatal(err)
	}
	a, err := sB.Get("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if a.Modes[store.DomainAuth] != store.ModeLinked {
		if runtime.GOOS == "windows" && a.Modes[store.DomainAuth] == store.ModeOwned {
			t.Log("alpha auth degraded to owned because symlinks are unavailable")
		} else {
			t.Errorf("alpha auth: want linked, got %s", a.Modes[store.DomainAuth])
		}
	}
	b, err := sB.Get("beta")
	if err != nil {
		t.Fatal(err)
	}
	if b.Modes[store.DomainAuth] != store.ModeOwned {
		t.Errorf("beta auth: want owned, got %s", b.Modes[store.DomainAuth])
	}
	if b.Modes[store.DomainSkills] != store.ModeOwned {
		t.Errorf("beta skills: want owned, got %s", b.Modes[store.DomainSkills])
	}

	// Shared secrets present, 0600, content preserved.
	for _, name := range []string{"auth.json", "mcp-auth.json", "rightcapital.key"} {
		fi, err := os.Stat(filepath.Join(lB.Shared(), name))
		if err != nil {
			t.Fatalf("shared %s missing: %v", name, err)
		}
		if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
			t.Errorf("shared %s perm = %o, want 600", name, fi.Mode().Perm())
		}
	}
	if got, _ := os.ReadFile(filepath.Join(lB.Shared(), "rightcapital.key")); string(got) != "sk-secret-key-value" {
		t.Errorf("shared key content = %q", got)
	}

	// opencode.json: ref rewritten from source root to dest root.
	ajson, _ := os.ReadFile(lB.OpencodeJSON("alpha"))
	if bytes.Contains(ajson, []byte(absA)) {
		t.Errorf("alpha opencode.json still references the source root %q", absA)
	}
	if !bytes.Contains(ajson, []byte(filepath.ToSlash(absB))) {
		t.Errorf("alpha opencode.json missing rewritten dest root; got %s", ajson)
	}

	if md, _ := os.ReadFile(lB.AgentsMD("alpha")); string(md) != "jirai prompt\n" {
		t.Errorf("alpha AGENTS.md = %q", md)
	}

	// Linked domain is a symlink; owned domain is a real file with owned data.
	if fi, err := os.Lstat(lB.ProfileAuth("alpha")); err != nil {
		t.Errorf("alpha auth.json missing (err=%v)", err)
	} else if a.Modes[store.DomainAuth] == store.ModeLinked && fi.Mode()&os.ModeSymlink == 0 {
		t.Error("alpha auth.json should be a symlink")
	} else if a.Modes[store.DomainAuth] == store.ModeOwned && fi.Mode()&os.ModeSymlink != 0 {
		t.Error("alpha auth.json fallback should be a real file")
	}
	if fi, err := os.Lstat(lB.ProfileAuth("beta")); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("beta auth.json should be a real file (err=%v)", err)
	}
	if got, _ := os.ReadFile(lB.ProfileAuth("beta")); string(got) != `{"beta":"owned"}` {
		t.Errorf("beta owned auth content = %q", got)
	}

	// Skills: shared and beta-owned both present.
	if _, err := os.Stat(filepath.Join(lB.SharedSkills(), "demo", "SKILL.md")); err != nil {
		t.Errorf("shared skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lB.ProfileSkills("beta"), "demo", "SKILL.md")); err != nil {
		t.Errorf("beta owned skill missing: %v", err)
	}

	// The session DB is never carried.
	if _, err := os.Stat(filepath.Join(lB.ProfileDataOpencode("alpha"), "opencode.db")); !os.IsNotExist(err) {
		t.Error("opencode.db should not be imported")
	}

	// Global config/data travels in plaintext (minus shared-managed entries and DB).
	if got, _ := os.ReadFile(filepath.Join(liveCfgB, "opencode", "tui.json")); string(got) != `{"theme":"dark"}` {
		t.Errorf("global tui.json = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(liveCfgB, "opencode", "commands", "hello.json")); string(got) != `{"cmd":"hi"}` {
		t.Errorf("global commands/hello.json = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(liveCfgB, "opencode", "plugins", "local.ts")); string(got) != "export default async () => ({})\n" {
		t.Errorf("global plugins/local.ts = %q", got)
	}
	if _, err := os.Stat(filepath.Join(liveCfgB, "opencode", "plugins", "node_modules")); !os.IsNotExist(err) {
		t.Error("global plugin node_modules should not be imported")
	}
	if _, err := os.Stat(filepath.Join(liveCfgB, "opencode", "opencode.json.bak-1")); !os.IsNotExist(err) {
		t.Error("global config backups should not be imported")
	}
	if got, _ := os.ReadFile(filepath.Join(liveDataB, "opencode", "extensions", "foo.json")); string(got) != `{"enabled":true}` {
		t.Errorf("global extensions/foo.json = %q", got)
	}

	// Shared-managed files must not be duplicated into the global tree.
	if _, err := os.Stat(filepath.Join(liveDataB, "opencode", "auth.json")); !os.IsNotExist(err) {
		t.Error("global auth.json should not be imported (it lives in shared)")
	}
	if _, err := os.Stat(filepath.Join(liveCfgB, "opencode", "skills")); !os.IsNotExist(err) {
		t.Error("global skills/ should not be imported (it lives in shared)")
	}
	if _, err := os.Stat(filepath.Join(liveDataB, "opencode", "opencode.db")); !os.IsNotExist(err) {
		t.Error("global opencode.db should not be imported")
	}
	for _, rel := range []string{
		"opencode.db-wal",
		"snapshot",
		"storage",
		"tool-output",
		"log",
		"bin",
		"auth.json.bak-1",
	} {
		if _, err := os.Stat(filepath.Join(liveDataB, "opencode", rel)); !os.IsNotExist(err) {
			t.Errorf("global runtime data %s should not be imported", rel)
		}
	}
}

func TestCryptoRoundTrip(t *testing.T) {
	plain := []byte("top secret bytes — 雪乃碗")
	blob, err := Seal(plain, "pw")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(blob, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round trip mismatch: %q", got)
	}
	if _, err := Open(blob, "wrong"); err == nil {
		t.Error("wrong passphrase should fail")
	}
	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := Open(tampered, "pw"); err == nil {
		t.Error("tampered blob should fail authentication")
	}
}

func TestExportFailsWhenProfileConfigMissing(t *testing.T) {
	tmp := t.TempDir()
	liveCfg := filepath.Join(tmp, "live-config")
	liveData := filepath.Join(tmp, "live-data")
	t.Setenv("XDG_CONFIG_HOME", liveCfg)
	t.Setenv("XDG_DATA_HOME", liveData)
	mustMkdirAll(t, filepath.Join(liveCfg, "opencode"))
	mustMkdirAll(t, filepath.Join(liveData, "opencode"))

	l := paths.Layout{Root: filepath.Join(tmp, "store")}
	s, err := store.Open(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("broken", store.CreateOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(l.OpencodeJSON("broken")); err != nil {
		t.Fatal(err)
	}

	err = Export(l, ExportOpts{Out: filepath.Join(tmp, "broken.zip"), Passphrase: "pw"})
	if err == nil {
		t.Fatal("Export should fail when a profile is missing opencode config")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing opencode config")) {
		t.Fatalf("Export error = %v, want missing opencode config", err)
	}
}

func TestExportFailsWhenAgentsMissing(t *testing.T) {
	tmp := t.TempDir()
	liveCfg := filepath.Join(tmp, "live-config")
	liveData := filepath.Join(tmp, "live-data")
	t.Setenv("XDG_CONFIG_HOME", liveCfg)
	t.Setenv("XDG_DATA_HOME", liveData)
	mustMkdirAll(t, filepath.Join(liveCfg, "opencode"))
	mustMkdirAll(t, filepath.Join(liveData, "opencode"))

	l := paths.Layout{Root: filepath.Join(tmp, "store")}
	s, err := store.Open(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("broken", store.CreateOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(l.AgentsMD("broken")); err != nil {
		t.Fatal(err)
	}

	err = Export(l, ExportOpts{Out: filepath.Join(tmp, "broken.zip"), Passphrase: "pw"})
	if err == nil {
		t.Fatal("Export should fail when a profile is missing AGENTS.md")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing AGENTS.md")) {
		t.Fatalf("Export error = %v, want missing AGENTS.md", err)
	}
}

func TestImportFailsWhenBundleProfileConfigMissing(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "bad.zip")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	now := time.Unix(1_700_000_000, 0)
	man := `{
  "schema": 1,
  "tool": "opencode-profile",
  "tool_version": "test",
  "created_at": "2023-11-14T22:13:20Z",
  "source_os": "test",
  "source_root": "",
  "secrets": {"mode": "encrypted"},
  "profiles": [{
    "name": "broken",
    "modes": {"auth": "linked", "mcp_auth": "linked", "skills": "linked"},
    "created_at": "2023-11-14T22:13:20Z"
  }]
}
`
	if err := zipBytes(zw, manifestName, []byte(man), 0o644, now); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	err = Import(paths.Layout{Root: filepath.Join(tmp, "store")}, ImportOpts{Bundle: bundle})
	if err == nil {
		t.Fatal("Import should fail when a bundle profile is missing opencode config")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing opencode.json")) {
		t.Fatalf("Import error = %v, want missing opencode.json or opencode.jsonc", err)
	}
}

func TestImportFailsWhenBundleAgentsMissing(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "bad.zip")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	now := time.Unix(1_700_000_000, 0)
	man := `{
  "schema": 1,
  "tool": "opencode-profile",
  "tool_version": "test",
  "created_at": "2023-11-14T22:13:20Z",
  "source_os": "test",
  "source_root": "",
  "secrets": {"mode": "encrypted"},
  "profiles": [{
    "name": "broken",
    "modes": {"auth": "linked", "mcp_auth": "linked", "skills": "linked"},
    "created_at": "2023-11-14T22:13:20Z"
  }]
}
`
	if err := zipBytes(zw, manifestName, []byte(man), 0o644, now); err != nil {
		t.Fatal(err)
	}
	if err := zipBytes(zw, "profiles/broken/opencode.json", []byte("{}\n"), 0o600, now); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	err = Import(paths.Layout{Root: filepath.Join(tmp, "store")}, ImportOpts{Bundle: bundle})
	if err == nil {
		t.Fatal("Import should fail when a bundle profile is missing AGENTS.md")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing AGENTS.md")) {
		t.Fatalf("Import error = %v, want missing AGENTS.md", err)
	}
}

func TestJSONCConfigRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	liveCfg := filepath.Join(tmp, "live-config")
	liveData := filepath.Join(tmp, "live-data")
	t.Setenv("XDG_CONFIG_HOME", liveCfg)
	t.Setenv("XDG_DATA_HOME", liveData)
	mustWrite(t, filepath.Join(liveCfg, "opencode", "opencode.jsonc"), "{\n  // company provider\n  \"provider\": {\"company\": {}}\n}\n")
	mustMkdirAll(t, filepath.Join(liveData, "opencode"))

	lA := paths.Layout{Root: filepath.Join(tmp, "store-a")}
	sA, err := store.Open(lA)
	if err != nil {
		t.Fatal(err)
	}
	if err := sA.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := sA.Create("company", store.CreateOpts{}); err != nil {
		t.Fatal(err)
	}
	if got := lA.OpencodeConfig("company"); got != lA.OpencodeJSONC("company") {
		t.Fatalf("created config = %q, want %q", got, lA.OpencodeJSONC("company"))
	}

	bundle := filepath.Join(tmp, "jsonc.zip")
	if err := Export(lA, ExportOpts{Out: bundle, Passphrase: "pw"}); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	foundJSONC := false
	for _, f := range zr.File {
		if f.Name == "profiles/company/opencode.jsonc" {
			foundJSONC = true
			break
		}
	}
	zr.Close()
	if !foundJSONC {
		t.Fatal("bundle missing profiles/company/opencode.jsonc")
	}

	lB := paths.Layout{Root: filepath.Join(tmp, "store-b")}
	if err := Import(lB, ImportOpts{Bundle: bundle, Passphrase: "pw"}); err != nil {
		t.Fatal(err)
	}
	if got := lB.OpencodeConfig("company"); got != lB.OpencodeJSONC("company") {
		t.Fatalf("imported config = %q, want %q", got, lB.OpencodeJSONC("company"))
	}
	data, err := os.ReadFile(lB.OpencodeJSONC("company"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("// company provider")) {
		t.Fatalf("imported JSONC lost comments: %s", data)
	}
}
