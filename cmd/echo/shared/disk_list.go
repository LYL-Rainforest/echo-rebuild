package shared

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type DeviceItem struct {
	Path      string
	Model     string
	SizeBytes int64
}

type DiskList struct {
	Devices []DeviceItem
	Cursor  int
}

func NewDiskList(devices []DeviceItem) DiskList {
	return DiskList{Devices: devices}
}

func (m *DiskList) MoveCursor(delta int) {
	n := len(m.Devices)
	if n == 0 { return }
	m.Cursor = (m.Cursor + delta + n) % n
}

func (m DiskList) Update(msg tea.Msg) (DiskList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.MoveCursor(-1)
		case "down", "j":
			m.MoveCursor(1)
		}
	}
	return m, nil
}

func (m DiskList) View() string {
	var b strings.Builder
	for i, d := range m.Devices {
		cur := " "
		if i == m.Cursor { cur = ">" }
		b.WriteString(fmt.Sprintf("  %s %s\n", cur, d.Model))
	}
	return b.String()
}
