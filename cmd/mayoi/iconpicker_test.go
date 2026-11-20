package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The box of the file icon themes opens on the theme shown, and Escape
// goes back to it.
func TestIconPickerOpensAndGoesBack(t *testing.T) {
	hs := newHarness(t, true)
	a := hs.a
	a.run("view.selectFileIcons")
	hs.settle()
	if !a.icons.open || a.icons.sel != 0 {
		t.Fatalf("the box is not open on Mayoi's icons: open %v, row %d", a.icons.open, a.icons.sel)
	}
	hs.key(0, ui.KeyEscape)
	hs.settle()
	if a.icons.open || a.settings.FileIconTheme != "" {
		t.Errorf("Escape leaves the box open or the theme %q", a.settings.FileIconTheme)
	}
}
