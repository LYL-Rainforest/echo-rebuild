package shared

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgressBar struct {
	Title     string
	Total     int64
	Done      int64
	StartTime time.Time
	Message   string
	Width     int
}

func NewProgressBar(title string, total int64) ProgressBar {
	return ProgressBar{Title: title, Total: total, StartTime: time.Now(), Width: 40}
}

func (m ProgressBar) Update(msg tea.Msg) (ProgressBar, tea.Cmd) { return m, nil }

func (m ProgressBar) View() string {
	var b strings.Builder
	b.WriteString("  " + m.Title + "\n\n")

	if m.Total > 0 {
		pct := float64(m.Done) / float64(m.Total)
		filled := int(pct * float64(m.Width))
		if filled > m.Width { filled = m.Width }
		bar := strings.Repeat("█", filled) + strings.Repeat("░", m.Width-filled)
		b.WriteString(fmt.Sprintf("  %s  %d%%\n", bar, int(pct*100)))

		elapsed := time.Since(m.StartTime)
		speed := float64(0)
		if elapsed.Seconds() > 0 {
			speed = float64(m.Done) / elapsed.Seconds()
		}
		b.WriteString(fmt.Sprintf("  已处理: %s / %s\n", byteStr(m.Done), byteStr(m.Total)))
		b.WriteString(fmt.Sprintf("  速度: %s/s\n", byteStr(int64(speed))))
		b.WriteString(fmt.Sprintf("  用时: %s\n", elapsed.Round(time.Second)))
		if speed > 0 {
			remaining := time.Duration(float64(m.Total-m.Done)/speed) * time.Second
			b.WriteString(fmt.Sprintf("  预计剩余: %s\n", remaining.Round(time.Second)))
		}
	}

	if m.Message != "" {
		b.WriteString(fmt.Sprintf("\n  当前: %s\n", m.Message))
	}

	return b.String()
}

func byteStr(b int64) string {
	const unit = 1024
	if b < unit { return fmt.Sprintf("%d B", b) }
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
