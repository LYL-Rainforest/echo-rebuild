package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuChoice int

const (
	menuImageBackup menuChoice = iota + 1
	menuImageRestore
	menuConfigBackup
	menuConfigRestore
	menuExit
)

type MainMenuModel struct {
	choices []string
	cursor  int
}

func NewMainMenuModel() MainMenuModel {
	return MainMenuModel{
		choices: []string{
			"1.  创建系统镜像",
			"2.  还原系统镜像",
			"3.  创建系统配置",
			"4.  还原系统配置",
			"0.  退出",
		},
	}
}

func (m MainMenuModel) Init() tea.Cmd { return nil }

func (m MainMenuModel) Update(msg tea.Msg) (MainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor - 1 + len(m.choices)) % len(m.choices)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.choices)
		case "enter", " ":
			switch m.cursor {
			case 0:
				return m, func() tea.Msg { return menuChoice(menuImageBackup) }
			case 1:
				return m, func() tea.Msg { return menuChoice(menuImageRestore) }
			case 2:
				return m, func() tea.Msg { return menuChoice(menuConfigBackup) }
			case 3:
				return m, func() tea.Msg { return menuChoice(menuConfigRestore) }
			case 4:
				return m, tea.Quit
			}
		case "1":
			return m, func() tea.Msg { return menuChoice(menuImageBackup) }
		case "2":
			return m, func() tea.Msg { return menuChoice(menuImageRestore) }
		case "3":
			return m, func() tea.Msg { return menuChoice(menuConfigBackup) }
		case "4":
			return m, func() tea.Msg { return menuChoice(menuConfigRestore) }
		case "0":
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m MainMenuModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EchoRebuild — 系统备份与配置还原工具"))
	b.WriteString("\n\n")

	for i, choice := range m.choices {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
			b.WriteString(selectedMenuStyle.Render(cursor + choice) + "\n")
		} else {
			b.WriteString(menuStyle.Render(cursor + choice) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑↓ 移动  Enter 选择  q 退出"))
	b.WriteString("\n")

	return b.String()
}
