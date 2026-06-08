// Package tui is the interactive profile picker/manager. The root model holds a
// screen enum plus sub-models; only the active screen handles input. Sub-models
// stay decoupled by emitting messages (navMsg / editPromptMsg) the root acts on.
// Launch is signalled by setting launch and quitting — the exec happens in main.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tcdw/opencode-profile/internal/launch"
	"github.com/tcdw/opencode-profile/internal/paths"
	"github.com/tcdw/opencode-profile/internal/store"
)

type screen int

const (
	screenList screen = iota
	screenCreate
	screenConfirm
	screenDetail
	screenMCP
	screenProviders
	screenDomains
)

type launchReq struct{ name string }

type rootModel struct {
	layout paths.Layout
	store  *store.Store
	screen screen
	width  int
	height int

	list      list.Model
	create    createModel
	detail    detailModel
	mcp       mcpModel
	providers providersModel
	domains   domainsModel
	confirm   string // name pending deletion

	launch *launchReq
	status string
}

// Run shows the picker and, if the user chose to launch, returns the Plan to
// exec. A nil Plan means the user quit without launching.
func Run(l paths.Layout, s *store.Store) (*launch.Plan, error) {
	p := tea.NewProgram(newRoot(l, s), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	rm := final.(rootModel)
	if rm.launch == nil {
		return nil, nil
	}
	return launch.BuildPlan(l, rm.launch.name, nil)
}

func newRoot(l paths.Layout, s *store.Store) rootModel {
	li := list.New(buildItems(l, s), list.NewDefaultDelegate(), 0, 0)
	li.Title = "opencode profiles"
	li.SetShowStatusBar(false)
	li.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Launch, keys.New, keys.Edit, keys.Delete}
	}
	li.AdditionalFullHelpKeys = li.AdditionalShortHelpKeys
	return rootModel{layout: l, store: s, list: li}
}

func (m rootModel) Init() tea.Cmd { return nil }

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, max(1, msg.Height-2))
		return m, nil
	case navMsg:
		return m.navigate(msg), nil
	case editPromptMsg:
		return m, m.openEditor(msg.path)
	case editorFinishedMsg:
		if msg.err != nil {
			m.status = "editor error: " + msg.err.Error()
		}
		return m, nil
	}

	switch m.screen {
	case screenCreate:
		return m.updateCreate(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenDetail:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	case screenMCP:
		var cmd tea.Cmd
		m.mcp, cmd = m.mcp.Update(msg)
		return m, cmd
	case screenProviders:
		var cmd tea.Cmd
		m.providers, cmd = m.providers.Update(msg)
		return m, cmd
	case screenDomains:
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, cmd
	default:
		return m.updateList(msg)
	}
}

// navigate switches screens, rebuilding the target sub-model so its data is
// freshly read from disk.
func (m rootModel) navigate(msg navMsg) rootModel {
	switch msg.to {
	case screenDetail:
		m.detail = newDetail(m.layout, msg.name)
		m.screen = screenDetail
	case screenMCP:
		m.mcp = newMCP(m.layout, msg.name)
		m.screen = screenMCP
	case screenProviders:
		m.providers = newProviders(m.layout, msg.name)
		m.screen = screenProviders
	case screenDomains:
		m.domains = newDomains(m.store, msg.name)
		m.screen = screenDomains
	default:
		m.refreshList()
		m.screen = screenList
	}
	return m
}

func (m rootModel) openEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	// EDITOR may carry flags (e.g. "code -w"); GUI editors need their wait flag
	// or ExecProcess returns immediately — we don't try to fix that here.
	fields := strings.Fields(editor)
	args := append(fields[1:], path)
	c := exec.Command(fields[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err: err} })
}

func (m rootModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Launch):
			if it, ok := m.list.SelectedItem().(profileItem); ok {
				m.launch = &launchReq{name: it.name}
				return m, tea.Quit
			}
		case key.Matches(km, keys.New):
			m.create = newCreateModel()
			m.status = ""
			m.screen = screenCreate
			return m, m.create.Init()
		case key.Matches(km, keys.Edit):
			if it, ok := m.list.SelectedItem().(profileItem); ok {
				if it.isDefault {
					m.status = "the default profile edits your live config; open it in opencode"
					return m, nil
				}
				m.detail = newDetail(m.layout, it.name)
				m.screen = screenDetail
			}
			return m, nil
		case key.Matches(km, keys.Delete):
			if it, ok := m.list.SelectedItem().(profileItem); ok {
				if it.isDefault {
					m.status = "the built-in default profile can't be deleted"
					return m, nil
				}
				m.confirm = it.name
				m.screen = screenConfirm
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m rootModel) updateCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.create, cmd = m.create.Update(msg)

	switch {
	case m.create.cancelled:
		m.screen = screenList
		return m, nil
	case m.create.submitted:
		name := m.create.nameInput.Value()
		desc := m.create.descInput.Value()
		if _, err := m.store.Create(name, store.CreateOpts{Description: desc, Blank: m.create.blank}); err != nil {
			m.create.err = err
			m.create.submitted = false
			return m, nil
		}
		m.refreshList()
		m.status = fmt.Sprintf("created profile %q", name)
		if !m.create.blank && nonEmptyFile(m.layout.LiveAgentsMD()) {
			m.status += "; copied non-empty live AGENTS.md"
		}
		m.screen = screenList
		return m, nil
	}
	return m, cmd
}

func (m rootModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y", "enter":
			if err := m.store.Remove(m.confirm); err != nil {
				m.status = "delete failed: " + err.Error()
			} else {
				m.status = fmt.Sprintf("deleted profile %q", m.confirm)
				m.refreshList()
			}
			m.screen = screenList
		case "n", "N", "esc":
			m.screen = screenList
		}
	}
	return m, nil
}

func (m *rootModel) refreshList() {
	m.list.SetItems(buildItems(m.layout, m.store))
}

func (m rootModel) View() string {
	switch m.screen {
	case screenCreate:
		return m.create.View()
	case screenConfirm:
		return confirmView(m.confirm)
	case screenDetail:
		return m.detail.View()
	case screenMCP:
		return m.mcp.View()
	case screenProviders:
		return m.providers.View()
	case screenDomains:
		return m.domains.View()
	default:
		v := m.list.View()
		if m.status != "" {
			v += "\n" + statusStyle.Render(m.status)
		}
		return v
	}
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
