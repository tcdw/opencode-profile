package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"

	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

// profileItem adapts a profile (or the synthetic default) to bubbles/list.
type profileItem struct {
	name        string
	model       string
	description string
	badges      string
	isDefault   bool
}

func (i profileItem) Title() string {
	if i.isDefault {
		return i.name + "  (built-in)"
	}
	return i.name
}

func (i profileItem) Description() string {
	parts := make([]string, 0, 3)
	if i.model != "" && i.model != "-" {
		parts = append(parts, i.model)
	}
	if i.badges != "" {
		parts = append(parts, i.badges)
	}
	if i.description != "" {
		parts = append(parts, i.description)
	}
	return strings.Join(parts, " · ")
}

func (i profileItem) FilterValue() string { return i.name }

// buildItems returns the synthetic default profile followed by stored profiles.
func buildItems(l paths.Layout, s *store.Store) []list.Item {
	items := []list.Item{
		profileItem{
			name:        store.ReservedDefault,
			model:       "live config",
			description: "current ~/.config/opencode",
			isDefault:   true,
		},
	}
	for _, p := range s.Profiles {
		items = append(items, profileItem{
			name:        p.Name,
			model:       store.ReadModel(l, p.Name),
			description: p.Description,
			badges:      p.DomainBadges(),
		})
	}
	return items
}
