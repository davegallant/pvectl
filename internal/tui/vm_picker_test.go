package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/davegallant/pvectl/internal/api"
)

func testVMs() []api.VM {
	return []api.VM{
		{VMID: 201, Name: "web-01", Node: "pve1", Status: "running"},
		{VMID: 202, Name: "db-01", Node: "pve1", Status: "stopped"},
		{VMID: 203, Name: "web-02", Node: "pve2", Status: "running"},
	}
}

// manyVMs returns n VMs with VMIDs 200, 201, 202, ... — enough to exceed a
// small terminal height so scrolling behavior can be exercised.
func manyVMs(n int) []api.VM {
	out := make([]api.VM, n)
	for i := range out {
		out[i] = api.VM{VMID: 200 + i, Name: "vm-" + strconv.Itoa(200+i), Node: "pve1", Status: "running"}
	}
	return out
}

func TestFilterVMsEmptyQuery(t *testing.T) {
	got := FilterVMs(testVMs(), "")
	if len(got) != 3 {
		t.Errorf("FilterVMs with empty query returned %d, want 3", len(got))
	}
}

func TestFilterVMsMatchesSubstring(t *testing.T) {
	got := FilterVMs(testVMs(), "web")
	if len(got) != 2 {
		t.Fatalf(`FilterVMs("web") returned %d, want 2`, len(got))
	}
	if got[0].Name != "web-01" || got[1].Name != "web-02" {
		t.Errorf(`FilterVMs("web") = %+v`, got)
	}
}

func TestFilterVMsCaseInsensitive(t *testing.T) {
	got := FilterVMs(testVMs(), "WEB-01")
	if len(got) != 1 || got[0].Name != "web-01" {
		t.Errorf(`FilterVMs("WEB-01") = %+v, want [web-01]`, got)
	}
}

func TestFilterVMsNoMatch(t *testing.T) {
	got := FilterVMs(testVMs(), "nonexistent")
	if len(got) != 0 {
		t.Errorf(`FilterVMs("nonexistent") returned %d, want 0`, len(got))
	}
}

func TestVMPickerModelCursorDown(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(vmPickerModel)

	if pm.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d, want 1", pm.cursor)
	}
}

func TestVMPickerModelCursorStaysInBounds(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(vmPickerModel)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.cursor)
	}
}

func TestVMPickerModelEnterSelects(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(vmPickerModel)

	if pm.selected == nil || pm.selected.VMID != 201 {
		t.Errorf("selected = %+v, want vmid 201", pm.selected)
	}
}

func TestVMPickerModelEscCancels(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm := updated.(vmPickerModel)

	if pm.selected != nil {
		t.Error("selected should be nil after Esc")
	}
	if !pm.quitting {
		t.Error("quitting should be true after Esc")
	}
}

func TestVMPickerViewLimitsRowsToWindowHeight(t *testing.T) {
	m := newVMPickerModel(manyVMs(20), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	pm := updated.(vmPickerModel)

	view := pm.View()
	if strings.Count(view, "vm-219") > 0 {
		t.Errorf("View() with a 10-row terminal and 20 VMs should not render the last VM yet")
	}
	if !strings.Contains(view, "vm-200") {
		t.Errorf("View() = %q, want first VM vm-200 visible at cursor 0", view)
	}
}

func TestVMPickerViewScrollsAsCursorMovesDown(t *testing.T) {
	m := newVMPickerModel(manyVMs(20), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	pm := updated.(vmPickerModel)

	// Move the cursor past the bottom of an 8-row visible window
	// (10 terminal rows - 2 chrome lines) so the window must scroll.
	for i := 0; i < 15; i++ {
		next, _ := pm.Update(tea.KeyMsg{Type: tea.KeyDown})
		pm = next.(vmPickerModel)
	}

	view := pm.View()
	if strings.Contains(view, "vm-200") {
		t.Errorf("View() after scrolling down 15 rows should no longer show vm-200 (scrolled off top)")
	}
	if !strings.Contains(view, "vm-215") {
		t.Errorf("View() after scrolling down 15 rows should show the current cursor row vm-215:\n%s", view)
	}
}

func TestVMPickerViewShowsPositionCounter(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)

	view := m.View()
	if !strings.Contains(view, "(1/3)") {
		t.Errorf("View() = %q, want it to contain position counter (1/3)", view)
	}
}

func TestVMPickerViewPositionCounterTracksCursor(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(vmPickerModel)

	view := pm.View()
	if !strings.Contains(view, "(2/3)") {
		t.Errorf("View() = %q, want it to contain position counter (2/3)", view)
	}
}

func TestVMPickerViewPositionCounterEmptyList(t *testing.T) {
	m := newVMPickerModel(testVMs(), nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nonexistent")})
	pm := updated.(vmPickerModel)

	view := pm.View()
	if !strings.Contains(view, "(0/0)") {
		t.Errorf("View() = %q, want it to contain position counter (0/0) when filter matches nothing", view)
	}
}
