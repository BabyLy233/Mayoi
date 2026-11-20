package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"mayoi/internal/keymap"
	"mayoi/internal/plugin"
	"mayoi/locales"
)

// File icon themes: a plugin brings VS Code's file icon theme JSON and
// the SVGs it names, and the Explorer, tabs, Quick Open, search, Source
// Control and breadcrumbs draw those icons. A theme that leaves a file
// out falls back to fileIcon's own icons.

const (
	maxIconThemeBytes = 1 << 20
	maxIconSVGBytes   = 256 << 10
)

// fileIconPaint is how an icon is drawn: its shape, the color of a mono
// icon or of a fallback, and whether the SVG carries colors of its own
// and is drawn as an image.
type fileIconPaint struct {
	svg   *ui.SVG
	color ui.Color
	own   bool
}

// element is the icon as a MyGo element of size, an icon in its color or
// an image in its own.
func (ip fileIconPaint) element(c *ui.Context, size float32) ui.Element {
	if ip.own {
		return ui.Image(c, ip.svg).Size(size, size)
	}
	return ui.Icon(c, ip.svg).FontSize(size).TextColor(ip.color)
}

// paint draws the icon in r, as a diff's painter draws one.
func (ip fileIconPaint) paint(pt *ui.Painter, r ui.Rect) {
	if ip.own {
		pt.Image(ip.svg, r, ui.Contain)
		return
	}
	pt.Icon(ip.svg, r, ip.color)
}

// iconThemeDef is one entry of a theme's iconDefinitions: the SVG of an
// icon, or a font character the theme needs and Mayoi does not draw.
type iconThemeDef struct {
	IconPath      string `json:"iconPath"`
	FontCharacter string `json:"fontCharacter"`
}

// iconThemeFile is VS Code's file icon theme, of the keys Mayoi reads.
type iconThemeFile struct {
	IconDefinitions     map[string]iconThemeDef `json:"iconDefinitions"`
	File                string                  `json:"file"`
	Folder              string                  `json:"folder"`
	FolderExpanded      string                  `json:"folderExpanded"`
	FileNames           map[string]string       `json:"fileNames"`
	FileExtensions      map[string]string       `json:"fileExtensions"`
	FolderNames         map[string]string       `json:"folderNames"`
	FolderNamesExpanded map[string]string       `json:"folderNamesExpanded"`
	Light               *iconThemeFile          `json:"light"`
}

// loadedIconTheme is a theme read from a plugin's folder, with the SVGs
// parsed so far.
type loadedIconTheme struct {
	fsys fs.FS
	dir  string
	dark iconThemeFile
	// light is dark with the theme's light object over it.
	light iconThemeFile
	svgs  map[string]*iconThemeSVG
	// warn writes a line in the plugin's log, nil where there is none.
	warn func(format string, args ...any)
}

// iconThemeSVG is an SVG of a theme, and whether it carries colors of its
// own.
type iconThemeSVG struct {
	svg *ui.SVG
	own bool
}

// pluginIconTheme is a theme a plugin contributes.
type pluginIconTheme struct {
	plugin  *plugin.Plugin
	contrib plugin.FileIconThemeContrib
}

// pluginIconThemes are the file icon themes of the plugins that are on.
func pluginIconThemes() []pluginIconTheme {
	if plugins == nil || plugins.reg == nil {
		return nil
	}
	var out []pluginIconTheme
	for _, p := range plugins.enabled() {
		for _, t := range p.Contributes.FileIconThemes {
			out = append(out, pluginIconTheme{p, t})
		}
	}
	return out
}

// iconThemeCache keeps the themes read from the plugins, by id, until the
// plugins change.
var iconThemeCache = struct {
	sync.Mutex
	gen    uint64
	themes map[string]*loadedIconTheme
}{themes: map[string]*loadedIconTheme{}}

// pluginGen counts the changes of the plugins and their contributions.
func pluginGen() uint64 {
	if plugins == nil {
		return 0
	}
	plugins.mu.Lock()
	defer plugins.mu.Unlock()
	return plugins.gen
}

// cachedIconTheme is the theme of id, read once per generation of the
// plugins, nil when no plugin on contributes it.
func cachedIconTheme(id string) *loadedIconTheme {
	gen := pluginGen()
	iconThemeCache.Lock()
	if iconThemeCache.gen != gen {
		iconThemeCache.gen, iconThemeCache.themes = gen, map[string]*loadedIconTheme{}
	}
	if t, ok := iconThemeCache.themes[id]; ok {
		iconThemeCache.Unlock()
		return t
	}
	iconThemeCache.Unlock()
	var t *loadedIconTheme
	for _, pt := range pluginIconThemes() {
		if pt.contrib.ID == id {
			t = readIconTheme(pt)
			break
		}
	}
	iconThemeCache.Lock()
	iconThemeCache.themes[id] = t
	iconThemeCache.Unlock()
	return t
}

