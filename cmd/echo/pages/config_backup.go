package pages

import (
	"context"
	"fmt"
	"strings"
	"time"

	msgpkg "echo-rebuild/cmd/echo/msg"
	"echo-rebuild/internal/app"
	"echo-rebuild/internal/engine"
	"echo-rebuild/internal/scanner"
	"echo-rebuild/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type treeStep int

const (
	cbScan treeStep = iota
	cbTree
	cbSourceType
	cbSearchURL
	cbSourceInput
	cbConfirm
	cbDone
)

type TreeNode struct {
	Name     string
	Entry    *AppEntryExt
	Children []*TreeNode
	Depth    int
	Expanded bool
	Checked  bool
	Partial  bool
}

type AppEntryExt struct {
	Name        string
	OrigName    string
	Category    string
	SourceType  string
	SourceValue string
	Platform    string
}

type TreeModel struct {
	Roots       []*TreeNode
	Cursor      int
	ScrollOffset int
	Width       int
	Height      int
	ShowSource  bool
}

func NewTreeModel(roots []*TreeNode, width, height int) TreeModel {
	return TreeModel{Roots: roots, Width: width, Height: height}
}

func (n *TreeNode) IsLeaf() bool { return len(n.Children) == 0 }

func (n *TreeNode) toggle() {
	n.Checked = !n.Checked
	n.Partial = false
	for _, c := range n.Children { c.setAll(n.Checked) }
}

func (n *TreeNode) setAll(checked bool) {
	n.Checked = checked
	n.Partial = false
	for _, c := range n.Children { c.setAll(checked) }
}

func (tm *TreeModel) visibleNodes() []*TreeNode {
	var nodes []*TreeNode
	var walk func(n *TreeNode, depth int)
	walk = func(n *TreeNode, depth int) {
		n.Depth = depth
		nodes = append(nodes, n)
		if n.Expanded {
			for _, c := range n.Children { walk(c, depth+1) }
		}
	}
	for _, r := range tm.Roots { walk(r, 0) }
	return nodes
}

func (tm *TreeModel) MoveCursor(delta int) {
	nodes := tm.visibleNodes()
	if len(nodes) == 0 { return }
	tm.Cursor += delta
	if tm.Cursor < 0 { tm.Cursor = 0 }
	if tm.Cursor >= len(nodes) { tm.Cursor = len(nodes) - 1 }
	// scroll viewport to keep cursor visible
	if tm.Cursor < tm.ScrollOffset {
		tm.ScrollOffset = tm.Cursor
	}
	if tm.ScrollOffset+tm.Height > 0 && tm.Cursor >= tm.ScrollOffset+tm.Height {
		tm.ScrollOffset = tm.Cursor - tm.Height + 1
	}
}

func (tm *TreeModel) ToggleExpand() {
	nodes := tm.visibleNodes()
	if tm.Cursor >= len(nodes) { return }
	node := nodes[tm.Cursor]
	if !node.IsLeaf() { node.Expanded = !node.Expanded }
}

func (tm *TreeModel) ToggleCheck() {
	nodes := tm.visibleNodes()
	if tm.Cursor >= len(nodes) { return }
	nodes[tm.Cursor].toggle()
	tm.recalcAll()
}

func (tm *TreeModel) recalcAll() {
	for _, r := range tm.Roots { tm.recalcNode(r) }
}

func (tm *TreeModel) recalcNode(n *TreeNode) {
	if len(n.Children) == 0 { return }
	for _, c := range n.Children { tm.recalcNode(c) }
	anyC, allC := false, true
	for _, c := range n.Children {
		if c.Checked || c.Partial { anyC = true }
		if !c.Checked && !c.Partial { allC = false }
	}
	n.Checked = allC && anyC
	n.Partial = anyC && !n.Checked
}

func (tm *TreeModel) CurrentNode() *TreeNode {
	nodes := tm.visibleNodes()
	if tm.Cursor < len(nodes) { return nodes[tm.Cursor] }
	return nil
}

func (tm TreeModel) SelectedLeaves() []*TreeNode {
	var leaves []*TreeNode
	var collect func(n *TreeNode)
	collect = func(n *TreeNode) {
		if n.Checked || n.Partial {
			if n.IsLeaf() && n.Entry != nil { leaves = append(leaves, n) }
			for _, c := range n.Children { collect(c) }
		}
	}
	for _, r := range tm.Roots { collect(r) }
	return leaves
}

func (tm TreeModel) Update(tmsg tea.Msg) (TreeModel, tea.Cmd) {
	switch msg := tmsg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k": tm.MoveCursor(-1)
		case "down", "j": tm.MoveCursor(1)
		case "right": tm.ToggleExpand()
		case "left":
			nodes := tm.visibleNodes()
			if tm.Cursor < len(nodes) {
				node := nodes[tm.Cursor]
				if node.Expanded { node.Expanded = false }
			}
		case " ": tm.ToggleCheck()
		}
	}
	return tm, nil
}

