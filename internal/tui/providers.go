package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tcdw/opencode-profile/internal/ocfg"
	"github.com/tcdw/opencode-profile/internal/paths"
)

// providersModel is a read-only view of the auth.json this profile uses. It
// shows provider ids and credential types only — never secret values.
type providersModel struct {
	name      string
	providers []ocfg.Provider
	err       error
}

func newProviders(l paths.Layout, name string) providersModel {
	p, err := ocfg.ListProviders(l.ProfileAuth(name))
	return providersModel{name: name, providers: p, err: err}
}

func (m providersModel) Update(msg tea.Msg) (providersModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "q", "enter":
			return m, func() tea.Msg { return navMsg{to: screenDetail, name: m.name} }
		}
	}
	return m, nil
}

func (m providersModel) View() string {
	b := titleStyle.Render("Providers (API keys) — "+m.name) + "\n" +
		hintStyle.Render("read-only; reflects the auth.json this profile uses") + "\n\n"
	if m.err != nil {
		b += errStyle.Render(m.err.Error()) + "\n"
	}
	if len(m.providers) == 0 && m.err == nil {
		b += hintStyle.Render("(no providers logged in)") + "\n"
	}
	for _, p := range m.providers {
		b += fmt.Sprintf("  • %s  (%s)\n", p.ID, orDash(p.Type))
	}
	b += "\n" + hintStyle.Render("[esc] back")
	return appStyle.Render(b)
}
