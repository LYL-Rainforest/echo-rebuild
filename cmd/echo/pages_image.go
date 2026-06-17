package main

import (
	"context"
	"fmt"
	"strings"

	"echo-rebuild/internal/app"
	"echo-rebuild/internal/tbi"
	tea "github.com/charmbracelet/bubbletea"
)

type imageBackupStep int

const (
	ibDevice imageBackupStep = iota
	ibType
	ibCompression
	ibOutput
	ibConfirm
	ibProgress
	ibDone
)

type ImageBackupModel struct {
	step       imageBackupStep
	devices    []tbi.DeviceInfo
	cursor     int
	imgType    tbi.ImageType
	compLevel  int
	outputPath string
	statusMsg  string
	errMsg     string
	width      int
}

func NewImageBackupModel(w int) ImageBackupModel {
	return ImageBackupModel{
		step:      ibDevice,
		width:     w,
		imgType:   tbi.ImageTarZstd,
		compLevel: 6,
	}
}

func (m ImageBackupModel) Init() tea.Cmd {
	return func() tea.Msg {
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

type devicesLoadedMsg struct {
	devices []tbi.DeviceInfo
	err     error
}

type captureDoneMsg struct {
	err error
}

func (m ImageBackupModel) Update(msg tea.Msg) (ImageBackupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case devicesLoadedMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("获取设备列表失败: %v", msg.err)
			m.step = ibDone
			return m, nil
		}
		m.devices = msg.devices
		return m, nil

	case captureDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("备份失败: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("镜像已保存到: %s", m.outputPath)
		}
		m.step = ibDone
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case ibDevice:
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
					m.step = ibType
				}
			case "esc":
				return m, func() tea.Msg { return menuChoice(0) }
			}

		case ibType:
			switch msg.String() {
			case "1":
				m.imgType = tbi.ImageRaw
				m.step = ibCompression
			case "2":
				m.imgType = tbi.ImageTarZstd
				m.step = ibCompression
			case "3":
				m.imgType = tbi.ImageTBI
				m.step = ibCompression
			case "esc":
				m.step = ibDevice
			}

		case ibCompression:
			switch msg.String() {
			case "enter":
				m.step = ibOutput
			case "esc":
				m.step = ibType
			case "backspace":
				if m.compLevel > 0 {
					m.compLevel /= 10
				}
				return m, nil
			default:
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.compLevel = m.compLevel*10 + int(msg.String()[0]-'0')
					if m.compLevel > 19 {
						m.compLevel = 19
					}
				}
				return m, nil
			}

		case ibOutput:
			switch msg.String() {
			case "enter":
				if m.outputPath != "" {
					m.step = ibConfirm
				}
			case "esc":
				m.step = ibCompression
			case "backspace":
				if len(m.outputPath) > 0 {
					m.outputPath = m.outputPath[:len(m.outputPath)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.outputPath += msg.String()
				}
				return m, nil
			}

		case ibConfirm:
			switch msg.String() {
			case "1":
				m.step = ibProgress
				return m, func() tea.Msg {
					wf, err := app.NewWorkflow("")
					if err != nil {
						return captureDoneMsg{err: err}
					}
					source := m.devices[m.cursor].Path
					opts := tbi.CaptureOptions{
						Compression: m.compLevel,
					}
					err = wf.CaptureImage(context.Background(), source, m.outputPath, m.imgType, opts)
					return captureDoneMsg{err: err}
				}
			case "0", "esc":
				m.step = ibOutput
			}

		case ibDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return m, func() tea.Msg { return menuChoice(0) }
			}
		}
	}
	return m, nil
}

func (m ImageBackupModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("创建系统镜像") + "\n\n")

	switch m.step {
	case ibDevice:
		b.WriteString("选择源分区:\n\n")
		for i, d := range m.devices {
			cur := " "
			if i == m.cursor {
				cur = ">"
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", cur, d.Model))
		}
		b.WriteString(helpStyle.Render("  ↑↓ 选择  Enter 确认  Esc 返回") + "\n")

	case ibType:
		b.WriteString("选择镜像类型:\n\n")
		b.WriteString("  1. raw — 原始扇区镜像（全平台）\n")
		b.WriteString("  2. tar.zstd — 文件级压缩（推荐）\n")
		b.WriteString("  3. tbi — True Image 格式（需外部工具）\n")
		b.WriteString(helpStyle.Render("  数字选择  Esc 返回") + "\n")

	case ibCompression:
		b.WriteString(fmt.Sprintf("压缩等级 [0-19] (当前: %d):\n", m.compLevel))
		b.WriteString("  0=不压缩  3=快速  6=推荐  19=最小体积\n")
		b.WriteString(fmt.Sprintf("  > %d\n", m.compLevel))
		b.WriteString(helpStyle.Render("  输入数字  Enter确认  Esc返回") + "\n")

	case ibOutput:
		b.WriteString("输出文件路径:\n")
		b.WriteString(fmt.Sprintf("  > %s\n", m.outputPath))
		b.WriteString(helpStyle.Render("  输入路径  Enter确认  Esc返回") + "\n")

	case ibConfirm:
		source := m.devices[m.cursor].Path
		b.WriteString("即将创建镜像:\n\n")
		b.WriteString(fmt.Sprintf("  源分区: %s\n", m.devices[m.cursor].Model))
		b.WriteString(fmt.Sprintf("  源路径: %s\n", source))
		b.WriteString(fmt.Sprintf("  镜像类型: %s\n", m.imgType))
		b.WriteString(fmt.Sprintf("  压缩等级: %d\n", m.compLevel))
		b.WriteString(fmt.Sprintf("  输出路径: %s\n", m.outputPath))
		b.WriteString("\n  1. 开始备份\n")
		b.WriteString("  0. 返回修改\n")

	case ibProgress:
		b.WriteString("正在创建镜像，请稍候...\n")

	case ibDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + m.errMsg) + "\n")
		} else {
			b.WriteString(successStyle.Render("  ✓ " + m.statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}

	return b.String()
}
