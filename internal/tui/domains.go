package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tcdw/opencode-profile/internal/store"
)

var domainLabels = map[store.Domain]string{
	store.DomainAuth:    "auth.json (API keys)",
	store.DomainMCPAuth: "mcp-auth.json (MCP OAuth)",
	store.DomainSkills:  "skills/",
}

// domainsModel toggles each shareable domain between linked (shares the base)
// and owned (an isolated copy). Toggling writes through store.SetMode.
type domainsModel struct {
	store  *store.Store
	name   string
	cursor int
	status string
}

func newDomains(s *store.Store, name string) domainsModel {
	return domainsModel{store: s, name: name}
}

func (m domainsModel) modeOf(d store.Domain) store.DomainMode {
	if p, err := m.store.Get(m.name); err == nil {
		return p.Modes[d]
	}
	return store.ModeLinked
}

func (m domainsModel) Update(msg tea.Msg) (domainsModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "q":
		return m, func() tea.Msg { return navMsg{to: screenDetail, name: m.name} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(store.AllDomains)-1 {
			m.cursor++
		}
	case " ", "enter":
		d := store.AllDomains[m.cursor]
		next := store.ModeOwned
		if m.modeOf(d) == store.ModeOwned {
			next = store.ModeLinked
		}
		if err := m.store.SetMode(m.name, d, next); err != nil {
			m.status = "switch failed: " + err.Error()
		} else if next == store.ModeOwned {
			m.status = domainLabels[d] + " → isolated copy"
		} else {
			m.status = domainLabels[d] + " → shared (previous copy backed up)"
		}
	}
	return m, nil
}

func (m domainsModel) View() string {
	b := titleStyle.Render("Domains (share / isolate) — "+m.name) + "\n" +
		hintStyle.Render("linked = shares the base · owned = isolated copy") + "\n\n"
	for i, d := range store.AllDomains {
		mode := "linked (shared)"
		if m.modeOf(d) == store.ModeOwned {
			mode = "owned  (isolated)"
		}
		line := domainLabels[d] + "  —  " + mode
		if i == m.cursor {
			b += focusStyle.Render("> "+line) + "\n"
		} else {
			b += "  " + line + "\n"
		}
	}
	b += "\n" + hintStyle.Render("[space] toggle   [esc] back")
	if m.status != "" {
		b += "\n\n" + statusStyle.Render(m.status)
	}
	return appStyle.Render(b)
}
