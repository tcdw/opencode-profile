package store

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tcdw/opencode-profile/internal/paths"
)

// ReadModel returns a profile's configured model for display, reading only the
// "model" field and ignoring the rest of the (possibly large) config. Returns
// "-" when unset or unreadable.
func ReadModel(l paths.Layout, name string) string {
	data, err := os.ReadFile(l.OpencodeConfig(name))
	if err != nil {
		return "-"
	}
	var c struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &c) != nil || c.Model == "" {
		return "-"
	}
	return c.Model
}

var domainShortName = map[Domain]string{
	DomainAuth:    "auth",
	DomainMCPAuth: "mcp",
	DomainSkills:  "skills",
}

// DomainBadges renders a compact "auth:link mcp:link skills:own" summary.
func (p Profile) DomainBadges() string {
	parts := make([]string, 0, len(AllDomains))
	for _, d := range AllDomains {
		mode := "link"
		if p.Modes[d] == ModeOwned {
			mode = "own"
		}
		parts = append(parts, fmt.Sprintf("%s:%s", domainShortName[d], mode))
	}
	return strings.Join(parts, " ")
}
