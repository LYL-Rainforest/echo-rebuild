package main

import (
	"context"
	"fmt"
	"strings"

	"echo-rebuild/internal/app"
	"echo-rebuild/internal/tbi"
	tea "github.com/charmbracelet/bubbletea"
)

type imageRestoreStep int

const (
	irImage imageRestoreStep = iota
	irDevice
	irConfirm
	irProgress
	irDone
)

type ImageRestoreModel struct {
	step       imageRestoreStep
	imagePath  string
	devices    []tbi.DeviceInfo
	cursor     int
	statusMsg  string
	errMsg     string
	width      int
}

func NewImageRestoreModel(w int) ImageRestoreModel {
	return ImageRestoreModel{
		step:  irImage,
		width: w,
	}
}

func (m ImageRestoreModel) Init() tea.Cmd {
	return nil
}

func (m ImageRestoreModel) Update(msg tea.Msg) (ImageRestoreModel, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesLoadedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("获取设备列表失败: %v", msg.err)
			m.step = irDone
			return m, nil
		}
		m.devices = msg.devices
		return m, nil

	case restoreImageDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("还原失败: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("镜像已还原到: %s", m.devices[m.cursor].Path)
		}
		m.step = irDone
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case irImage:
			switch msg.String() {
			case "enter":
				if m.imagePath != "" {
					m.step = irDevice
					return m, func() tea.Msg {
						mgr := tbi.NewImageManager()
						devices, err := mgr.ListDevices(context.Background())
						if err != nil {
							return devicesLoadedMsg{err: err}
						}
						if devices == nil {
							devices = []tbi.DeviceInfo{}
						}
						return devicesLoadedMsg{devices: devices}
					}
				}
			case "esc":
				return m, func() tea.Msg { return menuChoice(0) }
			case "backspace":
				if len(m.imagePath) > 0 {
					m.imagePath = m.imagePath[:len(m.imagePath)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.imagePath += msg.String()
				}
				return m, nil
			}

		case irDevice:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.devices)-1 {
					m.cursor++
				}
			case "enter":
				if len(m.devices) > 0 {
					m.step = irConfirm
				}
			case "esc":
				m.step = irImage
			}

		case irConfirm:
			switch msg.String() {
			case "1":
				m.step = irProgress
				return m, func() tea.Msg {
					wf, err := app.NewWorkflow("")
					if err != nil {
						return restoreImageDoneMsg{err: err}
					}
					opts := tbi.RestoreOptions{
						TargetDevice: m.devices[m.cursor].Path,
					}
					err = wf.RestoreImage(context.Background(), m.imagePath, m.devices[m.cursor].Path, opts)
					return restoreImageDoneMsg{err: err}
				}
			case "0", "esc":
				m.step = irDevice
			}

		case irDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return m, func() tea.Msg { return menuChoice(0) }
			}
		}
	}
	return m, nil
}

type restoreImageDoneMsg struct {
	err error
}

func (m ImageRestoreModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("还原系统镜像") + "\n\n")

	switch m.step {
	case irImage:
		b.WriteString("输入镜像文件路径:\n")
		b.WriteString(fmt.Sprintf("  > %s\n", m.imagePath))
		b.WriteString(helpStyle.Render("  Enter 确认  Esc 返回") + "\n")

	case irDevice:
		b.WriteString("选择目标分区:\n\n")
		for i, d := range m.devices {
			cur := " "
			if i == m.cursor {
				cur = ">"
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", cur, d.Model))
		}
		b.WriteString(helpStyle.Render("  ↑↓ 选择  Enter 确认  Esc 返回") + "\n")

	case irConfirm:
		b.WriteString("即将还原镜像:\n\n")
		b.WriteString(fmt.Sprintf("  镜像文件: %s\n", m.imagePath))
		b.WriteString(fmt.Sprintf("  目标分区: %s\n", m.devices[m.cursor].Model))
		b.WriteString("\n  1. 开始还原\n")
		b.WriteString("  0. 返回修改\n")

	case irProgress:
		b.WriteString("正在还原镜像，请稍候...\n")

	case irDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + m.errMsg) + "\n")
		} else {
			b.WriteString(successStyle.Render("  ✓ " + m.statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}

	return b.String()
}
