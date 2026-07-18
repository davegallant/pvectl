package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterMenuItemsEmptyQuery(t *testing.T) {
	got := FilterMenuItems(ActionTree, "")
	if len(got) != len(ActionTree) {
		t.Errorf("FilterMenuItems with empty query returned %d, want %d", len(got), len(ActionTree))
	}
}

func TestFilterMenuItemsMatchesSubstring(t *testing.T) {
	got := FilterMenuItems(ActionTree, "sta")
	if len(got) != 1 || got[0].Label != "start" {
		t.Errorf(`FilterMenuItems("sta") = %v, want [start]`, got)
	}
}

func TestFilterMenuItemsCaseInsensitive(t *testing.T) {
	got := FilterMenuItems(ActionTree, "ENTER")
	if len(got) != 1 || got[0].Label != "enter" {
		t.Errorf(`FilterMenuItems("ENTER") = %v, want [enter]`, got)
	}
}

func TestFilterMenuItemsNoMatch(t *testing.T) {
	got := FilterMenuItems(ActionTree, "nonexistent")
	if len(got) != 0 {
		t.Errorf(`FilterMenuItems("nonexistent") returned %d, want 0`, len(got))
	}
}

// TestFilterMenuItemsWithinGroup confirms filtering works the same way one
// level down — RunActionMenu re-filters against the current group's
// Children, not always the root ActionTree.
func TestFilterMenuItemsWithinGroup(t *testing.T) {
	var snapshots menuItem
	for _, it := range ActionTree {
		if it.Label == "snapshots" {
			snapshots = it
		}
	}
	if !snapshots.isGroup() {
		t.Fatal("expected \"snapshots\" to be a group in ActionTree")
	}

	got := FilterMenuItems(snapshots.Children, "del")
	if len(got) != 1 || got[0].Label != "delete" {
		t.Errorf(`FilterMenuItems(snapshots.Children, "del") = %v, want [delete]`, got)
	}
}

func TestMenuModelTypingFiltersAndResetsCursor(t *testing.T) {
	m := newMenuModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(menuModel)
	if pm.cursor != 1 {
		t.Fatalf("cursor after KeyDown = %d, want 1", pm.cursor)
	}

	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("stop")})
	pm = updated.(menuModel)
	if len(pm.filtered) != 1 || pm.filtered[0].Label != "stop" {
		t.Fatalf("filtered = %v, want [stop]", pm.filtered)
	}
	if pm.cursor != 0 {
		t.Errorf("cursor after filtering = %d, want 0 (reset)", pm.cursor)
	}

	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = updated.(menuModel)
	if pm.selected != "stop" {
		t.Errorf("selected = %q, want %q", pm.selected, "stop")
	}
}

func TestMenuModelCursorMovesAndSelects(t *testing.T) {
	m := newMenuModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(menuModel)
	if pm.cursor != 1 {
		t.Fatalf("cursor after KeyDown = %d, want 1", pm.cursor)
	}

	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = updated.(menuModel)
	if pm.selected != ActionTree[1].Action {
		t.Errorf("selected = %q, want %q", pm.selected, ActionTree[1].Action)
	}
}

func TestMenuModelCursorStaysInBounds(t *testing.T) {
	m := newMenuModel()
	for i := 0; i < len(ActionTree)+5; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(menuModel)
	}
	if m.cursor != len(ActionTree)-1 {
		t.Errorf("cursor = %d, want %d (clamped to last item)", m.cursor, len(ActionTree)-1)
	}
}

func TestMenuModelEscCancelsAtRoot(t *testing.T) {
	m := newMenuModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm := updated.(menuModel)
	if pm.selected != "" {
		t.Error("selected should be empty after Esc")
	}
	if !pm.quitting {
		t.Error("quitting should be true after Esc at the root level")
	}
}

// moveCursorTo returns m with the cursor moved to the item labeled label
// within m.filtered, failing the test if it isn't found.
func moveCursorTo(t *testing.T, m menuModel, label string) menuModel {
	t.Helper()
	for i, it := range m.filtered {
		if it.Label == label {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("label %q not found in filtered items", label)
	return m
}

// TestMenuModelEnterConfigThenSelectEdit confirms "edit" is reached via
// the "config" group (matching `ct config edit`/`qm config edit`), not as
// a root-level leaf the way it used to be.
func TestMenuModelEnterConfigThenSelectEdit(t *testing.T) {
	m := newMenuModel()
	m = moveCursorTo(t, m, "config")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(menuModel)
	if len(pm.filtered) != 1 || pm.filtered[0].Label != "edit" {
		t.Fatalf(`filtered after entering "config" = %v, want [edit]`, pm.filtered)
	}

	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = updated.(menuModel)
	if pm.selected != "edit" {
		t.Errorf(`selected = %q, want "edit"`, pm.selected)
	}
}

func TestMenuModelEnterGroupThenSelectLeaf(t *testing.T) {
	m := newMenuModel()
	m = moveCursorTo(t, m, "snapshots")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(menuModel)
	if pm.selected != "" {
		t.Fatalf("selected = %q after entering a group, want empty (should drill in, not select)", pm.selected)
	}
	if pm.quitting {
		t.Fatal("quitting = true after entering a group, want false")
	}
	if len(pm.filtered) != 4 {
		t.Fatalf("filtered after entering \"snapshots\" = %d items, want 4 (create/list/delete/rollback)", len(pm.filtered))
	}

	pm = moveCursorTo(t, pm, "delete")
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = updated.(menuModel)
	if pm.selected != "delete-snapshot" {
		t.Errorf(`selected = %q, want "delete-snapshot"`, pm.selected)
	}
}

func TestMenuModelBreadcrumbInsideGroup(t *testing.T) {
	m := newMenuModel()
	m = moveCursorTo(t, m, "backups")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(menuModel)
	if got := pm.breadcrumb(); got != "backups > " {
		t.Errorf(`breadcrumb() = %q, want "backups > "`, got)
	}
}

func TestMenuModelEscGoesBackFromGroupInsteadOfCancelling(t *testing.T) {
	m := newMenuModel()
	m = moveCursorTo(t, m, "snapshots")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(menuModel)

	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm = updated.(menuModel)
	if pm.quitting {
		t.Fatal("quitting = true after Esc inside a group, want false (should go back a level)")
	}
	if len(pm.filtered) != len(ActionTree) {
		t.Errorf("filtered after Esc-ing out of a group = %d items, want %d (back at root)", len(pm.filtered), len(ActionTree))
	}
	if pm.breadcrumb() != "" {
		t.Errorf("breadcrumb() after Esc back to root = %q, want empty", pm.breadcrumb())
	}
}
