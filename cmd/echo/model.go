package main

import (
	msgpkg "echo-rebuild/cmd/echo/msg"
	"echo-rebuild/cmd/echo/pages"
	"echo-rebuild/internal/scanner"
	tea "github.com/charmbracelet/bubbletea"
)

type page int

const (
	pgMainMenu page = iota
	pgConfigBackup
	pgConfigRestore
)

type MainModel struct {
	page    page
	menu    pages.MainMenu
	configB *pages.ConfigBackup
	configR *pages.ConfigRestore
}

func NewMainModel() MainModel {
	return MainModel{page: pgMainMenu, menu: pages.NewMainMenu()}
}

func (m MainModel) Init() tea.Cmd { return nil }

func (m MainModel) Update(tmsg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := tmsg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case msgpkg.MenuChoice:
		switch msg {
		case msgpkg.MenuConfigBackup:
			cb := pages.NewConfigBackup(scanner.New())
			m.configB = &cb
			m.page = pgConfigBackup
			return m, m.configB.Init()
		case msgpkg.MenuConfigRestore:
			cr := pages.NewConfigRestore()
			m.configR = &cr
			m.page = pgConfigRestore
			return m, m.configR.Init()
		case msgpkg.MenuExit:
			return m, tea.Quit
		default: // 0 = back to menu
			m.page = pgMainMenu
			return m, nil
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	switch m.page {
	case pgMainMenu:
		mm, c := m.menu.Update(tmsg)
		m.menu = mm
		cmd = c
	case pgConfigBackup:
		if m.configB != nil {
			cb, c := m.configB.Update(tmsg)
			m.configB = &cb
			cmd = c
		}
	case pgConfigRestore:
		if m.configR != nil {
			cr, c := m.configR.Update(tmsg)
			m.configR = &cr
			cmd = c
		}
	}
	return m, cmd
}

func (m MainModel) View() string {
	switch m.page {
	case pgMainMenu:
		return m.menu.View()
	case pgConfigBackup:
		if m.configB != nil {
			return m.configB.View()
		}
	case pgConfigRestore:
		if m.configR != nil {
			return m.configR.View()
		}
	}
	return ""
}
