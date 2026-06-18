package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"echo-rebuild/internal/app"
	"echo-rebuild/internal/engine"
	"echo-rebuild/internal/scanner"
	"echo-rebuild/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type configBackupStep int

const (
	cbScan configBackupStep = iota
	cbTree
	cbSourceType
	cbSearchURL
	cbSourceInput
	cbConfirm
	cbDone
)

type ConfigBackupModel struct {
	step        configBackupStep
	scanner     scanner.Scanner
	tree        TreeModel
	savePath    string

	progress    ProgressModel
	cursorLeaf  *TreeNode
	sourceStep  int
	sourceInput string
	searchURLs  []string
	searchCur   int
	statusMsg   string
	errMsg      string
}

func newScanner() scanner.Scanner {
	return scanner.New()
}

func NewConfigBackupModel(sc scanner.Scanner, w int) ConfigBackupModel {
	return ConfigBackupModel{
		step:     cbScan,
		scanner:  sc,
		progress: NewProgressModel("正在扫描系统...", 0),
		tree:     NewTreeModel(nil, w-4, 20),
	}
}

func (m ConfigBackupModel) Init() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.scanner.Scan(context.Background(), scanner.ScanOptions{})
		if err != nil {
			return scanDoneMsg{err: err}
		}
		return scanDoneMsg{entries: entries}
	}
}

type scanDoneMsg struct {
	entries []store.AppEntry
	err     error
}

