package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"mayoi/internal/plugin"
	"mayoi/internal/settings"
)

const testIconColor = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M2 2h20v20H2z" fill="#00add8"/></svg>`
const testIconMono = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 2h20v20H2z"/></svg>`

func testIconThemeFS() fstest.MapFS {
	return fstest.MapFS{
		"icons.json": {Data: []byte(`{
			"iconDefinitions": {
				"_file":       {"iconPath": "icons/file.svg"},
				"_go":         {"iconPath": "icons/go.svg"},
				"_mod":        {"iconPath": "icons/file.svg"},
				"_named":      {"iconPath": "icons/go.svg"},
				"_gz":         {"iconPath": "icons/file.svg"},
				"_targz":      {"iconPath": "icons/file.svg"},
				"_missing":    {"iconPath": "icons/none.svg"},
				"_outside":    {"iconPath": "../secret.svg"},
				"_bad":        {"iconPath": "icons/bad.svg"},
				"_font":       {"fontCharacter": "\\E001"},
				"_folder":     {"iconPath": "icons/file.svg"},
				"_folderOpen": {"iconPath": "icons/file.svg"},
				"_git":        {"iconPath": "icons/file.svg"},
				"_gitOpen":    {"iconPath": "icons/file.svg"},
				"_light":      {"iconPath": "icons/file.svg"}
			},
			"file": "_file",
			"folder": "_folder",
			"folderExpanded": "_folderOpen",
			"fileNames": {"go.mod": "_named", "LICENSE": "_font", "bad.txt": "_bad", "gone.txt": "_missing", "escape.txt": "_outside"},
			"fileExtensions": {"go": "_go", "mod": "_mod", "tar.gz": "_targz", "gz": "_gz"},
			"folderNames": {".git": "_git"},
			"folderNamesExpanded": {".git": "_gitOpen"},
			"light": {"file": "_light"}
		}`)},
		"icons/file.svg": {Data: []byte(testIconMono)},
		"icons/go.svg":   {Data: []byte(testIconColor)},
		"icons/bad.svg":  {Data: []byte("not an svg")},
		"secret.svg":     {Data: []byte(testIconColor)},
	}
}

func TestIconThemeResolvesFiles(t *testing.T) {
	theme, err := parseIconTheme(testIconThemeFS(), "icons.json")
	if err != nil {
		t.Fatal(err)
	}
	dark := &palette{dark: true}
	light := &palette{dark: false}
	for _, tc := range []struct {
		name string
		p    *palette
		own  bool
		ok   bool
	}{
		{"main.go", dark, true, true},         // fileExtensions
		{"go.mod", dark, true, true},          // fileNames beats the extension
		{"x.mod", dark, false, true},          // the extension alone
		{"archive.tar.gz", dark, false, true}, // the longest extension
		{"raw.gz", dark, false, true},
		{"LICENSE", dark, false, false},     // a font character falls back
		{"bad.txt", dark, false, false},     // an SVG that does not parse falls back
		{"gone.txt", dark, false, false},    // a definition without a file falls back
		{"escape.txt", dark, false, false},  // a path outside the plugin falls back
		{"unknown.xyz", dark, false, true},  // the theme's file
		{"unknown.xyz", light, false, true}, // its light override
	} {
		got, ok := theme.file(tc.p, tc.name)
		if ok != tc.ok || (ok && got.own != tc.own) {
			t.Errorf("%s (dark %v): icon %v own %v, want ok %v own %v", tc.name, tc.p.dark, ok, got.own, tc.ok, tc.own)
		}
	}
}

func TestIconThemeResolvesFolders(t *testing.T) {
	theme, err := parseIconTheme(testIconThemeFS(), "icons.json")
	if err != nil {
		t.Fatal(err)
	}
	p := &palette{dark: true}
	for _, tc := range []struct {
		name string
		open bool
		ok   bool
	}{
		{".git", false, true},
		{".git", true, true},
		{"src", false, true},
		{"src", true, true},
	} {
		if _, ok := theme.folder(p, tc.name, tc.open); ok != tc.ok {
			t.Errorf("folder %s (open %v): %v, want %v", tc.name, tc.open, ok, tc.ok)
		}
	}
}

