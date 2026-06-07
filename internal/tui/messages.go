package tui

// navMsg asks the root to switch screens and (re)build the target sub-model so
// its data reloads from disk. Sub-models emit it on esc / drill-in.
type navMsg struct {
	to   screen
	name string
}

// editPromptMsg asks the root to open $EDITOR on a profile's AGENTS.md.
type editPromptMsg struct{ path string }

// editorFinishedMsg is delivered after the external editor exits.
type editorFinishedMsg struct{ err error }

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
