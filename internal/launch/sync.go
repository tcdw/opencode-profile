package launch

import (
	"encoding/json"
	"os"
)

// runSyncs merges each Src JSON file into its Dst counterpart. Missing keys
// in Dst that exist in Src are added; existing Dst keys are never overwritten.
// Errors are silently ignored — sync is best-effort and must never block exit.
func runSyncs(pairs []SyncPair) {
	for _, p := range pairs {
		mergeJSON(p.Src, p.Dst)
	}
}

// mergeJSON reads src and dst as flat JSON objects, copies any keys from src
// that are absent in dst, and writes the merged result back to dst.
func mergeJSON(src, dst string) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return
	}
	var srcMap map[string]json.RawMessage
	if err := json.Unmarshal(srcData, &srcMap); err != nil {
		return
	}

	dstData, err := os.ReadFile(dst)
	if err != nil {
		return
	}
	var dstMap map[string]json.RawMessage
	if err := json.Unmarshal(dstData, &dstMap); err != nil {
		return
	}

	changed := false
	for k, v := range srcMap {
		if _, exists := dstMap[k]; !exists {
			dstMap[k] = v
			changed = true
		}
	}
	if !changed {
		return
	}

	out, err := json.MarshalIndent(dstMap, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')
	_ = os.WriteFile(dst, out, 0o600)
}
