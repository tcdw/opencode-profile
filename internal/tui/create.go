package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// createModel is the new-profile form: name, description, and a seed toggle.
type createModel struct {
	nameInput textinput.Model
	descInput textinput.Model
	blank     bool
	focus     int // 0=name 1=desc 2=blank toggle
	submitted bool
	cancelled bool
	err       error
}

func newCreateModel() createModel {
	n := textinput.New()
	n.Placeholder = "profile name (letters, digits, _.-)"
	n.CharLimit = 64
	n.Focus()
	d := textinput.New()
	d.Placeholder = "description (optional)"
	d.CharLimit = 120
	return createModel{nameInput: n, descInput: d}
}

func (m createModel) Init() tea.Cmd { return textinput.Blink }

func (m createModel) Update(msg tea.Msg) (createModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.cancelled = true
			return m, nil
		case "enter":
			m.submitted = true
			return m, nil
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focus = (m.focus + 2) % 3
			m.syncFocus()
			return m, nil
		case " ":
			if m.focus == 2 {
				m.blank = !m.blank
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.descInput, cmd = m.descInput.Update(msg)
	}
	return m, cmd
}

func (m *createModel) syncFocus() {
	if m.focus == 0 {
		m.nameInput.Focus()
	} else {
		m.nameInput.Blur()
	}
	if m.focus == 1 {
		m.descInput.Focus()
	} else {
		m.descInput.Blur()
	}
}

func (m createModel) View() string {
	seed := "[ ] seed from current config (copies opencode.json[c] + AGENTS.md)"
	if m.blank {
		seed = "[x] seed BLANK (minimal config + empty AGENTS.md)"
	}
	if m.focus == 2 {
		seed = focusStyle.Render(seed)
	}
	body := titleStyle.Render("New profile") + "\n\n" +
		"name:  " + m.nameInput.View() + "\n" +
		"desc:  " + m.descInput.View() + "\n\n" +
		seed + "\n\n" +
		hintStyle.Render("[enter] create   [tab] next field   [space] toggle blank   [esc] cancel")
	if m.err != nil {
		body += "\n\n" + errStyle.Render("error: "+m.err.Error())
	}
	return appStyle.Render(body)
}

func confirmView(name string) string {
	body := titleStyle.Render(fmt.Sprintf("Delete profile %q?", name)) + "\n\n" +
		"Removes its config, system prompt, and session DB.\n" +
		hintStyle.Render("Shared API keys / skills are NOT affected.") + "\n\n" +
		hintStyle.Render("[y] yes    [n] no")
	return appStyle.Render(body)
}