func TestIconThemeOpenFolderFallsBackToFolder(t *testing.T) {
	fsys := fstest.MapFS{
		"t.json": {Data: []byte(`{
			"iconDefinitions": {"_folder": {"iconPath": "folder.svg"}},
			"folder": "_folder"
		}`)},
		"folder.svg": {Data: []byte(testIconMono)},
	}
	theme, err := parseIconTheme(fsys, "t.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := theme.folder(&palette{dark: true}, "src", true); !ok {
		t.Error("an open folder without folderExpanded does not use the theme's folder")
	}
}

// The light object's maps add to the dark ones, and a light override wins.
func TestIconThemeLightMergesMaps(t *testing.T) {
	fsys := fstest.MapFS{
		"t.json": {Data: []byte(`{
			"iconDefinitions": {
				"_go": {"iconPath": "color.svg"},
				"_md": {"iconPath": "mono.svg"},
				"_mdLight": {"iconPath": "color.svg"}
			},
			"fileExtensions": {"go": "_go", "md": "_md"},
			"fileNames": {"README.md": "_md"},
			"light": {"fileExtensions": {"md": "_mdLight"}, "fileNames": {"README.md": "_mdLight"}}
		}`)},
		"color.svg": {Data: []byte(testIconColor)},
		"mono.svg":  {Data: []byte(testIconMono)},
	}
	theme, err := parseIconTheme(fsys, "t.json")
	if err != nil {
		t.Fatal(err)
	}
	light := &palette{dark: false}
	dark := &palette{dark: true}
	if icon, ok := theme.file(light, "main.go"); !ok || !icon.own {
		t.Error("a light window loses an extension the light object does not name")
	}
	if icon, ok := theme.file(light, "README.md"); !ok || !icon.own {
		t.Error("a light window does not take the light object's name")
	}
	if icon, ok := theme.file(dark, "README.md"); !ok || icon.own {
		t.Error("a dark window takes the light object's name")
	}
}

func TestSVGOwnColors(t *testing.T) {
	if svgOwnColors([]byte(testIconMono)) {
		t.Error("a currentColor SVG does not carry its own colors")
	}
	if !svgOwnColors([]byte(testIconColor)) {
		t.Error("a filled SVG carries its own colors")
	}
	if svgOwnColors([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h1v1z"/></svg>`)) {
		t.Error("an SVG without colors is drawn as a mono icon")
	}
}

type iconTestHost struct{}

func (iconTestHost) Call(*plugin.Plugin, string, json.RawMessage) (any, error) { return nil, nil }
func (iconTestHost) Settings(string) json.RawMessage                           { return nil }
func (iconTestHost) MayoiSetting(string) any                                   { return nil }
func (iconTestHost) Folder() string                                            { return "" }
func (iconTestHost) Changed()                                                  {}
func (iconTestHost) Logged(*plugin.Plugin, plugin.LogLine)                     {}

// A window with a theme chosen draws the theme's icons, and Mayoi's for a
// file the theme leaves out.
func TestFileIconFromPluginTheme(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	dir := filepath.Join(pluginsDir, "demo")
	for name, body := range map[string]string{
		"plugin.json": `{
			"name": "demo", "version": "0.1.0",
			"contributes": {"fileIconThemes": [{"id": "demo.icons", "label": "Demo", "path": "icons.json"}]}
		}`,
		"icons.json": `{
			"iconDefinitions": {"_go": {"iconPath": "go.svg"}},
			"fileExtensions": {"go": "_go"}
		}`,
		"go.svg": testIconColor,
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := plugin.New(plugin.Dirs{
		Plugins: pluginsDir,
		Scripts: filepath.Join(root, "scripts"),
		Storage: filepath.Join(root, "storage"),
	}, iconTestHost{})
	r.Version = "0.20.0"
	r.Load()
	defer r.Close()
	old := plugins
	plugins = &pluginHost{reg: r}
	defer func() { plugins = old }()

	s := settings.Default()
	s.FileIconTheme = "demo.icons"
	a := &app{settings: s, pal: darkPal}
	if got := a.fileIcon("main.go"); !got.own || got.svg == nil {
		t.Error("a Go file does not draw the theme's own-color icon")
	}
	if got := a.fileIcon("notes.unknown"); got.own || got.svg == nil {
		t.Error("a file the theme leaves out does not fall back to Mayoi's icon")
	}
	if got := a.folderIcon("src", true); got.svg != iconFolderOpen {
		t.Error("a folder the theme leaves out does not fall back to Mayoi's folder icon")
	}
}