func (m ConfigBackupModel) Update(msg tea.Msg) (ConfigBackupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case scanDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("扫描失败: %v", msg.err)
			m.step = cbDone
			return m, nil
		}
		m.tree = buildTreeFromEntries(msg.entries, m.tree.Width, m.tree.Height)
		m.tree.ShowSource = true
		m.step = cbTree
		m.savePath = fmt.Sprintf("conf_backup_%s.db", time.Now().Format("20060102_150405"))
		return m, nil

	case saveDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("保存失败: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("已保存 %d 项到 %s", msg.count, msg.path)
		}
		m.step = cbDone
		return m, nil

	case searchDoneMsg:
		if msg.err != nil || len(msg.urls) == 0 {
			m.sourceStep = 1
			m.sourceInput = ""
			m.step = cbSourceInput
		} else {
			m.searchURLs = msg.urls
			m.searchCur = 0
		}
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case cbTree:
			switch msg.String() {
			case "p":
				node := m.tree.CurrentNode()
				if node != nil && node.Entry != nil {
					m.cursorLeaf = node
					m.sourceStep = 0
					m.step = cbSourceType
				}
				return m, nil
			case "enter":
				leaves := m.tree.SelectedLeaves()
				if len(leaves) == 0 {
					m.errMsg = "请至少选择一项"
					m.step = cbDone
					return m, nil
				}
				m.step = cbConfirm
				return m, nil
			case "esc":
				return m, func() tea.Msg { return menuChoice(0) }
			default:
				var cmd tea.Cmd
				m.tree, cmd = m.tree.Update(msg)
				return m, cmd
			}

		case cbSourceType:
			switch msg.String() {
			case "1":
				if m.cursorLeaf != nil && m.cursorLeaf.Entry != nil {
					m.step = cbSearchURL
					m.searchURLs = nil
					m.searchCur = 0
					softwareName := m.cursorLeaf.Entry.Name
					return m, func() tea.Msg {
						inst := engine.NewInstaller("")
						urls, err := inst.AutoSearchURL(context.Background(), softwareName)
						if err != nil {
							return searchDoneMsg{urls: nil, err: err}
						}
						return searchDoneMsg{urls: urls}
					}
				}
			case "2":
				m.sourceStep = 2
				m.sourceInput = ""
				m.step = cbSourceInput
			case "3":
				m.sourceStep = 3
				m.sourceInput = ""
				m.step = cbSourceInput
			case "0", "esc":
				m.step = cbTree
			}
			return m, nil

		case cbSearchURL:
			switch msg.String() {
			case "1":
				if len(m.searchURLs) > 0 {
					if m.cursorLeaf != nil && m.cursorLeaf.Entry != nil {
						m.cursorLeaf.Entry.SourceType = "url"
						m.cursorLeaf.Entry.SourceValue = m.searchURLs[m.searchCur]
					}
					m.step = cbTree
				}
			case "2":
				m.sourceStep = 1
				m.sourceInput = ""
				m.step = cbSourceInput
			case "0", "esc":
				m.step = cbTree
			default:
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					idx := int(msg.String()[0] - '0')
					if idx > 0 && idx <= len(m.searchURLs) {
						m.searchCur = idx - 1
					}
				}
			}
			return m, nil

		case cbSourceInput:
			switch msg.String() {
			case "enter":
				if m.sourceInput == "" {
					m.step = cbTree
					return m, nil
				}
				if m.cursorLeaf != nil && m.cursorLeaf.Entry != nil {
					switch m.sourceStep {
					case 1:
						m.cursorLeaf.Entry.SourceType = "url"
						m.cursorLeaf.Entry.SourceValue = m.sourceInput
					case 2:
						m.cursorLeaf.Entry.SourceType = "portable"
						m.cursorLeaf.Entry.SourceValue = m.sourceInput
					case 3:
						m.cursorLeaf.Entry.SourceType = "archive"
						m.cursorLeaf.Entry.SourceValue = m.sourceInput
					}
				}
				m.step = cbTree
				return m, nil
			case "esc":
				m.step = cbTree
				return m, nil
			case "backspace":
				if len(m.sourceInput) > 0 {
					m.sourceInput = m.sourceInput[:len(m.sourceInput)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.sourceInput += msg.String()
				}
				return m, nil
			}

		case cbConfirm:
			switch msg.String() {
			case "enter":
				m.step = cbDone
				return m, func() tea.Msg {
					leaves := m.tree.SelectedLeaves()
	var entries []store.AppEntry
					for _, leaf := range leaves {
						if leaf.Entry == nil {
							continue
						}
						if leaf.Entry.Category == "system" {
							continue
						}
						e := store.AppEntry{
							Name:     leaf.Entry.Name,
							Platform: leaf.Entry.Platform,
						}
						switch leaf.Entry.SourceType {
						case "url":
							e.DownloadURL = leaf.Entry.SourceValue
							e.NeedManualDL = leaf.Entry.SourceValue == ""
						case "portable":
							e.PackagePath = leaf.Entry.SourceValue
							e.IsArchive = false
						case "archive":
							e.PackagePath = leaf.Entry.SourceValue
							e.IsArchive = true
						default:
							e.NeedManualDL = true
						}
						entries = append(entries, e)
					}
					wf, err := app.NewWorkflow(m.savePath)
					if err != nil {
						return saveDoneMsg{err: err}
					}
					defer wf.DB().Close()
					if err := wf.BackupConfig(context.Background(), entries); err != nil {
						return saveDoneMsg{err: err}
					}
					return saveDoneMsg{path: m.savePath, count: len(entries)}
				}
			case "backspace":
				if len(m.savePath) > 0 {
					m.savePath = m.savePath[:len(m.savePath)-1]
				}
				return m, nil
			case "esc":
				m.step = cbTree
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.savePath += msg.String()
				}
				return m, nil
			}

		case cbDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return m, func() tea.Msg { return menuChoice(0) }
			}
		}
	}

	return m, nil
}

type saveDoneMsg struct {
	path  string
	count int
	err   error
}

type searchDoneMsg struct {
	urls []string
	err  error
}

