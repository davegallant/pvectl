package editconf

import (
	"fmt"
	"sort"
	"strings"
)

// render converts config fields into sorted "key: value" lines, applying
// valueFn to each value first (e.g. to truncate it).
func render(fields map[string]string, valueFn func(string) string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, valueFn(fields[k]))
	}
	return b.String()
}

func identity(s string) string { return s }

// Render converts config fields into sorted "key: value" lines, matching
// the shape of a real Proxmox /etc/pve/lxc/<vmid>.conf file.
func Render(fields map[string]string) string {
	return render(fields, identity)
}

// maxPreviewValueLen caps how many runes of a single field's value are
// shown by RenderPreview, so one oversized field (e.g. a community-script
// installer's HTML "notes" field) can't crowd out every other field in
// the picker's preview pane.
const maxPreviewValueLen = 200

// RenderPreview is like Render, but truncates each field's value to
// maxPreviewValueLen runes. It's for the picker's live preview pane only —
// $EDITOR and the diff/PUT path always use the untruncated Render, so a
// long value is never silently clipped when actually editing or saving.
func RenderPreview(fields map[string]string) string {
	return render(fields, func(v string) string {
		return truncateValue(v, maxPreviewValueLen)
	})
}

func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "…"
}

// Parse converts "key: value" lines back into a map, ignoring blank lines
// and lines without a colon.
func Parse(text string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		fields[key] = value
	}
	return fields
}

// Diff describes what changed between an original and edited field map.
type Diff struct {
	Changed map[string]string
	Removed []string
}

// DiffFields returns the keys that were added or changed, and the keys
// that were removed, between original and edited.
func DiffFields(original, edited map[string]string) Diff {
	diff := Diff{Changed: make(map[string]string)}

	for k, v := range edited {
		if orig, ok := original[k]; !ok || orig != v {
			diff.Changed[k] = v
		}
	}
	for k := range original {
		if _, ok := edited[k]; !ok {
			diff.Removed = append(diff.Removed, k)
		}
	}
	sort.Strings(diff.Removed)
	return diff
}