// readIconTheme reads a plugin's theme, nil when its file does not read,
// with a line in the plugin's log saying why.
func readIconTheme(t pluginIconTheme) *loadedIconTheme {
	name := path.Clean(t.contrib.Path)
	theme, err := parseIconTheme(t.plugin.FS(), name)
	if err != nil {
		t.plugin.Log("warn", fmt.Sprintf("file icon theme %s: %v", t.contrib.ID, err))
		return nil
	}
	theme.warn = func(format string, args ...any) { t.plugin.Log("warn", fmt.Sprintf(format, args...)) }
	return theme
}

// parseIconTheme reads the theme of name from fsys, as a plugin's folder
// gives it.
func parseIconTheme(fsys fs.FS, name string) (*loadedIconTheme, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	if len(data) > maxIconThemeBytes {
		return nil, fmt.Errorf("%s is larger than 1 MiB", name)
	}
	var dark iconThemeFile
	if err := json.Unmarshal(keymap.StripJSONC(data), &dark); err != nil {
		return nil, err
	}
	light := dark
	light.Light = nil
	if dark.Light != nil {
		light = mergeIconTheme(dark, *dark.Light)
	}
	dark.Light = nil
	normalizeIconTheme(&dark)
	normalizeIconTheme(&light)
	return &loadedIconTheme{
		fsys:  fsys,
		dir:   path.Dir(name),
		dark:  dark,
		light: light,
		svgs:  map[string]*iconThemeSVG{},
	}, nil
}

// mergeIconTheme is base with the light object's keys over it. The light
// object's maps add to the base's and override the keys they name, since
// a theme may name only what differs in a light window.
func mergeIconTheme(base, over iconThemeFile) iconThemeFile {
	over.Light = nil
	if over.IconDefinitions != nil {
		defs := make(map[string]iconThemeDef, len(base.IconDefinitions)+len(over.IconDefinitions))
		maps.Copy(defs, base.IconDefinitions)
		maps.Copy(defs, over.IconDefinitions)
		base.IconDefinitions = defs
	}
	for _, p := range []struct {
		dst *string
		src string
	}{
		{&base.File, over.File},
		{&base.Folder, over.Folder},
		{&base.FolderExpanded, over.FolderExpanded},
	} {
		if p.src != "" {
			*p.dst = p.src
		}
	}
	merge := func(base, over map[string]string) map[string]string {
		if over == nil {
			return base
		}
		out := maps.Clone(base)
		if out == nil {
			out = map[string]string{}
		}
		maps.Copy(out, over)
		return out
	}
	base.FileNames = merge(base.FileNames, over.FileNames)
	base.FileExtensions = merge(base.FileExtensions, over.FileExtensions)
	base.FolderNames = merge(base.FolderNames, over.FolderNames)
	base.FolderNamesExpanded = merge(base.FolderNamesExpanded, over.FolderNamesExpanded)
	return base
}

// normalizeIconTheme lowercases the names and extensions a theme matches,
// as VS Code does, so a lookup lowercases the file's name alone.
func normalizeIconTheme(f *iconThemeFile) {
	f.FileNames = lowerIconKeys(f.FileNames)
	f.FileExtensions = lowerIconKeys(f.FileExtensions)
	f.FolderNames = lowerIconKeys(f.FolderNames)
	f.FolderNamesExpanded = lowerIconKeys(f.FolderNamesExpanded)
}

func lowerIconKeys(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

// themeFile is the theme's dark or light side.
func (t *loadedIconTheme) themeFile(dark bool) *iconThemeFile {
	if dark {
		return &t.dark
	}
	return &t.light
}

// file is the icon of a file and whether the theme has one.
func (t *loadedIconTheme) file(p *palette, name string) (fileIconPaint, bool) {
	f := t.themeFile(p.dark)
	lower := strings.ToLower(name)
	id := f.FileNames[lower]
	if id == "" {
		id = iconExtension(f.FileExtensions, lower)
	}
	if id == "" {
		id = f.File
	}
	return t.paint(f, id, p)
}

// folder is the icon of a folder named name, open or closed, and whether
// the theme has one.
func (t *loadedIconTheme) folder(p *palette, name string, open bool) (fileIconPaint, bool) {
	f := t.themeFile(p.dark)
	lower := strings.ToLower(name)
	id := ""
	if open {
		id = f.FolderNamesExpanded[lower]
		if id == "" {
			id = f.FolderNames[lower]
		}
		if id == "" {
			id = f.FolderExpanded
		}
		if id == "" {
			id = f.Folder
		}
	} else {
		id = f.FolderNames[lower]
		if id == "" {
			id = f.Folder
		}
	}
	return t.paint(f, id, p)
}

// iconExtension is the definition a file name's extensions name, the
// longest first, so "tar.gz" beats "gz".
func iconExtension(m map[string]string, name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' && i+1 < len(name) {
			if id := m[name[i+1:]]; id != "" {
				return id
			}
		}
	}
	return ""
}

