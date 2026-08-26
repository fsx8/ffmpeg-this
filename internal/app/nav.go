package app

import tea "github.com/charmbracelet/bubbletea"

type pushMsg struct{ m tea.Model }
type popMsg struct{}
type replaceMsg struct{ m tea.Model }

func push(m tea.Model) tea.Cmd {
	return func() tea.Msg { return pushMsg{m: m} }
}

func pop() tea.Cmd {
	return func() tea.Msg { return popMsg{} }
}

func replace(m tea.Model) tea.Cmd {
	return func() tea.Msg { return replaceMsg{m: m} }
}
