package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tcdw/opencode-profile/internal/ocfg"
	"github.com/tcdw/opencode-profile/internal/paths"
)

var detailActions = []string{
	"Edit system prompt (AGENTS.md)",
	"Set model",
	"MCP servers",
	"Providers (API keys)",
	"Domains (share / isolate)",
}

// detailModel is the per-profile edit menu. "Set model" edits inline; the other
// actions emit messages the root turns into an editor exec or a screen change.
type detailModel struct {
	layout  paths.Layout
	name    string
	model   string
	cursor  int
	editing bool
	input   textinput.Model
	status  string
}

func newDetail(l paths.Layout, name string) detailModel {
	in := textinput.New()
	in.Placeholder = "provider/model"
	in.CharLimit = 120
	return detailModel{
		layout: l,
		name:   name,
		model:  ocfg.GetModel(l.OpencodeJSON(name)),
		input:  in,
	}
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	if m.editing {
		return m.updateEditing(msg)
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "q":
		return m, func() tea.Msg { return navMsg{to: screenList} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(detailActions)-1 {
			m.cursor++
		}
	case "enter":
		return m.activate()
	}
	return m, nil
}

func (m detailModel) activate() (detailModel, tea.Cmd) {
	switch m.cursor {
	case 0:
		path := m.layout.AgentsMD(m.name)
		return m, func() tea.Msg { return editPromptMsg{path: path} }
	case 1:
		m.editing = true
		m.input.SetValue(m.model)
		m.input.CursorEnd()
		m.input.Focus()
		return m, textinput.Blink
	case 2:
		return m, func() tea.Msg { return navMsg{to: screenMCP, name: m.name} }
	case 3:
		return m, func() tea.Msg { return navMsg{to: screenProviders, name: m.name} }
	case 4:
		return m, func() tea.Msg { return navMsg{to: screenDomains, name: m.name} }
	}
	return m, nil
}

func (m detailModel) updateEditing(msg tea.Msg) (detailModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.editing = false
			m.input.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if err := ocfg.SetModel(m.layout.OpencodeJSON(m.name), val); err != nil {
				m.status = "set model failed: " + err.Error()
			} else {
				m.model = val
				m.status = "model updated"
			}
			m.editing = false
			m.input.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m detailModel) View() string {
	b := titleStyle.Render("profile: "+m.name) + "\n" +
		hintStyle.Render("model: "+orDash(m.model)) + "\n\n"
	for i, a := range detailActions {
		if i == m.cursor {
			b += focusStyle.Render("> "+a) + "\n"
		} else {
			b += "  " + a + "\n"
		}
	}
	if m.editing {
		b += "\nset model: " + m.input.View() + "\n" +
			hintStyle.Render("[enter] save   [esc] cancel")
	} else {
		b += "\n" + hintStyle.Render("[enter] select   [esc] back to list")
	}
	if m.status != "" {
		b += "\n\n" + statusStyle.Render(m.status)
	}
	return appStyle.Render(b)
}
