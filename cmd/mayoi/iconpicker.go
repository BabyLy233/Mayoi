package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"mayoi/internal/workspace"
	"mayoi/locales"
)

// iconPicker is the box of the file icon themes. The window takes each
// theme as the choice moves to it: Return keeps it, and Escape goes back
// to the theme the box opened on.
type iconPicker struct {
	open  bool
	query string
	sel   int
	moved bool
	// was is the setting as the box opened.
	was     string
	results []workspace.Match // the themes the query matches
	shown   string            // the query the results are of
	stale   bool
}

// openIconPicker shows the box of the file icon themes, on the window's.
func (a *app) openIconPicker() {
	a.icons = iconPicker{open: true, stale: true, was: a.settings.FileIconTheme}
}

// closeIconPicker hides the box: keep keeps the theme it shows, or else
// the windows go back to the theme the box opened on.
func (a *app) closeIconPicker(keep bool) {
	ip := &a.icons
	if !ip.open {
		return
	}
	ip.open = false
	if !keep && a.settings.FileIconTheme != ip.was {
		a.settings.FileIconTheme = ip.was
		a.host.redrawWindows()
	}
	if keep && a.settings.FileIconTheme != ip.was {
		a.settings.Changed()
	}
	if keep && a.settings.FileIconTheme != "" && cachedIconTheme(a.settings.FileIconTheme) == nil {
		a.fail("%s", locales.FileIconThemeFailed(a.settings.FileIconTheme))
	}
	if a.focus != paneTerminal {
		a.focusColumn()
	}
}

// previewIconChoice shows the windows in the theme of a row of the
// results, without writing it in the settings.
func (a *app) previewIconChoice(i int) {
	ip := &a.icons
	choices := allFileIconThemes()
	if i < 0 || i >= len(ip.results) || ip.results[i].Index >= len(choices) {
		return
	}
	id := choices[ip.results[i].Index].id
	if a.settings.FileIconTheme == id {
		return
	}
	a.settings.FileIconTheme = id
	a.host.redrawWindows()
}

// iconPickerView shows the box of the file icon themes while it is open.
func (a *app) iconPickerView(c *ui.Context) {
	ip := &a.icons
	if !ip.open {
		return
	}
	p := a.pal
	choices := allFileIconThemes()
	if ip.stale || ip.shown != ip.query {
		q := strings.TrimSpace(ip.query)
		names := make([]string, len(choices))
		for i, ch := range choices {
			names[i] = ch.name
		}
		ip.results = workspace.Fuzzy(names, q, len(names))
		if q == "" {
			ip.results = ip.results[:0]
			for i := range names {
				ip.results = append(ip.results, workspace.Match{Index: i})
			}
		}
		// The row of the theme shown, or else the first, which the window
		// then shows.
		sel := -1
		for i, m := range ip.results {
			if m.Index < len(choices) && choices[m.Index].id == a.settings.FileIconTheme {
				sel = i
			}
		}
		if ip.sel = max(sel, 0); sel < 0 {
			a.previewIconChoice(0)
		}
		// The chosen row comes into view, as the box opens or the query changes.
		ip.moved = true
		ip.shown, ip.stale = ip.query, false
	}
	ui.Overlay(c, func() {
		back := a.dialogBack(c)
		back.Children(func() {
			box := a.floating(c, ui.Column(c).Width(520).MaxWidthPercent(92).Margin(titleBarH+8, 0, 0, 0).Radius(10), true).Clip().
				Role(ui.RoleDialog).Label(prefTitle("fileIconTheme"))
			box.Children(func() {
				ui.Row(c).Padding(10, 14).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
					ui.Icon(c, iconFile).FontSize(15).TextColor(p.muted)
					in := ui.TextInputBase(c.Key("icon-theme-input"), &ip.query)
					if a.chording() {
						in.ReadOnly(true)
					}
					in.Grow(1).FontSize(14).AutoFocus().
						Placeholder(locales.FileIconThemePlaceholder()).Label(prefTitle("fileIconTheme"))
					move := func(to int) {
						to = min(max(to, 0), len(ip.results)-1)
						if to >= 0 && to != ip.sel {
							ip.sel, ip.moved = to, true
							a.previewIconChoice(to)
						}
					}
					switch {
					case in.Shortcut(0, ui.KeyDown):
						move(ip.sel + 1)
					case in.Shortcut(0, ui.KeyUp):
						move(ip.sel - 1)
					case in.Submitted():
						a.post(func() { a.closeIconPicker(true) })
					}
				})
				if len(ip.results) > 0 {
					ui.Scroll(c.Key("icon-theme-results")).MaxHeight(380).Padding(6).Children(func() {
						for i := range ip.results {
							a.iconPickerRow(c, i, choices)
						}
					})
				} else {
					ui.Text(c, locales.FileIconThemeNoMatches()).FontSize(13).TextColor(p.muted).Padding(12, 16)
				}
				ip.moved = false
				a.iconPickerFooter(c)
			})
			if box.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				a.closeIconPicker(false)
			}
		})
	})
}

// iconPickerRow is a row of the box: the theme's icon of a file, its name
// with the characters the query matched in the accent color, and what the
// theme is.
func (a *app) iconPickerRow(c *ui.Context, i int, choices []iconThemeChoice) {
	ip := &a.icons
	p := a.pal
	m := ip.results[i]
	if m.Index >= len(choices) {
		return
	}
	ch := choices[m.Index]
	row := ui.ButtonBase(c.Key("icon-theme:"+ch.id)).Padding(6, 8).Gap(12).Radius(8).Justify(ui.Start).AlignItems(ui.Center).
		Label(ch.name)
	if i != ip.sel && row.Hovered() {
		row.Background(p.hover)
	}
	if i == ip.sel && ip.moved {
		row.ScrollIntoView()
	}
	if row.Clicked() {
		a.post(func() {
			ip.sel = i
			a.previewIconChoice(i)
			a.closeIconPicker(true)
		})
	}
	row.Transition(hoverFade)
	row.Children(func() {
		a.pickMark(c, i == ip.sel)
		a.fileIconFor(ch.id, "example.unknown").element(c, 16)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.RichText(c, a.matchSpans(ch.name, 0, m.Positions, p.text)...).FontSize(13.5).FontWeight(550).SingleLine()
			ui.Text(c, ch.about).FontSize(12).TextColor(p.muted).SingleLine().Shrink(1)
		})
		if ch.id == ip.was {
			ui.Icon(c.Key("icon-theme-check"), iconCheck).FontSize(14).TextColor(p.accent).Transition(dotPop)
		}
	})
}

// iconPickerFooter is the box's foot, with its keys.
func (a *app) iconPickerFooter(c *ui.Context) {
	p := a.pal
	ui.Row(c).Height(30).Padding(0, 14).Gap(14).AlignItems(ui.Center).Background(p.hover).
		BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
		hint := func(keys, label string) {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				a.keycaps(c, keys)
				ui.Text(c, label).FontSize(11.5).TextColor(p.muted).SingleLine()
			})
		}
		hint("↑↓", locales.QuickNavigate())
		hint("⏎", locales.ThemeKeep())
		hint("esc", locales.QuickClose())
	})
}
