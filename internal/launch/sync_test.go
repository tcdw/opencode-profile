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

func TestRunSyncsDoesNotOverwriteProfileCredentials(t *testing.T) {
	tmp := t.TempDir()
	live := filepath.Join(tmp, "live", "auth.json")
	profile := filepath.Join(tmp, "profile", "auth.json")
	mustWriteJSON(t, live, `{"openai":{"token":"live"},"google":{"token":"live-google"}}`)
	mustWriteJSON(t, profile, `{"openai":{"token":"profile"}}`)

	runSyncs([]SyncPair{{Src: live, Dst: profile}})

	got := readProviders(t, profile)
	if got["openai"]["token"] != "profile" {
		t.Fatalf("openai token = %q, want profile", got["openai"]["token"])
	}
	if got["google"]["token"] != "live-google" {
		t.Fatalf("google token = %q, want live-google", got["google"]["token"])
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
