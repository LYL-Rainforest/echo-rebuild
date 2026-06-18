package pages

import (
	"context"
	"fmt"
	"strings"

	msgpkg "echo-rebuild/cmd/echo/msg"
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

type ConfigRestore struct {
	step      restoreStep
	dbPath    string
	tree      *TreeModel
	summary   *app.RestoreSummary
	statusMsg string
	errMsg    string
	loaded    []store.AppEntry
}

func NewConfigRestore() ConfigRestore {
	return ConfigRestore{step: rsPathInput}
}

func (cr ConfigRestore) Init() tea.Cmd { return nil }

func (cr ConfigRestore) Update(tmsg tea.Msg) (ConfigRestore, tea.Cmd) {
	switch msg := tmsg.(type) {
	case msgpkg.ScanDone:
		if msg.Err != nil {
			cr.errMsg = fmt.Sprintf("加载失败: %v", msg.Err)
			cr.step = rsDone; return cr, nil
		}
		cr.loaded = msg.Entries
		cr.tree = buildTreeFromEntries(msg.Entries, 60, 20)
		cr.step = rsTree; return cr, nil

	case msgpkg.RestoreDone:
		if s, ok := msg.Summary.(*app.RestoreSummary); ok {
			cr.summary = s
			cr.statusMsg = fmt.Sprintf("完成: %d 成功, %d 手动, %d 回退, %d 跳过",
				s.Success, s.Manual, s.Fallback, s.Skipped)
		}
		cr.step = rsDone; return cr, nil

	case tea.KeyMsg:
		switch cr.step {
		case rsPathInput:
			switch msg.String() {
			case "enter":
				p := cr.dbPath
				if p == "" { p = "backup.db" }
				return cr, func() tea.Msg {
					wf, err := app.NewWorkflow(p)
					if err != nil { return msgpkg.ScanDone{Err: err} }
					entries, err := store.LoadEntries(wf.DB(), "")
					if err != nil { return msgpkg.ScanDone{Err: err} }
					return msgpkg.ScanDone{Entries: entries}
				}
			case "esc": return cr, func() tea.Msg { return msgpkg.MenuChoice(0) }
			case "backspace":
				if len(cr.dbPath) > 0 { cr.dbPath = cr.dbPath[:len(cr.dbPath)-1] }
				return cr, nil
			default:
				if len(msg.String()) == 1 { cr.dbPath += msg.String() }
				return cr, nil
			}
		case rsTree:
			switch msg.String() {
			case "enter":
				leaves := cr.tree.SelectedLeaves()
				if len(leaves) == 0 {
					cr.errMsg = "请至少选择一项"; cr.step = rsDone; return cr, nil
				}
				cr.step = rsProgress
				return cr, func() tea.Msg {
					wf, err := app.NewWorkflow(cr.dbPath)
					if err != nil { return msgpkg.RestoreDone{Summary: &app.RestoreSummary{}} }
					var entries []store.AppEntry
					for _, leaf := range leaves {
						if leaf.Entry == nil { continue }
						name := leaf.Entry.OrigName
						if name == "" {
							if leaf.Entry.Category == "software" {
								name = "[软件] " + leaf.Entry.Name
							} else {
								name = "[系统设置] " + leaf.Entry.Name
							}
						}
						var matched *store.AppEntry
						for i := range cr.loaded {
							if cr.loaded[i].Name == name {
								matched = &cr.loaded[i]; break
							}
						}
						if matched != nil {
							entries = append(entries, *matched)
						} else {
							entries = append(entries, store.AppEntry{Name: name, Platform: leaf.Entry.Platform})
						}
					}
					summary := wf.RestoreConfig(context.Background(), entries, ".")
					return msgpkg.RestoreDone{Summary: summary}
				}
			case "esc": cr.step = rsPathInput; cr.dbPath = ""; return cr, nil
			default:
				tm, cmd := cr.tree.Update(tmsg)
				cr.tree = &tm; return cr, cmd
			}
		case rsDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return cr, func() tea.Msg { return msgpkg.MenuChoice(0) }
			}
		}
	}
	return cr, nil
}

func (cr ConfigRestore) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("还原系统配置") + "\n\n")
	switch cr.step {
	case rsPathInput:
		b.WriteString("请输入备份文件路径:\n  > " + cr.dbPath + "\n")
		b.WriteString(helpStyle.Render("  Enter 确认  Esc 返回") + "\n")
	case rsTree:
		b.WriteString("↑↓移动  →←展开  Space选择  Enter还原  Esc返回\n")
		b.WriteString(cr.tree.View() + "\n")
		b.WriteString(fmt.Sprintf("  已选: %d 项\n", len(cr.tree.SelectedLeaves())))
	case rsProgress:
		b.WriteString("正在还原...\n")
	case rsDone:
		if cr.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + cr.errMsg) + "\n")
		} else {
			b.WriteString(successStyle.Render("  ✓ "+cr.statusMsg) + "\n")
			if cr.summary != nil {
				if len(cr.summary.ManualNames) > 0 {
					b.WriteString("\n  ⚠ 需手动操作:\n")
					for _, n := range cr.summary.ManualNames { b.WriteString(fmt.Sprintf("       - %s\n", n)) }
				}
				if len(cr.summary.FallbackNames) > 0 {
					b.WriteString("\n  ⚠ 已回退到浏览器:\n")
					for _, n := range cr.summary.FallbackNames { b.WriteString(fmt.Sprintf("       - %s\n", n)) }
				}
				if len(cr.summary.SkippedNames) > 0 {
					b.WriteString("\n  - 已跳过:\n")
					for _, n := range cr.summary.SkippedNames { b.WriteString(fmt.Sprintf("       - %s\n", n)) }
				}
			}
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}
	return b.String()
}
