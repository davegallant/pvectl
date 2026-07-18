package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/davegallant/pvectl/internal/api"
)

// FilterVMs returns the VMs whose name contains query (case-insensitive).
// A pure function so it's testable independently of the bubbletea event
// loop.
func FilterVMs(vms []api.VM, query string) []api.VM {
	if query == "" {
		return vms
	}
	query = strings.ToLower(query)

	var out []api.VM
	for _, v := range vms {
		if strings.Contains(strings.ToLower(v.Name), query) {
			out = append(out, v)
		}
	}
	return out
}

type vmPickerModel struct {
	all          []api.VM
	filtered     []api.VM
	cursor       int
	scrollOffset int
	height       int
	input        textinput.Model
	preview      string
	fetch        PreviewFetcher
	selected     *api.VM
	quitting     bool
}

// listVisibleRows returns how many list rows fit in the terminal, reserving
// space for the search input and position counter lines above the list.
func (m vmPickerModel) listVisibleRows() int {
	h := m.height
	if h <= 0 {
		h = defaultTerminalHeight
	}
	rows := h - chromeLines
	if rows < 1 {
		rows = 1
	}
	return rows
}

func newVMPickerModel(vms []api.VM, fetch PreviewFetcher) vmPickerModel {
	ti := textinput.New()
	ti.Placeholder = "qm > "
	ti.Focus()
	return vmPickerModel{
		all:      vms,
		filtered: vms,
		input:    ti,
		fetch:    fetch,
	}
}

func (m vmPickerModel) Init() tea.Cmd {
	return m.fetchPreviewCmd()
}

func (m vmPickerModel) fetchPreviewCmd() tea.Cmd {
	if len(m.filtered) == 0 || m.fetch == nil {
		return nil
	}
	v := m.filtered[m.cursor]
	fetch := m.fetch
	return func() tea.Msg {
		text, err := fetch(v.Node, v.VMID)
		if err != nil {
			return previewMsg{text: fmt.Sprintf("preview unavailable: %v", err)}
		}
		return previewMsg{text: text}
	}
}

func (m vmPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case previewMsg:
		m.preview = msg.text
		return m, nil

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.scrollOffset = clampScrollOffset(m.scrollOffset, m.cursor, len(m.filtered), m.listVisibleRows())
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				selected := m.filtered[m.cursor]
				m.selected = &selected
			}
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			m.scrollOffset = clampScrollOffset(m.scrollOffset, m.cursor, len(m.filtered), m.listVisibleRows())
			return m, m.fetchPreviewCmd()
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			m.scrollOffset = clampScrollOffset(m.scrollOffset, m.cursor, len(m.filtered), m.listVisibleRows())
			return m, m.fetchPreviewCmd()
		}
	}

	var cmd tea.Cmd
	prevValue := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevValue {
		m.filtered = FilterVMs(m.all, m.input.Value())
		m.cursor = 0
		m.scrollOffset = 0
		return m, tea.Batch(cmd, m.fetchPreviewCmd())
	}
	return m, cmd
}

func (m vmPickerModel) View() string {
	if m.quitting {
		return ""
	}

	total := len(m.filtered)
	visibleRows := m.listVisibleRows()
	start := m.scrollOffset
	end := start + visibleRows
	if end > total {
		end = total
	}

	var list strings.Builder
	for idx := start; idx < end; idx++ {
		v := m.filtered[idx]
		cursor := "  "
		if idx == m.cursor {
			cursor = cursorStyle.Render("> ")
		}
		name := truncate(v.Name, nameColumnWidth)
		status := statusStyle(v.Status).Render(v.Status)
		fmt.Fprintf(&list, "%s%5d  %-*s  %s\n", cursor, v.VMID, nameColumnWidth, name, status)
	}

	counter := "(0/0)"
	if total > 0 {
		counter = fmt.Sprintf("(%d/%d)", m.cursor+1, total)
	}

	left := lipgloss.NewStyle().Width(50).Render(m.input.View() + "\n" + counter + "\n" + list.String())
	right := lipgloss.NewStyle().Width(50).Render(m.preview)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// RunVMPicker launches the interactive VM picker and returns the selected
// VM, or ErrCancelled if the user quit without selecting.
func RunVMPicker(vms []api.VM, fetch PreviewFetcher) (api.VM, error) {
	if len(vms) == 0 {
		return api.VM{}, fmt.Errorf("no VMs found")
	}

	m := newVMPickerModel(vms, fetch)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return api.VM{}, err
	}

	result := finalModel.(vmPickerModel)
	if result.selected == nil {
		return api.VM{}, ErrCancelled
	}
	return *result.selected, nil
}
