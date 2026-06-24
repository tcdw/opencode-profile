package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func runStartupSyncs(pairs []SyncPair) {
	for _, p := range pairs {
		mergeJSON(p.Dst, p.Src, true)
	}
}

// runSyncs writes credentials back from the live XDG dir into the profile after
// opencode exits. It overwrites so that tokens opencode refreshed mid-session
// (e.g. an OAuth re-auth that mints a new access token) land in the profile;
// without overwrite, a key the profile already held would be frozen at its old
// value and every refresh would be silently discarded. This is safe because
// runStartupSyncs already pushed the profile's values into live, so any key the
// session did NOT touch still carries the profile's own value here.
// Errors are silently ignored — sync is best-effort and must never block exit.
func runSyncs(pairs []SyncPair) {
	for _, p := range pairs {
		mergeJSON(p.Src, p.Dst, true)
	}
}

// mergeJSON reads src and dst as flat JSON objects, copies any keys from src
// that are absent in dst, and writes the merged result back to dst.
func mergeJSON(src, dst string, overwrite bool) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return
	}
	var srcMap map[string]json.RawMessage
	if err := json.Unmarshal(srcData, &srcMap); err != nil {
		return
	}
	if srcMap == nil {
		srcMap = map[string]json.RawMessage{}
	}

	dstData, err := os.ReadFile(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	var dstMap map[string]json.RawMessage
	if err == nil {
		if err := json.Unmarshal(dstData, &dstMap); err != nil {
			return
		}
	}
	if dstMap == nil {
		dstMap = map[string]json.RawMessage{}
	}

	if !mergeRawMessages(srcMap, dstMap, overwrite) {
		return
	}

	out, err := json.MarshalIndent(dstMap, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(dst, out, 0o600)
}

func mergeRawMessages(srcMap, dstMap map[string]json.RawMessage, overwrite bool) bool {
	changed := false
	for k, v := range srcMap {
		current, exists := dstMap[k]
		if !exists || (overwrite && string(current) != string(v)) {
			dstMap[k] = v
			changed = true
		}
	}
	return changed
}
