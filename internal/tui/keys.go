package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Launch key.Binding
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Quit   key.Binding
	Back   key.Binding
}

var keys = keyMap{
	Launch: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "launch")),
	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}
