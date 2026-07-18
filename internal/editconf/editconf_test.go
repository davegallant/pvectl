package editconf

import (
	"reflect"
	"strings"
	"testing"
)

func TestRenderParseRoundTrip(t *testing.T) {
	fields := map[string]string{
		"hostname": "web01",
		"cores":    "2",
		"memory":   "512",
	}

	text := Render(fields)
	got := Parse(text)

	if !reflect.DeepEqual(got, fields) {
		t.Errorf("Parse(Render(fields)) = %+v, want %+v", got, fields)
	}
}

func TestRenderIsSorted(t *testing.T) {
	fields := map[string]string{"zeta": "1", "alpha": "2"}
	want := "alpha: 2\nzeta: 1\n"
	if got := Render(fields); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRenderPreviewTruncatesLongValues(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "x"
	}
	fields := map[string]string{"description": long, "cores": "2"}

	got := RenderPreview(fields)

	if !strings.Contains(got, "cores: 2\n") {
		t.Errorf("RenderPreview() = %q, want it to still contain the short cores field in full", got)
	}
	if strings.Contains(got, long) {
		t.Error("RenderPreview() contains the full 500-char value, want it truncated")
	}
	wantPrefix := "description: " + long[:200] + "…"
	if !strings.Contains(got, wantPrefix) {
		t.Errorf("RenderPreview() = %q, want it to contain %q", got, wantPrefix)
	}
}

func TestRenderPreviewLeavesShortValuesUnchanged(t *testing.T) {
	fields := map[string]string{"hostname": "web01"}
	want := "hostname: web01\n"
	if got := RenderPreview(fields); got != want {
		t.Errorf("RenderPreview() = %q, want %q", got, want)
	}
}

func TestParseIgnoresBlankAndMalformedLines(t *testing.T) {
	text := "hostname: web01\n\n# not a real comment, just noise\nno-colon-here\ncores: 2\n"
	got := Parse(text)
	want := map[string]string{"hostname": "web01", "cores": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %+v, want %+v", got, want)
	}
}

func TestDiffFields(t *testing.T) {
	original := map[string]string{"hostname": "web01", "cores": "2", "memory": "512"}
	edited := map[string]string{"hostname": "web01", "cores": "4", "swap": "256"}

	diff := DiffFields(original, edited)

	wantChanged := map[string]string{"cores": "4", "swap": "256"}
	if !reflect.DeepEqual(diff.Changed, wantChanged) {
		t.Errorf("Changed = %+v, want %+v", diff.Changed, wantChanged)
	}

	wantRemoved := []string{"memory"}
	if !reflect.DeepEqual(diff.Removed, wantRemoved) {
		t.Errorf("Removed = %+v, want %+v", diff.Removed, wantRemoved)
	}
}

func TestDiffFieldsNoChanges(t *testing.T) {
	fields := map[string]string{"hostname": "web01"}
	diff := DiffFields(fields, map[string]string{"hostname": "web01"})
	if len(diff.Changed) != 0 || len(diff.Removed) != 0 {
		t.Errorf("Diff = %+v, want empty", diff)
	}
}
