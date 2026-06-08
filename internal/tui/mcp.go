package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tcdw/opencode-profile/internal/ocfg"
	"github.com/tcdw/opencode-profile/internal/paths"
)

// mcpModel is a checkbox list over the profile's MCP servers; space toggles
// enabled and writes through immediately.
type mcpModel struct {
	name    string
	path    string
	entries []ocfg.MCPEntry
	cursor  int
	status  string
	err     error
}

func newMCP(l paths.Layout, name string) mcpModel {
	p := l.OpencodeConfig(name)
	e, err := ocfg.ListMCP(p)
	return mcpModel{name: name, path: p, entries: e, err: err}
}

func (m mcpModel) Update(msg tea.Msg) (mcpModel, tea.Cmd) {
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
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case " ", "enter":
		if len(m.entries) == 0 {
			return m, nil
		}
		e := &m.entries[m.cursor]
		next := !e.Enabled
		if err := ocfg.SetMCPEnabled(m.path, e.Name, next); err != nil {
			m.status = "toggle failed: " + err.Error()
		} else {
			e.Enabled = next
			state := "disabled"
			if next {
				state = "enabled"
			}
			m.status = fmt.Sprintf("%s %s", e.Name, state)
		}
	}
	return m, nil
}

func (m mcpModel) View() string {
	b := titleStyle.Render("MCP servers — "+m.name) + "\n\n"
	if m.err != nil {
		return appStyle.Render(b + errStyle.Render("config unreadable: "+m.err.Error()))
	}
	if len(m.entries) == 0 {
		b += hintStyle.Render("(no MCP servers in this profile's config)") + "\n"
	}
	for i, e := range m.entries {
		box := "[ ]"
		if e.Enabled {
			box = "[x]"
		}
		line := fmt.Sprintf("%s %s (%s)", box, e.Name, orDash(e.Type))
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
