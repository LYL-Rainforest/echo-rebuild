package pages

import (
	"strings"

	msgpkg "echo-rebuild/cmd/echo/msg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#36C")).Padding(0, 2)
	selectedMenuStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#47F")).Padding(0, 2)
	menuStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#DDD")).Padding(0, 2)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Padding(0, 2)
	boxStyle          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#47F")).Padding(0, 1)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#0C0"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#C00"))
	dimmedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))
)

type MainMenu struct {
	choices []string
	cursor  int
}

func NewMainMenu() MainMenu {
	return MainMenu{
		choices: []string{
			"1.  创建系统配置",
			"2.  还原系统配置",
			"0.  退出",
		},
	}
}

func (mnu MainMenu) Init() tea.Cmd { return nil }

func (mnu MainMenu) Update(tmsg tea.Msg) (MainMenu, tea.Cmd) {
	switch msg := tmsg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			mnu.cursor = (mnu.cursor - 1 + len(mnu.choices)) % len(mnu.choices)
		case "down", "j":
			mnu.cursor = (mnu.cursor + 1) % len(mnu.choices)
		case "enter", " ":
			switch mnu.cursor {
			case 0:
				return mnu, func() tea.Msg { return msgpkg.MenuConfigBackup }
			case 1:
				return mnu, func() tea.Msg { return msgpkg.MenuConfigRestore }
			case 2:
				return mnu, tea.Quit
			}
		case "1":
			return mnu, func() tea.Msg { return msgpkg.MenuConfigBackup }
		case "2":
			return mnu, func() tea.Msg { return msgpkg.MenuConfigRestore }
		case "0", "q", "ctrl+c":
			return mnu, tea.Quit
		}
	}
	return mnu, nil
}

func (mnu MainMenu) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EchoRebuild — 系统配置备份与还原工具"))
	b.WriteString("\n\n")
	for i, c := range mnu.choices {
		cur := "  "
		if i == mnu.cursor {
			cur = "> "
			b.WriteString(selectedMenuStyle.Render(cur+c) + "\n")
		} else {
			b.WriteString(menuStyle.Render(cur+c) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("  ↑↓ 移动  Enter 选择  q 退出") + "\n")
	return b.String()
}
