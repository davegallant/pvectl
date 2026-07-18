package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/davegallant/pvectl/internal/api"
)

func testContainers() []api.Container {
	return []api.Container{
		{VMID: 101, Name: "web-01", Node: "pve1", Status: "running"},
		{VMID: 102, Name: "db-01", Node: "pve1", Status: "stopped"},
		{VMID: 103, Name: "web-02", Node: "pve2", Status: "running"},
	}
}

// manyContainers returns n containers with VMIDs 100, 101, 102, ... — enough
// to exceed a small terminal height so scrolling behavior can be exercised.
func manyContainers(n int) []api.Container {
	out := make([]api.Container, n)
	for i := range out {
		out[i] = api.Container{VMID: 100 + i, Name: "ct-" + strconv.Itoa(100+i), Node: "pve1", Status: "running"}
	}
	return out
}

func TestTruncateShorterThanWidth(t *testing.T) {
	if got := truncate("web-01", 24); got != "web-01" {
		t.Errorf("truncate() = %q, want %q (unchanged)", got, "web-01")
	}
}

func TestTruncateLongerThanWidth(t *testing.T) {
	got := truncate("a-very-long-container-hostname", 10)
	want := "a-very-lo…"
	if got != want {
		t.Errorf("truncate() = %q, want %q", got, want)
	}
}

func TestFilterContainersEmptyQuery(t *testing.T) {
	got := FilterContainers(testContainers(), "")
	if len(got) != 3 {
		t.Errorf("FilterContainers with empty query returned %d, want 3", len(got))
	}
}

func TestFilterContainersMatchesSubstring(t *testing.T) {
	got := FilterContainers(testContainers(), "web")
	if len(got) != 2 {
		t.Fatalf(`FilterContainers("web") returned %d, want 2`, len(got))
	}
	if got[0].Name != "web-01" || got[1].Name != "web-02" {
		t.Errorf(`FilterContainers("web") = %+v`, got)
	}
}

func TestFilterContainersCaseInsensitive(t *testing.T) {
	got := FilterContainers(testContainers(), "WEB-01")
	if len(got) != 1 || got[0].Name != "web-01" {
		t.Errorf(`FilterContainers("WEB-01") = %+v, want [web-01]`, got)
	}
}

func TestFilterContainersNoMatch(t *testing.T) {
	got := FilterContainers(testContainers(), "nonexistent")
	if len(got) != 0 {
		t.Errorf(`FilterContainers("nonexistent") returned %d, want 0`, len(got))
	}
}

func TestPickerModelCursorDown(t *testing.T) {
	m := newPickerModel(testContainers(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(pickerModel)

	if pm.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d, want 1", pm.cursor)
	}
}

func TestPickerModelCursorStaysInBounds(t *testing.T) {
	m := newPickerModel(testContainers(), nil)
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(pickerModel)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.cursor)
	}
}

func TestPickerModelEnterSelects(t *testing.T) {
	m := newPickerModel(testContainers(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(pickerModel)

	if pm.selected == nil || pm.selected.VMID != 101 {
		t.Errorf("selected = %+v, want vmid 101", pm.selected)
	}
}

func TestPickerModelEscCancels(t *testing.T) {
	m := newPickerModel(testContainers(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm := updated.(pickerModel)

	if pm.selected != nil {
		t.Error("selected should be nil after Esc")
	}
	if !pm.quitting {
		t.Error("quitting should be true after Esc")
	}
}

func TestClampScrollOffsetCursorAboveWindowScrollsUp(t *testing.T) {
	// cursor moved above the current window (e.g. jumped via KeyUp) — offset
	// must move up to exactly the cursor, not recenter.
	if got := clampScrollOffset(5, 3, 20, 5); got != 3 {
		t.Errorf("clampScrollOffset() = %d, want 3", got)
	}
}

func TestClampScrollOffsetCursorBelowWindowScrollsDown(t *testing.T) {
	// cursor at index 10 with a 5-row window starting at 0 is out of view;
	// offset must shift by the minimum amount to bring it into view.
	if got := clampScrollOffset(0, 10, 20, 5); got != 6 {
		t.Errorf("clampScrollOffset() = %d, want 6", got)
	}
}

func TestClampScrollOffsetCursorWithinWindowUnchanged(t *testing.T) {
	if got := clampScrollOffset(4, 6, 20, 5); got != 4 {
		t.Errorf("clampScrollOffset() = %d, want 4 (unchanged, cursor already visible)", got)
	}
}

func TestClampScrollOffsetNeverExceedsMaxOffset(t *testing.T) {
	// total shrank (e.g. filter narrowed results) below offset+visibleRows.
	if got := clampScrollOffset(10, 2, 6, 5); got != 1 {
		t.Errorf("clampScrollOffset() = %d, want 1 (clamped to total-visibleRows)", got)
	}
}

func TestClampScrollOffsetNeverNegative(t *testing.T) {
	if got := clampScrollOffset(0, 0, 3, 5); got != 0 {
		t.Errorf("clampScrollOffset() = %d, want 0", got)
	}
}

func TestPickerViewLimitsRowsToWindowHeight(t *testing.T) {
	m := newPickerModel(manyContainers(20), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	pm := updated.(pickerModel)

	view := pm.View()
	if strings.Count(view, "ct-119") > 0 {
		t.Errorf("View() with a 10-row terminal and 20 containers should not render the last container yet")
	}
	if !strings.Contains(view, "ct-100") {
		t.Errorf("View() = %q, want first container ct-100 visible at cursor 0", view)
	}
}

func TestPickerViewScrollsAsCursorMovesDown(t *testing.T) {
	m := newPickerModel(manyContainers(20), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	pm := updated.(pickerModel)

	// Move the cursor past the bottom of an 8-row visible window
	// (10 terminal rows - 2 chrome lines) so the window must scroll.
	for i := 0; i < 15; i++ {
		next, _ := pm.Update(tea.KeyMsg{Type: tea.KeyDown})
		pm = next.(pickerModel)
	}

	view := pm.View()
	if strings.Contains(view, "ct-100") {
		t.Errorf("View() after scrolling down 15 rows should no longer show ct-100 (scrolled off top)")
	}
	if !strings.Contains(view, "ct-115") {
		t.Errorf("View() after scrolling down 15 rows should show the current cursor row ct-115:\n%s", view)
	}
}

func TestPickerViewShowsPositionCounter(t *testing.T) {
	m := newPickerModel(testContainers(), nil)

	view := m.View()
	if !strings.Contains(view, "(1/3)") {
		t.Errorf("View() = %q, want it to contain position counter (1/3)", view)
	}
}

func TestPickerViewPositionCounterTracksCursor(t *testing.T) {
	m := newPickerModel(testContainers(), nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(pickerModel)

	view := pm.View()
	if !strings.Contains(view, "(2/3)") {
		t.Errorf("View() = %q, want it to contain position counter (2/3)", view)
	}
}

func TestPickerViewPositionCounterEmptyList(t *testing.T) {
	m := newPickerModel(testContainers(), nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nonexistent")})
	pm := updated.(pickerModel)

	view := pm.View()
	if !strings.Contains(view, "(0/0)") {
		t.Errorf("View() = %q, want it to contain position counter (0/0) when filter matches nothing", view)
	}
}
