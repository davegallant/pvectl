package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// menuItem is either a leaf (Action set, Children nil) — selecting it
// returns Action directly — or a group (Action empty, Children set) —
// selecting it drills into Children instead of returning anything.
type menuItem struct {
	Label    string
	Action   string
	Children []menuItem
}

func (m menuItem) isGroup() bool { return len(m.Children) > 0 }

// ActionTree is the action menu's top-level items, mirroring pvectl's own
// `ct`/`qm` subcommand tree: `config`/`snapshots`/`backups` are groups whose
// children match `ct config`/`ct snapshots`/`ct backups`'s own subcommands
// (`config`: `edit`, matching `ct config edit`/`qm config edit` — see
// AGENTS.md's `ctConfigCmd`/`qmConfigCmd` note; `append` isn't included
// here, since it needs `--line` values the menu has no prompt for), rather
// than the flat "edit"/"snapshot"/"snapshots"/"delete-snapshot"/
// "rollback-snapshot"/"backup"/"backups"/"delete-backup" list this used to
// be. Leaf Action values are unchanged from that flat list, so
// dispatchAction/dispatchVMAction (cmd/ct.go, cmd/qm.go) need no changes —
// RunActionMenu's return contract is exactly the same string set as before.
var ActionTree = []menuItem{
	{Label: "enter", Action: "enter"},
	{Label: "start", Action: "start"},
	{Label: "stop", Action: "stop"},
	{Label: "reboot", Action: "reboot"},
	{Label: "config", Children: []menuItem{
		{Label: "edit", Action: "edit"},
	}},
	{Label: "rename", Action: "rename"},
	{Label: "snapshots", Children: []menuItem{
		{Label: "create", Action: "snapshot"},
		{Label: "list", Action: "snapshots"},
		{Label: "delete", Action: "delete-snapshot"},
		{Label: "rollback", Action: "rollback-snapshot"},
	}},
	{Label: "backups", Children: []menuItem{
		{Label: "create", Action: "backup"},
		{Label: "list", Action: "backups"},
		{Label: "delete", Action: "delete-backup"},
	}},
	{Label: "migrate", Action: "migrate"},
	{Label: "delete", Action: "delete"},
}

// LeafActions returns every leaf Action value in ActionTree, recursively
// flattening groups — lets other packages (cmd's
// TestActionAnnouncementsCoverActionTree) check they handle every action
// without being able to name the unexported menuItem type themselves.
func LeafActions() []string {
	var out []string
	var walk func(items []menuItem)
	walk = func(items []menuItem) {
		for _, it := range items {
			if it.isGroup() {
				walk(it.Children)
				continue
			}
			out = append(out, it.Action)
		}
	}
	walk(ActionTree)
	return out
}

// FilterMenuItems returns the items whose label contains query
// (case-insensitive). A pure function so it's testable independently of
// the bubbletea event loop.
func FilterMenuItems(items []menuItem, query string) []menuItem {
	if query == "" {
		return items
	}
	query = strings.ToLower(query)

	var out []menuItem
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Label), query) {
			out = append(out, it)
		}
	}
	return out
}

// menuLevel is one level of the menu's navigation stack: the items shown
// at that level, and the label of the group that was entered to reach it
// (empty for the root level).
type menuLevel struct {
	items []menuItem
	label string
}

type menuModel struct {
	stack    []menuLevel
	filtered []menuItem
	cursor   int
	input    textinput.Model
	selected string
	quitting bool
}

func newMenuModel() menuModel {
	ti := textinput.New()
	ti.Placeholder = "action > "
	ti.Focus()
	return menuModel{
		stack:    []menuLevel{{items: ActionTree}},
		filtered: ActionTree,
		input:    ti,
	}
}

func (m menuModel) Init() tea.Cmd { return nil }

// current is the level on top of the navigation stack — what's currently
// being browsed/filtered.
func (m menuModel) current() menuLevel {
	return m.stack[len(m.stack)-1]
}

// breadcrumb renders the group labels leading to the current level (e.g.
// "snapshots > "), or "" at the root.
func (m menuModel) breadcrumb() string {
	if len(m.stack) <= 1 {
		return ""
	}
	labels := make([]string, 0, len(m.stack)-1)
	for _, lvl := range m.stack[1:] {
		labels = append(labels, lvl.label)
	}
	return strings.Join(labels, " > ") + " > "
}

// resetLevel clears the search input and cursor after navigating to a
// (possibly new) level — the query and cursor position from the previous
// level don't carry any meaning at the new one.
func (m *menuModel) resetLevel() {
	m.filtered = m.current().items
	m.cursor = 0
	m.input.SetValue("")
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
				m.resetLevel()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				item := m.filtered[m.cursor]
				if item.isGroup() {
					m.stack = append(m.stack, menuLevel{items: item.Children, label: item.Label})
					m.resetLevel()
					return m, nil
				}
				m.selected = item.Action
			}
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	prevValue := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevValue {
		m.filtered = FilterMenuItems(m.current().items, m.input.Value())
		m.cursor = 0
	}
	return m, cmd
}

func (m menuModel) View() string {
	if m.quitting {
		return ""
	}
	var out strings.Builder
	out.WriteString(m.breadcrumb())
	out.WriteString(m.input.View())
	out.WriteString("\n")
	for i, it := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		out.WriteString(cursor)
		out.WriteString(it.Label)
		if it.isGroup() {
			out.WriteString("/")
		}
		out.WriteString("\n")
	}
	return out.String()
}

// RunActionMenu presents the action menu and returns the chosen leaf
// action, or ErrCancelled if the user quit (Esc at the root level) without
// choosing one. Esc inside a group goes back one level instead of quitting.
func RunActionMenu() (string, error) {
	p := tea.NewProgram(newMenuModel(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	result := finalModel.(menuModel)
	if result.selected == "" {
		return "", ErrCancelled
	}
	return result.selected, nil
}
