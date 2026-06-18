package shared

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Input struct {
	ti      textinput.Model
	prompt  string
}

func NewInput(placeholder, prompt string, width int) Input {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Width = width
	ti.Focus()
	return Input{ti: ti, prompt: prompt}
}

func (i Input) SetValue(v string) { i.ti.SetValue(v) }
func (i Input) Value() string     { return i.ti.Value() }

func (i Input) Init() tea.Cmd { return textinput.Blink }

func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	var cmd tea.Cmd
	i.ti, cmd = i.ti.Update(msg)
	return i, cmd
}

func (i Input) View() string {
	return i.prompt + i.ti.View()
}
