package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/davegallant/pvectl/internal/api"
)

// ErrCancelled is returned when the user quits a TUI screen without making
// a selection.
var ErrCancelled = errors.New("selection cancelled")

// FilterContainers returns the containers whose name contains query
// (case-insensitive). A pure function so it's testable independently of
// the bubbletea event loop.
func FilterContainers(containers []api.Container, query string) []api.Container {
	if query == "" {
		return containers
	}
	query = strings.ToLower(query)

	var out []api.Container
	for _, c := range containers {
		if strings.Contains(strings.ToLower(c.Name), query) {
			out = append(out, c)
		}
	}
	return out
}

// PreviewFetcher fetches the preview text for a highlighted container.
type PreviewFetcher func(node string, vmid int) (string, error)

const nameColumnWidth = 24

// defaultTerminalHeight is used to size the visible list window before the
// first tea.WindowSizeMsg arrives (bubbletea sends one on startup, but
// View() can render before it's processed).
const defaultTerminalHeight = 24

// chromeLines is the number of lines the picker's left column spends on
// non-list content (the search input and the position counter), which
// must be subtracted from the terminal height to get the list's row budget.
const chromeLines = 2

// clampScrollOffset returns the scroll offset that keeps cursor within the
// visible window of visibleRows items, shifting the previous offset by the
// minimum amount needed to bring cursor into view (rather than
// recentering), matching the scroll feel of fuzzy-finders like fzf.
func clampScrollOffset(offset, cursor, total, visibleRows int) int {
	if visibleRows <= 0 {
		return 0
	}
	if cursor < offset {
		offset = cursor
	} else if cursor >= offset+visibleRows {
		offset = cursor - visibleRows + 1
	}
	maxOffset := total - visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

var (
	cursorStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	statusRunningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	statusStoppedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusOtherStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return statusRunningStyle
	case "stopped":
		return statusStoppedStyle
	default:
		return statusOtherStyle
	}
}

// truncate shortens s to at most width runes, marking with an ellipsis if
// it had to cut anything, so long container names don't blow out the
// column alignment of the rest of the list. Operates on runes, not bytes,
// so a multi-byte name is never cut mid-rune.
func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}

type previewMsg struct{ text string }

type pickerModel struct {
	all          []api.Container
	filtered     []api.Container
	cursor       int
	scrollOffset int
	height       int
	input        textinput.Model
	preview      string
	fetch        PreviewFetcher
	selected     *api.Container
	quitting     bool
}

// listVisibleRows returns how many list rows fit in the terminal, reserving
// space for the search input and position counter lines above the list.
func (m pickerModel) listVisibleRows() int {
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

func newPickerModel(containers []api.Container, fetch PreviewFetcher) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "ct > "
	ti.Focus()
	return pickerModel{
		all:      containers,
		filtered: containers,
		input:    ti,
		fetch:    fetch,
	}
}

func (m pickerModel) Init() tea.Cmd {
	return m.fetchPreviewCmd()
}

func (m pickerModel) fetchPreviewCmd() tea.Cmd {
	if len(m.filtered) == 0 || m.fetch == nil {
		return nil
	}
	c := m.filtered[m.cursor]
	fetch := m.fetch
	return func() tea.Msg {
		text, err := fetch(c.Node, c.VMID)
		if err != nil {
			return previewMsg{text: fmt.Sprintf("preview unavailable: %v", err)}
		}
		return previewMsg{text: text}
	}
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.filtered = FilterContainers(m.all, m.input.Value())
		m.cursor = 0
		m.scrollOffset = 0
		return m, tea.Batch(cmd, m.fetchPreviewCmd())
	}
	return m, cmd
}

func (m pickerModel) View() string {
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
		c := m.filtered[idx]
		cursor := "  "
		if idx == m.cursor {
			cursor = cursorStyle.Render("> ")
		}
		name := truncate(c.Name, nameColumnWidth)
		status := statusStyle(c.Status).Render(c.Status)
		fmt.Fprintf(&list, "%s%5d  %-*s  %s\n", cursor, c.VMID, nameColumnWidth, name, status)
	}

	counter := "(0/0)"
	if total > 0 {
		counter = fmt.Sprintf("(%d/%d)", m.cursor+1, total)
	}

	left := lipgloss.NewStyle().Width(50).Render(m.input.View() + "\n" + counter + "\n" + list.String())
	right := lipgloss.NewStyle().Width(50).Render(m.preview)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// RunPicker launches the interactive container picker and returns the
// selected container, or ErrCancelled if the user quit without selecting.
func RunPicker(containers []api.Container, fetch PreviewFetcher) (api.Container, error) {
	if len(containers) == 0 {
		return api.Container{}, fmt.Errorf("no containers found")
	}

	m := newPickerModel(containers, fetch)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return api.Container{}, err
	}

	result := finalModel.(pickerModel)
	if result.selected == nil {
		return api.Container{}, ErrCancelled
	}
	return *result.selected, nil
}
