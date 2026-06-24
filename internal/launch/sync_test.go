package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunStartupSyncsCopiesProfileCredentialsToLive(t *testing.T) {
	tmp := t.TempDir()
	live := filepath.Join(tmp, "live", "auth.json")
	profile := filepath.Join(tmp, "profile", "auth.json")
	mustWriteJSON(t, live, `{"openai":{"token":"live"},"google":{"token":"live-google"}}`)
	mustWriteJSON(t, profile, `{"openai":{"token":"profile"},"anthropic":{"token":"profile-anthropic"}}`)

	runStartupSyncs([]SyncPair{{Src: live, Dst: profile}})

	got := readProviders(t, live)
	if got["openai"]["token"] != "profile" {
		t.Fatalf("openai token = %q, want profile", got["openai"]["token"])
	}
	if got["anthropic"]["token"] != "profile-anthropic" {
		t.Fatalf("anthropic token = %q, want profile-anthropic", got["anthropic"]["token"])
	}
	if got["google"]["token"] != "live-google" {
		t.Fatalf("google token = %q, want live-google", got["google"]["token"])
	}
}

func TestRunStartupSyncsCreatesMissingLiveAuth(t *testing.T) {
	tmp := t.TempDir()
	live := filepath.Join(tmp, "live", "auth.json")
	profile := filepath.Join(tmp, "profile", "auth.json")
	mustWriteJSON(t, profile, `{"openai":{"token":"profile"}}`)

	runStartupSyncs([]SyncPair{{Src: live, Dst: profile}})

	got := readProviders(t, live)
	if got["openai"]["token"] != "profile" {
		t.Fatalf("openai token = %q, want profile", got["openai"]["token"])
	}
}

// runSyncs runs after opencode exits and must write back tokens the session
// refreshed (e.g. an OAuth re-auth). Because runStartupSyncs already pushed the
// profile's values into live, any key the session left untouched still carries
// the profile's own value, so overwriting is safe — and necessary, or refreshed
// credentials would be silently discarded on every subsequent launch.
func TestRunSyncsWritesBackRefreshedCredentials(t *testing.T) {
	tmp := t.TempDir()
	live := filepath.Join(tmp, "live", "auth.json")
	profile := filepath.Join(tmp, "profile", "auth.json")
	// live = post-session state: notion was re-authed to a fresh token, and the
	// untouched openai key still mirrors what startup pushed up from the profile.
	mustWriteJSON(t, live, `{"openai":{"token":"profile"},"notion":{"token":"refreshed"}}`)
	// profile already held a now-expired notion token plus a key only it knows.
	mustWriteJSON(t, profile, `{"openai":{"token":"profile"},"notion":{"token":"expired"},"glm":{"token":"profile-glm"}}`)

	runSyncs([]SyncPair{{Src: live, Dst: profile}})

	got := readProviders(t, profile)
	if got["notion"]["token"] != "refreshed" {
		t.Fatalf("notion token = %q, want refreshed", got["notion"]["token"])
	}
	if got["openai"]["token"] != "profile" {
		t.Fatalf("openai token = %q, want profile", got["openai"]["token"])
	}
	// A key the profile holds but live never saw must survive the write-back.
	if got["glm"]["token"] != "profile-glm" {
		t.Fatalf("glm token = %q, want profile-glm", got["glm"]["token"])
	}
}

func mustWriteJSON(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readProviders(t *testing.T, path string) map[string]map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
