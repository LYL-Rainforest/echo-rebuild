package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type TreeNode struct {
	Name     string
	Entry    *AppEntryExt // nil for category nodes
	Children []*TreeNode
	Depth    int
	Expanded bool
	Checked  bool
	Partial  bool
}

type AppEntryExt struct {
	Name         string
	Category     string
	SourceType   string // "url" / "portable" / "archive" / ""
	SourceValue  string
	Platform     string
}

func (n *TreeNode) IsLeaf() bool {
	return len(n.Children) == 0
}

func (n *TreeNode) toggle() {
	n.Checked = !n.Checked
	n.Partial = false
	for _, child := range n.Children {
		child.setAll(n.Checked)
	}
}

func (n *TreeNode) setAll(checked bool) {
	n.Checked = checked
	n.Partial = false
	for _, child := range n.Children {
		child.setAll(checked)
	}
}

type TreeModel struct {
	Roots       []*TreeNode
	Cursor      int
	Width       int
	Height      int
	ShowSource  bool
	viewport    viewport.Model
	OnEnter     func(selected []*TreeNode) tea.Cmd
}

func NewTreeModel(roots []*TreeNode, width, height int) TreeModel {
	vp := viewport.New(width, height)
	vp.Style = boxStyle
	return TreeModel{
		Roots:    roots,
		Width:    width,
		Height:   height,
		viewport: vp,
	}
}

func (m *TreeModel) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	m.viewport.Width = w
	m.viewport.Height = h
}

func (m TreeModel) visibleNodes() []*TreeNode {
	var nodes []*TreeNode
	var walk func(n *TreeNode, depth int)
	walk = func(n *TreeNode, depth int) {
		n.Depth = depth
		nodes = append(nodes, n)
		if n.Expanded {
			for _, child := range n.Children {
				walk(child, depth+1)
			}
		}
	}
	for _, root := range m.Roots {
		walk(root, 0)
	}
	return nodes
}

func (m *TreeModel) MoveCursor(delta int) {
	nodes := m.visibleNodes()
	if len(nodes) == 0 {
		return
	}
	m.Cursor = (m.Cursor + delta + len(nodes)) % len(nodes)
	m.scrollToCursor()
}

func (m *TreeModel) scrollToCursor() {
	if m.viewport.YOffset > m.Cursor {
		m.viewport.YOffset = m.Cursor
	}
	if m.viewport.YOffset+m.viewport.Height-1 <= m.Cursor {
		m.viewport.YOffset = m.Cursor - m.viewport.Height + 2
	}
	if m.viewport.YOffset < 0 {
		m.viewport.YOffset = 0
	}
}

func (m *TreeModel) ToggleExpand() {
	nodes := m.visibleNodes()
	if m.Cursor >= len(nodes) {
		return
	}
	node := nodes[m.Cursor]
	if !node.IsLeaf() {
		node.Expanded = !node.Expanded
	}
	m.scrollToCursor()
}

func (m *TreeModel) ToggleCheck() {
	nodes := m.visibleNodes()
	if m.Cursor >= len(nodes) {
		return
	}
	node := nodes[m.Cursor]
	node.toggle()
	m.recalcAll()
}

func (m *TreeModel) recalcAll() {
	for _, root := range m.Roots {
		m.recalcNode(root, nil)
	}
}

func (m *TreeModel) recalcNode(n *TreeNode, _ []*TreeNode) {
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			m.recalcNode(child, nil)
		}
		anyChecked := false
		allChecked := true
		for _, child := range n.Children {
			if child.Checked || child.Partial {
				anyChecked = true
			}
			if !child.Checked && !child.Partial {
				allChecked = false
			}
		}
		n.Checked = allChecked && anyChecked
		n.Partial = anyChecked && !n.Checked
	}
}

func (m *TreeModel) CurrentNode() *TreeNode {
	nodes := m.visibleNodes()
	if m.Cursor < len(nodes) {
		return nodes[m.Cursor]
	}
	return nil
}

func (m TreeModel) SelectedLeaves() []*TreeNode {
	var leaves []*TreeNode
	var collect func(n *TreeNode)
	collect = func(n *TreeNode) {
		if n.Checked || n.Partial {
			if n.IsLeaf() && n.Entry != nil {
				leaves = append(leaves, n)
			}
			for _, child := range n.Children {
				collect(child)
			}
		}
	}
	for _, root := range m.Roots {
		collect(root)
	}
	return leaves
}

func (m TreeModel) Update(msg tea.Msg) (TreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.MoveCursor(-1)
		case "down", "j":
			m.MoveCursor(1)
		case "right":
			m.ToggleExpand()
		case "left":
			nodes := m.visibleNodes()
			if m.Cursor < len(nodes) {
				node := nodes[m.Cursor]
				if node.Expanded {
					node.Expanded = false
				}
			}
		case " ":
			m.ToggleCheck()
		}
	}
	return m, nil
}

func (m TreeModel) View() string {
	nodes := m.visibleNodes()
	var lines []string
	for i, node := range nodes {
		indent := strings.Repeat("  ", node.Depth)

		check := "[ ]"
		if node.Checked {
			check = "[✓]"
		} else if node.Partial {
			check = "[~]"
		}

		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		marker := " "
		if !node.IsLeaf() {
			if node.Expanded {
				marker = "▼"
			} else {
				marker = "▶"
			}
		}

		line := cursor + check + " " + marker + " " + indent + node.Name

		if m.ShowSource && node.Entry != nil && node.Entry.SourceType != "" {
			src := ""
			switch node.Entry.SourceType {
			case "url":
				if node.Entry.SourceValue != "" {
					src = "URL: " + node.Entry.SourceValue
				} else {
					src = "URL (未填)"
				}
			case "portable":
				src = "免安装 → " + node.Entry.SourceValue
			case "archive":
				src = "压缩包 → " + node.Entry.SourceValue
			}
			line += dimmedStyle.Render("  [" + src + "]")
		}

		if i == m.Cursor {
			lines = append(lines, selectedMenuStyle.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	return m.viewport.View()
}
