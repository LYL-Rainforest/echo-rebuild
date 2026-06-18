package pages

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Result struct {
	Success  bool
	Message  string
	Detail   string
}

func NewResult(success bool, msg, detail string) Result {
	return Result{Success: success, Message: msg, Detail: detail}
}

func (m Result) Init() tea.Cmd { return nil }

func (m Result) Update(msg tea.Msg) (Result, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", " ":
			return m, nil
		}
	}
	return m, nil
}

func (m Result) View() string {
	var b strings.Builder
	if m.Success {
		b.WriteString(titleStyle.Render("操作完成") + "\n\n")
		b.WriteString(successStyle.Render("  ✓ " + m.Message) + "\n")
	} else {
		b.WriteString(titleStyle.Render("操作失败") + "\n\n")
		b.WriteString(errorStyle.Render("  ✗ " + m.Message) + "\n")
	}
	if m.Detail != "" {
		b.WriteString("\n" + m.Detail + "\n")
	}
	b.WriteString(helpStyle.Render("\n  Enter 返回") + "\n")
	return b.String()
}
