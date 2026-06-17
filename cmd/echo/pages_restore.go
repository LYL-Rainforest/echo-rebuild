package main

import (
	"context"
	"fmt"
	"strings"

	"echo-rebuild/internal/app"
	"echo-rebuild/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type restoreStep int

const (
	rsPathInput restoreStep = iota
	rsTree
	rsProgress
	rsDone
)

type ConfigRestoreModel struct {
	step      restoreStep
	dbPath    string
	tree      TreeModel
	summary   *app.RestoreSummary
	statusMsg string
	errMsg    string
	width     int
	loaded    []store.AppEntry
}

func NewConfigRestoreModel(w int) ConfigRestoreModel {
	return ConfigRestoreModel{
		step:  rsPathInput,
		width: w,
	}
}

func (m ConfigRestoreModel) Init() tea.Cmd { return nil }

func (m ConfigRestoreModel) Update(msg tea.Msg) (ConfigRestoreModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadDBMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("加载失败: %v", msg.err)
			m.step = rsDone
			return m, nil
		}
		m.loaded = msg.entries
		m.tree = buildTreeFromEntries(msg.entries, m.width-4, 20)
		m.step = rsTree
		return m, nil

	case restoreDoneMsg:
		m.summary = msg.summary
		m.statusMsg = fmt.Sprintf("完成: %d 成功, %d 手动, %d 回退, %d 跳过",
			msg.summary.Success, msg.summary.Manual, msg.summary.Fallback, msg.summary.Skipped)
		m.step = rsDone
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case rsPathInput:
			switch msg.String() {
			case "enter":
				if m.dbPath == "" {
					m.dbPath = "backup.db"
				}
				return m, func() tea.Msg {
					wf, err := app.NewWorkflow(m.dbPath)
					if err != nil {
						return loadDBMsg{err: err}
					}
					entries, err := store.LoadEntries(wf.DB(), "")
					if err != nil {
						return loadDBMsg{err: err}
					}
					return loadDBMsg{entries: entries}
				}
			case "esc":
				return m, func() tea.Msg { return menuChoice(0) }
			case "backspace":
				if len(m.dbPath) > 0 {
					m.dbPath = m.dbPath[:len(m.dbPath)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.dbPath += msg.String()
				}
				return m, nil
			}

		case rsTree:
			switch msg.String() {
			case "enter":
				leaves := m.tree.SelectedLeaves()
				if len(leaves) == 0 {
					m.errMsg = "请至少选择一项"
					m.step = rsDone
					return m, nil
				}
				m.step = rsProgress
				m.statusMsg = "正在还原..."
				return m, func() tea.Msg {
					wf, err := app.NewWorkflow(m.dbPath)
					if err != nil {
						return restoreDoneMsg{summary: &app.RestoreSummary{}}
					}
					var entries []store.AppEntry
					for _, leaf := range leaves {
						if leaf.Entry == nil {
							continue
						}
						// match loaded entry by name
						var matched *store.AppEntry
						for i := range m.loaded {
							if m.loaded[i].Name == leaf.Entry.Name ||
								m.loaded[i].Name == "[软件] "+leaf.Entry.Name ||
								m.loaded[i].Name == "[系统设置] "+leaf.Entry.Name {
								matched = &m.loaded[i]
								break
							}
						}
						if matched != nil {
							entries = append(entries, *matched)
						} else {
							entries = append(entries, store.AppEntry{
								Name:     leaf.Entry.Name,
								Platform: leaf.Entry.Platform,
							})
						}
					}
					summary := wf.RestoreConfig(context.Background(), entries, ".")
					return restoreDoneMsg{summary: summary}
				}
			case "esc":
				m.step = rsPathInput
				m.dbPath = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.tree, cmd = m.tree.Update(msg)
				return m, cmd
			}

		case rsDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return m, func() tea.Msg { return menuChoice(0) }
			}
		}
	}
	return m, nil
}

func (m ConfigRestoreModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("还原系统配置") + "\n\n")

	switch m.step {
	case rsPathInput:
		b.WriteString("请输入备份文件路径:\n")
		b.WriteString(fmt.Sprintf("  > %s\n", m.dbPath))
		b.WriteString(helpStyle.Render("  Enter 确认  Esc 返回") + "\n")

	case rsTree:
		b.WriteString("↑↓ 移动  →← 展开/收起  Space 选择  Enter 还原  Esc 返回\n")
		b.WriteString(m.tree.View() + "\n")
		leaves := m.tree.SelectedLeaves()
		b.WriteString(fmt.Sprintf("  已选: %d 项\n", len(leaves)))

	case rsProgress:
		b.WriteString(m.statusMsg + "\n")

	case rsDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + m.errMsg) + "\n")
		} else {
			b.WriteString(successStyle.Render("  ✓ "+m.statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}

	return b.String()
}

type loadDBMsg struct {
	entries []store.AppEntry
	err     error
}

type restoreDoneMsg struct {
	summary *app.RestoreSummary
}