func (tm TreeModel) View() string {
	nodes := tm.visibleNodes()
	start := tm.ScrollOffset
	end := start + tm.Height
	if end > len(nodes) { end = len(nodes) }
	if start > len(nodes) { start = 0 }
	var lines []string
	for i := start; i < end; i++ {
		node := nodes[i]
		indent := strings.Repeat("  ", node.Depth)
		check := "[ ]"
		if node.Checked { check = "[✓]" } else if node.Partial { check = "[~]" }
		cur := "  "
		if i == tm.Cursor { cur = "> " }
		marker := " "
		if !node.IsLeaf() {
			if node.Expanded { marker = "▼" } else { marker = "▶" }
		}
		line := cur + check + " " + marker + " " + indent + node.Name
		if tm.ShowSource && node.Entry != nil && node.Entry.SourceType != "" {
			src := ""
			switch node.Entry.SourceType {
			case "url": src = "URL"
			case "portable": src = "免安装 → " + node.Entry.SourceValue
			case "archive": src = "压缩包 → " + node.Entry.SourceValue
			}
			line += dimmedStyle.Render("  [" + src + "]")
		}
		if i == tm.Cursor {
			lines = append(lines, selectedMenuStyle.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func buildTreeFromEntries(entries []store.AppEntry, width, height int) *TreeModel {
	sys := &TreeNode{Name: "系统设置", Checked: true, Expanded: true}
	sw := &TreeNode{Name: "软件", Checked: true, Expanded: false}
	for _, entry := range entries {
		var cat string
		disp := entry.Name
		if strings.HasPrefix(entry.Name, "[软件]") {
			cat = "software"
			disp = strings.TrimPrefix(entry.Name, "[软件] ")
		} else if strings.HasPrefix(entry.Name, "[系统设置]") {
			cat = "system"
			disp = strings.TrimPrefix(entry.Name, "[系统设置] ")
		} else {
			cat = "system"
		}
		node := &TreeNode{
			Name:    disp,
			Checked: true,
			Entry: &AppEntryExt{
				Name:       disp,
				OrigName:   entry.Name,
				Category:   cat,
				Platform:   entry.Platform,
				SourceType: "url",
			},
		}
		if node.Entry.Category == "software" {
			sw.Children = append(sw.Children, node)
		} else {
			sys.Children = append(sys.Children, node)
		}
	}
	tm := NewTreeModel([]*TreeNode{sys, sw}, width, height)
	return &tm
}

type ConfigBackup struct {
	step        treeStep
	scanner     scanner.Scanner
	tree        *TreeModel
	savePath    string
	cursorLeaf  *TreeNode
	sourceStep  int
	sourceInput string
	searchURLs  []string
	searchCur   int
	statusMsg   string
	errMsg      string
}

func NewConfigBackup(sc scanner.Scanner) ConfigBackup {
	return ConfigBackup{step: cbScan, scanner: sc}
}

func (cb ConfigBackup) Init() tea.Cmd {
	return func() tea.Msg {
		entries, err := cb.scanner.Scan(context.Background(), scanner.ScanOptions{})
		if err != nil { return msgpkg.ScanDone{Err: err} }
		return msgpkg.ScanDone{Entries: entries}
	}
}

func (cb ConfigBackup) Update(tmsg tea.Msg) (ConfigBackup, tea.Cmd) {
	switch msg := tmsg.(type) {
	case msgpkg.ScanDone:
		if msg.Err != nil {
			cb.errMsg = fmt.Sprintf("扫描失败: %v", msg.Err)
			cb.step = cbDone; return cb, nil
		}
		cb.tree = buildTreeFromEntries(msg.Entries, 60, 20)
		cb.tree.ShowSource = true
		cb.step = cbTree
		cb.savePath = fmt.Sprintf("conf_backup_%s.db", time.Now().Format("20060102_150405"))
		return cb, nil

	case msgpkg.SaveDone:
		if msg.Err != nil {
			cb.errMsg = fmt.Sprintf("保存失败: %v", msg.Err)
		} else {
			cb.statusMsg = fmt.Sprintf("已保存 %d 项到 %s", msg.Count, msg.Path)
		}
		cb.step = cbDone; return cb, nil

	case msgpkg.SearchDone:
		if msg.Err != nil || len(msg.URLs) == 0 {
			cb.sourceStep = 1; cb.sourceInput = ""; cb.step = cbSourceInput
		} else {
			cb.searchURLs = msg.URLs; cb.searchCur = 0
		}
		return cb, nil

	case tea.KeyMsg:
		switch cb.step {
		case cbTree:
			switch msg.String() {
			case "p":
				node := cb.tree.CurrentNode()
				if node != nil && node.Entry != nil && node.Entry.Category != "system" {
					cb.cursorLeaf = node
					cb.sourceStep = 0
					cb.step = cbSourceType
				}
				return cb, nil
			case "enter":
				leaves := cb.tree.SelectedLeaves()
				if len(leaves) == 0 {
					cb.errMsg = "请至少选择一项"; cb.step = cbDone; return cb, nil
				}
				cb.step = cbConfirm; return cb, nil
			case "esc":
				return cb, func() tea.Msg { return msgpkg.MenuChoice(0) }
			default:
				tm, cmd := cb.tree.Update(tmsg)
				cb.tree = &tm; return cb, cmd
			}
		case cbSourceType:
			switch msg.String() {
			case "1":
				if cb.cursorLeaf != nil && cb.cursorLeaf.Entry != nil {
					cb.step = cbSearchURL
					cb.searchURLs = nil; cb.searchCur = 0
					name := cb.cursorLeaf.Entry.Name
					return cb, func() tea.Msg {
						inst := engine.NewInstaller("")
						urls, err := inst.AutoSearchURL(context.Background(), name)
						return msgpkg.SearchDone{URLs: urls, Err: err}
					}
				}
			case "2": cb.sourceStep = 2; cb.sourceInput = ""; cb.step = cbSourceInput
			case "3": cb.sourceStep = 3; cb.sourceInput = ""; cb.step = cbSourceInput
			case "0", "esc": cb.step = cbTree
			}
			return cb, nil
		case cbSearchURL:
			switch msg.String() {
			case "1":
				if len(cb.searchURLs) > 0 && cb.cursorLeaf != nil && cb.cursorLeaf.Entry != nil {
					cb.cursorLeaf.Entry.SourceType = "url"
					cb.cursorLeaf.Entry.SourceValue = cb.searchURLs[cb.searchCur]
					cb.step = cbTree
				}
			case "2": cb.sourceStep = 1; cb.sourceInput = ""; cb.step = cbSourceInput
			case "0", "esc": cb.step = cbTree
			default:
				if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
					idx := int(msg.String()[0] - '1')
					if idx < len(cb.searchURLs) { cb.searchCur = idx }
				}
			}
			return cb, nil
		case cbSourceInput:
			switch msg.String() {
			case "enter":
				if cb.cursorLeaf != nil && cb.cursorLeaf.Entry != nil && cb.sourceInput != "" {
					switch cb.sourceStep {
					case 1: cb.cursorLeaf.Entry.SourceType = "url"; cb.cursorLeaf.Entry.SourceValue = cb.sourceInput
					case 2: cb.cursorLeaf.Entry.SourceType = "portable"; cb.cursorLeaf.Entry.SourceValue = cb.sourceInput
					case 3: cb.cursorLeaf.Entry.SourceType = "archive"; cb.cursorLeaf.Entry.SourceValue = cb.sourceInput
					}
				}
				cb.step = cbTree; return cb, nil
			case "esc": cb.step = cbTree; return cb, nil
			case "backspace":
				if len(cb.sourceInput) > 0 { cb.sourceInput = cb.sourceInput[:len(cb.sourceInput)-1] }
				return cb, nil
			default:
				if len(msg.String()) == 1 { cb.sourceInput += msg.String() }
				return cb, nil
			}
		case cbConfirm:
			switch msg.String() {
			case "enter":
				cb.step = cbDone
				return cb, func() tea.Msg {
					leaves := cb.tree.SelectedLeaves()
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
						e := store.AppEntry{Name: name, Platform: leaf.Entry.Platform}
						switch leaf.Entry.SourceType {
						case "url":
							e.DownloadURL = leaf.Entry.SourceValue
							e.NeedManualDL = leaf.Entry.SourceValue == ""
						case "portable":
							e.PackagePath = leaf.Entry.SourceValue; e.IsArchive = false
						case "archive":
							e.PackagePath = leaf.Entry.SourceValue; e.IsArchive = true
						default:
							e.NeedManualDL = true
						}
						entries = append(entries, e)
					}
					wf, err := app.NewWorkflow(cb.savePath)
					if err != nil { return msgpkg.SaveDone{Err: err} }
					defer wf.DB().Close()
					if err := wf.BackupConfig(context.Background(), entries); err != nil {
						return msgpkg.SaveDone{Err: err}
					}
					return msgpkg.SaveDone{Path: cb.savePath, Count: len(entries)}
				}
			case "backspace":
				if len(cb.savePath) > 0 { cb.savePath = cb.savePath[:len(cb.savePath)-1] }
				return cb, nil
			case "esc": cb.step = cbTree; return cb, nil
			default:
				if len(msg.String()) == 1 { cb.savePath += msg.String() }
				return cb, nil
			}
		case cbDone:
			switch msg.String() {
			case "enter", "esc", " ":
				return cb, func() tea.Msg { return msgpkg.MenuChoice(0) }
			}
		}
	}
	return cb, nil
}

func (cb ConfigBackup) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("创建系统配置") + "\n\n")
	switch cb.step {
	case cbScan:
		b.WriteString("正在扫描系统...\n")
	case cbTree:
		b.WriteString("↑↓移动  →←展开  Space选择  p设来源  Enter确认  Esc返回\n")
		b.WriteString(cb.tree.View() + "\n")
		b.WriteString(fmt.Sprintf("  已选: %d 项\n", len(cb.tree.SelectedLeaves())))
	case cbSourceType:
		b.WriteString("选择安装包类型:\n\n  1. URL 安装包\n  2. 免安装目录\n  3. 压缩包\n  0. 取消\n")
	case cbSearchURL:
		if len(cb.searchURLs) > 0 {
			b.WriteString(fmt.Sprintf("找到 %d 个结果:\n\n", len(cb.searchURLs)))
			for i, u := range cb.searchURLs {
				cur := " "
				if i == cb.searchCur { cur = ">" }
				b.WriteString(fmt.Sprintf("  %s %d. %s\n", cur, i+1, u))
			}
			b.WriteString("\n  1. 确认  2. 手动输入  0. 取消\n")
		} else {
			b.WriteString("搜索结果为空，降级为手动输入\n")
		}
	case cbSourceInput:
		label := "输入下载地址:"
		if cb.sourceStep == 2 { label = "输入文件夹相对路径:" }
		if cb.sourceStep == 3 { label = "输入压缩包相对路径:" }
		b.WriteString(label + "\n  > " + cb.sourceInput + "\n")
	case cbConfirm:
		leaves := cb.tree.SelectedLeaves()
		sw, sys, urls, portables, archives := 0, 0, 0, 0, 0
		for _, l := range leaves {
			if l.Entry == nil { continue }
			if l.Entry.Category == "system" { sys++; continue }
			sw++
			switch l.Entry.SourceType {
			case "url": urls++
			case "portable": portables++
			case "archive": archives++
			default: urls++
			}
		}
		b.WriteString(fmt.Sprintf("  软件 — %d 项 (URL: %d  免安装: %d  压缩包: %d)\n", sw, urls, portables, archives))
		b.WriteString(fmt.Sprintf("  系统设置 — %d 项\n", sys))
		b.WriteString("\n  保存文件名:\n  > " + cb.savePath + "\n")
		b.WriteString(helpStyle.Render("  Enter 确认保存  Esc 返回修改") + "\n")
	case cbDone:
		if cb.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + cb.errMsg) + "\n")
		} else if cb.statusMsg != "" {
			b.WriteString(successStyle.Render("  ✓ " + cb.statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("  Enter 返回") + "\n")
	}
	return b.String()
}
