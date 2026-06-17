package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

type page int

const (
	pageMainMenu page = iota
	pageConfigBackup
	pageConfigRestore
	pageImageBackup
	pageImageRestore
)

type MainModel struct {
	page    page
	menu    MainMenuModel
	configB *ConfigBackupModel
	configR *ConfigRestoreModel
	imageB  *ImageBackupModel
	imageR  *ImageRestoreModel
	width   int
	height  int
}

func NewMainModel() MainModel {
	return MainModel{
		page: pageMainMenu,
		menu: NewMainMenuModel(),
	}
}

func (m MainModel) Init() tea.Cmd {
	return nil
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case menuChoice:
		switch msg {
		case menuImageBackup:
			ib := NewImageBackupModel(m.width)
			m.imageB = &ib
			m.page = pageImageBackup
			return m, m.imageB.Init()

		case menuImageRestore:
			ir := NewImageRestoreModel(m.width)
			m.imageR = &ir
			m.page = pageImageRestore
			return m, nil
		case menuConfigBackup:
			sc := newScanner()
			cb := NewConfigBackupModel(sc, m.width)
			m.configB = &cb
			m.page = pageConfigBackup
			return m, m.configB.Init()
		case menuConfigRestore:
			cr := NewConfigRestoreModel(m.width)
			m.configR = &cr
			m.page = pageConfigRestore
			return m, nil
		case menuExit:
			return m, tea.Quit
		default:
			m.page = pageMainMenu
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
	case pageMainMenu:
		mm, c := m.menu.Update(msg)
		m.menu = mm
		cmd = c
	case pageConfigBackup:
		if m.configB != nil {
			cb, c := m.configB.Update(msg)
			m.configB = &cb
			cmd = c
		}
	case pageConfigRestore:
		if m.configR != nil {
			cr, c := m.configR.Update(msg)
			m.configR = &cr
			cmd = c
		}
	case pageImageBackup:
		if m.imageB != nil {
			ib, c := m.imageB.Update(msg)
			m.imageB = &ib
			cmd = c
		}
	case pageImageRestore:
		if m.imageR != nil {
			ir, c := m.imageR.Update(msg)
			m.imageR = &ir
			cmd = c
		}
	}
	return m, cmd
}

func (m MainModel) View() string {
	switch m.page {
	case pageMainMenu:
		return m.menu.View()
	case pageConfigBackup:
		if m.configB != nil {
			return m.configB.View()
		}
	case pageConfigRestore:
		if m.configR != nil {
			return m.configR.View()
		}
	case pageImageBackup:
		if m.imageB != nil {
			return m.imageB.View()
		}
	case pageImageRestore:
		if m.imageR != nil {
			return m.imageR.View()
		}
	}
	return ""
}
