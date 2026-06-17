package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgressModel struct {
	Title     string
	Total     int
	Done      int
	StartTime time.Time
	Message   string
	DoneMsg   string
	Failed    bool
	ErrMsg    string
	Finished  bool
	Width     int
}

func NewProgressModel(title string, total int) ProgressModel {
	return ProgressModel{
		Title:     title,
		Total:     total,
		StartTime: time.Now(),
	}
}

func (m ProgressModel) Update(msg tea.Msg) (ProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Finished {
			switch msg.String() {
			case "enter", "esc", " ":
				return m, nil
			}
		}
	}
	return m, nil
}

func (m ProgressModel) View() string {
	var b strings.Builder

	b.WriteString(boxStyle.Render(m.Title) + "\n\n")

	if m.Total > 0 {
		barWidth := m.Width - 4
		if barWidth < 10 {
			barWidth = 10
		}
		pct := float64(m.Done) / float64(m.Total)
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		b.WriteString(fmt.Sprintf("  [%s]  %d/%d\n", bar, m.Done, m.Total))
	}

	elapsed := time.Since(m.StartTime).Round(time.Second)
	b.WriteString(fmt.Sprintf("  用时: %s\n", elapsed))

	if m.Message != "" {
		b.WriteString(fmt.Sprintf("  %s\n", m.Message))
	}

	if m.Finished {
		b.WriteString("\n")
		if m.Failed {
			b.WriteString(errorStyle.Render("  ✗ " + m.ErrMsg) + "\n")
		} else {
			b.WriteString(successStyle.Render("  ✓ "+m.DoneMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}

	return b.String()
}
