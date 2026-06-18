package pages

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Progress struct {
	Title     string
	Total     int
	Done      int
	StartTime time.Time
	Message   string
}

func NewProgress(title string, total int) Progress {
	return Progress{Title: title, Total: total, StartTime: time.Now()}
}

func (m Progress) Update(msg tea.Msg) (Progress, tea.Cmd) { return m, nil }

func (m Progress) View() string {
	var b strings.Builder
	b.WriteString(boxStyle.Render(m.Title) + "\n\n")
	if m.Total > 0 {
		pct := float64(m.Done) / float64(m.Total)
		w := 30
		filled := int(pct * float64(w))
		if filled > w { filled = w }
		bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
		b.WriteString(fmt.Sprintf("  [%s]  %d/%d\n", bar, m.Done, m.Total))
	}
	elapsed := time.Since(m.StartTime).Round(time.Second)
	b.WriteString(fmt.Sprintf("  用时: %s\n", elapsed))
	if m.Message != "" {
		b.WriteString(fmt.Sprintf("  %s\n", m.Message))
	}
	return b.String()
}