func (m ConfigBackupModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("创建系统配置") + "\n\n")

	switch m.step {
	case cbScan:
		b.WriteString(m.progress.View())

	case cbTree:
		b.WriteString("↑↓ 移动  →← 展开/收起  Space 选择  p 设置来源  Enter 确认  Esc 返回\n")
		b.WriteString(m.tree.View() + "\n")
		leaves := m.tree.SelectedLeaves()
		b.WriteString(fmt.Sprintf("  已选: %d 项\n", len(leaves)))

	case cbSourceType:
		b.WriteString("选择安装包类型:\n\n")
		b.WriteString("  1. URL 安装包 — 恢复时自动下载\n")
		b.WriteString("  2. 免安装目录 — 复制文件夹 + 桌面快捷方式\n")
		b.WriteString("  3. 压缩包 — 打开路径让用户手动处理\n")
		b.WriteString("  0. 取消\n")

	case cbSearchURL:
		b.WriteString("正在搜索下载地址...\n\n")
		if len(m.searchURLs) > 0 {
			b.WriteString(fmt.Sprintf("  找到 %d 个结果:\n\n", len(m.searchURLs)))
			for i, u := range m.searchURLs {
				cur := " "
				if i == m.searchCur {
					cur = ">"
				}
				b.WriteString(fmt.Sprintf("  %s %d. %s\n", cur, i+1, u))
			}
			b.WriteString("\n")
			b.WriteString("  1. 确认使用当前选中\n")
			b.WriteString("  2. 手动输入地址\n")
			b.WriteString("  0. 取消\n")
		} else {
			b.WriteString("  (搜索结果为空，降级为手动输入)\n")
			b.WriteString(helpStyle.Render("  Enter 手动输入  Esc 取消") + "\n")
		}

	case cbSourceInput:
		label := "输入下载地址:"
		if m.sourceStep == 2 {
			label = "输入文件夹相对路径:"
		} else if m.sourceStep == 3 {
			label = "输入压缩包相对路径:"
		}
		b.WriteString(label + "\n")
		b.WriteString(fmt.Sprintf("  > %s\n", m.sourceInput))

	case cbConfirm:
		leaves := m.tree.SelectedLeaves()
		b.WriteString("即将备份以下内容:\n\n")
		sw, sys, urls, portables, archives := 0, 0, 0, 0, 0
		for _, l := range leaves {
			if l.Entry == nil {
				continue
			}
			if l.Entry.Category == "system" {
				sys++
				continue
			}
			sw++
			switch l.Entry.SourceType {
			case "url":
				urls++
			case "portable":
				portables++
			case "archive":
				archives++
			default:
				urls++
			}
		}
		b.WriteString(fmt.Sprintf("  软件 — %d 项 (URL: %d  免安装: %d  压缩包: %d)\n", sw, urls, portables, archives))
		b.WriteString(fmt.Sprintf("  系统设置 — %d 项\n", sys))
		b.WriteString("\n  保存文件名:\n")
		b.WriteString(fmt.Sprintf("  > %s\n", m.savePath))
		b.WriteString(helpStyle.Render("  Enter 确认保存  Esc 返回修改") + "\n")

	case cbDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + m.errMsg) + "\n")
		} else if m.statusMsg != "" {
			b.WriteString(successStyle.Render("  ✓ " + m.statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}

	return b.String()
}

func buildTreeFromEntries(entries []store.AppEntry, width, height int) TreeModel {
	system := &TreeNode{Name: "系统设置", Checked: true, Expanded: true}
	software := &TreeNode{Name: "软件", Checked: true, Expanded: false}

	for _, entry := range entries {
		node := &TreeNode{
			Name: entry.Name,
			Entry: &AppEntryExt{
				Name:     entry.Name,
				Category: "software",
				Platform: entry.Platform,
			},
			Checked: true,
		}

		if strings.HasPrefix(entry.Name, "[软件]") {
			software.Children = append(software.Children, node)
			node.Name = strings.TrimPrefix(entry.Name, "[软件] ")
			node.Entry.Name = node.Name
			node.Entry.SourceType = "url"
		} else if strings.HasPrefix(entry.Name, "[系统设置]") {
			system.Children = append(system.Children, node)
			node.Name = strings.TrimPrefix(entry.Name, "[系统设置] ")
			node.Entry.Name = node.Name
			node.Entry.Category = "system"
		} else {
			system.Children = append(system.Children, node)
			node.Entry.Category = "system"
		}
	}

	roots := []*TreeNode{system, software}
	return NewTreeModel(roots, width, height)
}