// paint is the icon of a definition and whether it draws.
func (t *loadedIconTheme) paint(f *iconThemeFile, id string, p *palette) (fileIconPaint, bool) {
	if id == "" {
		return fileIconPaint{}, false
	}
	def, ok := f.IconDefinitions[id]
	if !ok || def.IconPath == "" {
		return fileIconPaint{}, false
	}
	s, ok := t.parse(def.IconPath)
	if !ok {
		return fileIconPaint{}, false
	}
	return fileIconPaint{svg: s.svg, color: p.muted, own: s.own}, true
}

// parse reads an SVG of the theme, once. A file that does not read is
// remembered as such, so a row does not read it each frame.
func (t *loadedIconTheme) parse(iconPath string) (*iconThemeSVG, bool) {
	key := path.Clean(iconPath)
	if s, ok := t.svgs[key]; ok {
		return s, s != nil
	}
	fail := func(why string) (*iconThemeSVG, bool) {
		t.svgs[key] = nil
		if t.warn != nil {
			t.warn("file icon theme: %s", why)
		}
		return nil, false
	}
	full := path.Clean(path.Join(t.dir, key))
	if path.IsAbs(key) || full == ".." || strings.HasPrefix(full, "../") {
		return fail(fmt.Sprintf("%s is outside the plugin", key))
	}
	data, err := fs.ReadFile(t.fsys, full)
	if err != nil {
		return fail(fmt.Sprintf("%s: %v", full, err))
	}
	if len(data) > maxIconSVGBytes {
		return fail(fmt.Sprintf("%s is larger than 256 KiB", full))
	}
	svg, err := ui.ParseSVG(data)
	if err != nil {
		return fail(fmt.Sprintf("%s: %v", full, err))
	}
	s := &iconThemeSVG{svg: svg, own: svgOwnColors(data)}
	t.svgs[key] = s
	return s, true
}

// svgOwnColors reports whether an SVG carries colors of its own, rather
// than currentColor, and so is drawn as an image. An SVG with no color at
// all would draw black, and is drawn as a mono icon instead.
func svgOwnColors(data []byte) bool {
	s := string(data)
	if strings.Contains(s, "currentColor") {
		return false
	}
	return strings.Contains(s, "fill=") || strings.Contains(s, "stroke=") || strings.Contains(s, "style=") ||
		strings.Contains(s, "fill:") || strings.Contains(s, "stroke:")
}

// iconThemeChoice is a file icon theme of the settings and the box: its
// id, its name, and what it is.
type iconThemeChoice struct {
	id    string
	name  string
	about string
}

// allFileIconThemes are Mayoi's icons, then those of the plugins that are
// on.
func allFileIconThemes() []iconThemeChoice {
	out := []iconThemeChoice{{"", locales.FileIconThemeDefault(), locales.FileIconThemeMayoiAbout()}}
	for _, t := range pluginIconThemes() {
		out = append(out, iconThemeChoice{t.contrib.ID, t.contrib.Label, locales.ThemeAboutPlugin(t.plugin.Title())})
	}
	return out
}

// fileIconFor is the icon of a theme by id, without choosing it, as the
// box of the themes previews one.
func (a *app) fileIconFor(id, name string) fileIconPaint {
	if id != "" {
		if t := cachedIconTheme(id); t != nil {
			if p, ok := t.file(a.pal, name); ok {
				return p
			}
		}
	}
	svg, col := builtinFileIcon(a.pal, name)
	return fileIconPaint{svg: svg, color: col}
}

// fileIcon is the icon of a file: the active theme's, or Mayoi's own.
func (a *app) fileIcon(name string) fileIconPaint {
	if t := a.activeIconTheme(); t != nil {
		if p, ok := t.file(a.pal, name); ok {
			return p
		}
	}
	svg, col := builtinFileIcon(a.pal, name)
	return fileIconPaint{svg: svg, color: col}
}

// folderIcon is the icon of a folder: the active theme's, or Mayoi's own.
func (a *app) folderIcon(name string, open bool) fileIconPaint {
	if t := a.activeIconTheme(); t != nil {
		if p, ok := t.folder(a.pal, name, open); ok {
			return p
		}
	}
	if open {
		return fileIconPaint{svg: iconFolderOpen, color: a.pal.accent}
	}
	return fileIconPaint{svg: iconFolder, color: a.pal.accent}
}

// activeIconTheme is the theme the settings choose, or nil for Mayoi's
// icons. A theme or a plugin generation that changed since the window
// last asked is looked up anew.
func (a *app) activeIconTheme() *loadedIconTheme {
	id := a.settings.FileIconTheme
	if id == "" || plugins == nil || plugins.reg == nil {
		a.iconThemeID, a.iconThemeLoaded = "", nil
		return nil
	}
	gen := pluginGen()
	if a.iconThemeID == id && a.iconThemeGen == gen {
		return a.iconThemeLoaded
	}
	a.iconThemeID, a.iconThemeGen = id, gen
	a.iconThemeLoaded = cachedIconTheme(id)
	return a.iconThemeLoaded
}
