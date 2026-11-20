# File icon themes: design

Status: built. This is the design of file icon themes: a plugin brings a set of icons, a setting chooses one, and every
place Mayoi shows a file draws that theme's icon. [PLUGINS.md](PLUGINS.md) is the author's guide to plugins, and
[PLUGINS-DESIGN.md](PLUGINS-DESIGN.md) is the design of the plugin system itself. `languageIds` is not read yet: most
places that draw an icon know a file's name and not its language.

## Goals

- **A declarative theme, not code.** A file icon theme is a JSON file and SVG files in a plugin's folder. A plugin
  that brings only icons starts no JavaScript and costs nothing until its theme is chosen.
- **VS Code's format and its themes.** Read the file icon theme JSON that VS Code reads, so an existing theme such as
  Material Icon Theme works with no port. Mayoi already reads VS Code color themes; this is the same bargain.
- **Every surface once.** The Explorer, tabs, Quick Open, search results, Source Control and breadcrumbs already share
  `fileIcon`; a theme chosen once applies to all of them.
- **Mayoi's fallback.** A file, folder or theme the icons do not name keeps the icon Mayoi draws today. Removing a
  theme gives the current look back at once.
- **No network, no host.** Icon files come from the plugin's folder on this machine, as its manifest and themes do.

## Non-goals

- Running VS Code icon theme extensions. We take the format, not the extension host.
- Font icons (`fonts` and `fontCharacter`). SVG only; a theme that names a font glyph falls back.
- `highContrast` variants. `light` is read; the other variants follow the window's appearance.
- Themes that ask for network or remote images. The SVG parser draws no embedded images anyway.
- Theming every folder icon in Mayoi, such as the Recent pickers or the project rail. See [Scope](#scope).

## Decisions

| Question         | Decision                                                                       |
| ---------------- | ------------------------------------------------------------------------------ |
| Where a theme is | `contributes.fileIconThemes` in `plugin.json`                                   |
| Format           | VS Code file icon theme JSON, SVG `iconPath` entries only                       |
| Choosing one     | `fileIconTheme` setting, `""` for Mayoi's icons                                 |
| Resolution       | `fileNames`, then `fileExtensions`, then the default                            |
| Colors           | Own colors drawn as an image; `currentColor` drawn as an icon in the text color |
| Folders          | `folder`, `folderExpanded`, `folderNames`, `folderNamesExpanded` in the Explorer |
| Missing entries  | Fall back to today's `fileIcon` by name                                         |
| Bad files        | Log once, draw the fallback, keep the rest of the theme                         |
| Remote folders   | Icons are local; a remote folder needs nothing on the host                      |

## Icon themes today

`fileIcon` in `cmd/mayoi/icons.go:135` is a `switch` over file names and extensions that returns one Lucide icon and one
color. It is called from:

- Explorer rows, `cmd/mayoi/sidebar.go:453`
- tabs, `cmd/mayoi/tabs.go:155`
- Quick Open, `cmd/mayoi/quickopen.go:330`
- search results, `cmd/mayoi/search.go:1114`
- Source Control, `cmd/mayoi/scm.go:1952`
- breadcrumbs, `cmd/mayoi/crumbs.go:281`
- Wiki suggestions, `cmd/mayoi/wikisuggest.go:927`
- plugin views, `cmd/mayoi/pluginviews.go:556`
- the folder tree in the breadcrumb bar's popover, `cmd/mayoi/view.go:948`, `cmd/mayoi/crumbs.go:245`

Folders are drawn apart from `fileIcon`: the Explorer picks `iconFolder` or `iconFolderOpen`, then colors it
`p.accent` (`cmd/mayoi/sidebar.go:447`), and breadcrumbs do the same (`cmd/mayoi/crumbs.go:276`).

The plugin manifest has no contribution that reaches any of this. `Contributes` in
`internal/plugin/manifest.go:45` lists commands, keybindings, menus, settings, status items, views, previews, themes,
languages, snippets, formatters, terminal links, webviews, language servers and agents. A plugin's view items take an
icon only by Mayoi's name or by `"file"` (`cmd/mayoi/pluginviews.go:554`), and those names are a fixed table
(`cmd/mayoi/pluginpick.go:185`). The one place a plugin may give an SVG is an agent's mark, which only the Sessions
view reads (`cmd/mayoi/icons.go:222`).

So the feature is a core contribution plus a small plugin format, not something a plugin can do from its own folder
today.

## The contribution

```jsonc
{
  "contributes": {
    "fileIconThemes": [
      { "id": "material.file-icons", "label": "Material Icon Theme", "path": "icons.json" }
    ]
  }
}
```

- `id` starts with the plugin's name and a dot, as every contributed id does.
- `label` is the name in the settings menu, as a theme's is.
- `path` is relative to the plugin's folder and must stay inside it.

A plugin with only this contribution has no `main`, so Mayoi never starts a runtime for it. It appears in the settings
menu while it is on, and disabling it falls back to Mayoi's icons.

`internal/plugin/manifest.go` gains:

```go
type FileIconThemeContrib struct {
    ID    string `json:"id"`
    Label string `json:"label"`
    Path  string `json:"path"`
}
```

`Manifest.check` validates the id with the same `own` helper the other contributions use, requires a non-empty path,
and rejects an absolute path or one whose cleaned form starts with `..`.

## The theme file

The file is VS Code's file icon theme, restricted to what the SVG path can draw:

```jsonc
{
  "iconDefinitions": {
    "_file":        { "iconPath": "./icons/file.svg" },
    "_folder":      { "iconPath": "./icons/folder.svg" },
    "_folder_open": { "iconPath": "./icons/folder-open.svg" },
    "_go":          { "iconPath": "./icons/go.svg" },
    "_go_light":    { "iconPath": "./icons/go-light.svg" }
  },
  "file": "_file",
  "folder": "_folder",
  "folderExpanded": "_folder_open",
  "fileNames": { "go.mod": "_go", "Dockerfile": "_docker" },
  "fileExtensions": { "go": "_go", "ts": "_typescript" },
  "folderNames": { ".git": "_git" },
  "folderNamesExpanded": { ".git": "_git_open" },
  "light": { "file": "_file_light", "fileExtensions": { "go": "_go_light" } }
}
```

### Resolution

For a file:

1. its whole name in `fileNames`, case-insensitively;
2. its extensions in `fileExtensions`, the longest first, so `tar.gz` beats `gz`;
3. `file`.

For a folder:

1. when it is open, its name in `folderNamesExpanded`, else in `folderNames`;
2. when it is open, `folderExpanded`, else `folder`.

A value is an entry of `iconDefinitions`. An entry without `iconPath`, or with a `fontCharacter`, falls through to the
default Mayoi draws. A definition is resolved once and cached; an SVG is parsed on first use, not when the theme loads.

The `light` object overrides the top-level keys when the window is light. Keys it leaves out keep the dark ones. The
maps add to the dark ones and override only the keys they name: Material Icon Theme's `light` has 32 extensions and
179 names, not the whole sets, and the rest keep their dark icons.

### Paths and limits

- Each `iconPath` is relative to the theme file's folder, and may use `..` within the plugin, as a theme in `dist/`
  pointing at `../icons` does; a path that leaves the plugin is an error.
- A theme file over 1 MiB is not read. An SVG over 256 KiB falls back to Mayoi's icon, with a line in the plugin's log.
- SVGs are parsed by `ui.ParseSVG` as plugin view SVGs already are. Text, filters and embedded images are not drawn;
  that is the parser's nature, not a new rule.

## Drawing

`fileIcon` today returns `(*ui.SVG, ui.Color)` and every caller draws it with `ui.Icon`, which paints the shapes in
the text color whatever colors the SVG has. Material's icons are colored, so the lookup needs to say which way to draw.

```go
type fileIconPaint struct {
    svg   *ui.SVG
    color ui.Color // for a mono icon and for a fallback
    own   bool     // draw the SVG's own colors
}

func (ip fileIconPaint) element(c *ui.Context, size float32) ui.Element
func (a *app) fileIcon(name string) fileIconPaint
func (a *app) folderIcon(name string, open bool) fileIconPaint
```

- **Own colors.** An SVG with a literal `fill` or `stroke` other than `currentColor` is drawn with `ui.Image` at the
  row's icon size. Material Icon Theme lands here.
- **Mono.** An SVG that uses `currentColor` only, as Lucide's do, is drawn with `ui.Icon` in the theme's neutral text
  color, `p.muted`. The theme chose monochrome; Mayoi does not add language colors to it.
- **Fallback.** With no theme, or no entry for the name, the existing `fileIcon` result is returned unchanged, so the
  current look is bit-for-bit what it was.

Callers draw one element:

```go
a.fileIcon(name).element(c, 14)
```

`element` picks `ui.Icon` or `ui.Image` from the paint. This replaces the two-line `icon, col := fileIcon(p, name)` /
`ui.Icon(...)` pattern at the call sites, so each site changes once and the colored case is honored everywhere.
A diff's painter draws the same paint with `paint(pt, rect)`. The few places that need the color itself, as the theme
tests do, read the paint's field.

Icon size stays the row's 14 px for both kinds, so a themed icon and a fallback icon sit in the same slot and a long
name never moves the label.

## Folders

The Explorer gains `folderIcon(p, name, open)` beside `fileIcon`: it asks the theme for `folderNames`,
`folderNamesExpanded`, `folder` or `folderExpanded`, and falls back to `iconFolder`/`iconFolderOpen` in `p.accent` as
today. Breadcrumbs use it too.

Other folder icons stay Mayoi's: Recent, Remote, the repository picker, the project rail and the file tree's generic
controls are not the Explorer's rows, and a theme should not recolor pickers.

## Choosing a theme

A new setting, `fileIconTheme`, in `settings.json`:

```jsonc
{
  "fileIconTheme": "material.file-icons"
}
```

- `""` is Mayoi's icons.
- A value names a contributed theme, as `theme` names a color theme.
- An id that no enabled plugin has falls back to `""`, so removing a plugin never leaves an empty Explorer.

The Appearance category of the settings window gets a menu of Mayoi's icons and each enabled plugin's themes.
**View → Theme → File Icon Theme…** opens a searchable box of them, previewing each as the choice moves; Return keeps
it and Escape goes back. Choosing one writes the setting and repaints the next frame.

## Loading, caches and freshness

The loader mirrors plugin color themes (`cmd/mayoi/plugintheme.go`):

```go
type pluginIconTheme struct {
    plugin  *plugin.Plugin
    contrib plugin.FileIconThemeContrib
}

func pluginIconThemes() []pluginIconTheme // from plugins.enabled(), in plugin order
```

- The active theme is looked up by id; an unknown id is nil and every lookup falls back.
- A cache holds, by theme id and window darkness, the parsed theme file and its definitions. Its generation is
  `plugins.gen`, so enabling, disabling or reloading a plugin drops it, exactly as `themeCache` does.
- Parsed `*ui.SVG`s are cached by resolved path and shared across rows, files and frames. The parser's own picture
  cache then does the per-color work.
- A change to `fileIconTheme` clears the active pointer only; the defined themes stay cached.
- Nothing here runs on a plugin goroutine. The manifest is enough to list themes, and the files are read from the
  plugin's `fs.FS` on the main thread, once.

## Scope

The lookup sits behind `fileIcon`, so every caller gets the theme for free. The plan is still to land it in this
order, so each step can ship alone:

1. **Explorer files.** Manifest, loader, lookup, `a.fileIcon`, the setting, and the Explorer's file rows. No
   color rendering: mono icons only, drawn with `ui.Icon`.
2. **Colors, folders and the box.** `ui.Image` for own-color SVGs, `folderIcon`, light and dark variants, the
   settings menu and **File Icon Theme…**.
3. **Every surface.** Tabs, Quick Open, search, Source Control, breadcrumbs and their folder tree, plugin views move to
   `a.fileIcon`; docs, tests and the changelog bullet.

All three are built, but `languageIds`: a theme's file name and extension maps cover the callers that know only a
name.

## Code layout

| File                              | Change                                                                    |
| --------------------------------- | ------------------------------------------------------------------------- |
| `internal/plugin/manifest.go`     | `FileIconThemes` contribution, its struct, validation                      |
| `cmd/mayoi/fileicons.go`          | New: theme listing, loading, lookup, caches, `fileIconPaint`               |
| `cmd/mayoi/icons.go`              | `fileIcon` asks the active theme first, then falls back                    |
| `cmd/mayoi/sidebar.go`            | Explorer file and folder rows use the lookup and the draw helper           |
| `cmd/mayoi/crumbs.go`             | Breadcrumb files and folders use it too                                    |
| `cmd/mayoi/tabs.go`, `quickopen.go`, `search.go`, `scm.go`, `wikisuggest.go`, `pluginviews.go`, `view.go` | Use `a.fileIcon` |
| `cmd/mayoi/iconpicker.go`         | New: **File Icon Theme…**, modeled on `themepicker.go`                     |
| `cmd/mayoi/prefscontrols.go`      | The Appearance row, as `theme` has one                                    |
| `internal/settings/schema.go`, `settings.go` | The `fileIconTheme` key                                         |
| `docs/PLUGINS.md`, `docs/SETTINGS.md` | The contribution, the file format, the setting                       |
| `CHANGELOG.md`                    | One bullet under `## Unreleased`                                          |

`fileIcon` becomes a method of the window's app, so it can read the setting and the plugins, and returns
`fileIconPaint`. That is nine call sites, all in `cmd/mayoi`, plus `entryIcon`, which becomes a method too; the
compiler finds them, and `element` keeps each edit to a line or two.

## Tests

- Manifest: a theme id without the plugin's prefix, a duplicate id, an absolute path and a `..` path are refused.
- Resolution: `fileNames` beats `fileExtensions`; the longest extension wins; folder names and their open variants;
  `light` overrides only what it names; a missing definition falls through to Mayoi's icon.
- Failure: a theme with an unreadable file, an oversized theme, an unparseable SVG and a `fontCharacter` entry each
  log once and draw the fallback; the rest of the theme still draws.
- Cache: a new plugin generation drops the theme cache; changing `fileIconTheme` swaps the active theme without a
  reload.
- Drawing: a `currentColor` SVG takes `p.muted`; a colored SVG keeps its own colors; the fallback matches
  `fileIcon`'s icon and color exactly.

## Risks

- **Parsing cost.** Material Icon Theme has about a thousand SVGs. Parsing is lazy and cached; the first Explorer
  frame after choosing the theme parses only the icons on screen.
- **Image cost.** `ui.Image` is heavier than a tinted `ui.Icon`. Only own-color themes take that path, and the engine
  caches the picture by SVG, size and color.
- **Contrast.** A colored icon theme can clash with a theme's background. The fallback does not, and Mayoi's icons are
  one choice away in the settings menu.
- **Untrusted SVG.** The files are plugin content, trusted as all plugin code is, but the loader still keeps paths
  inside the plugin and caps sizes, so a bad theme is a bad icon, not a path traversal.
- **Format drift.** VS Code may add keys; unknown keys are ignored, and an icon theme that needs a font falls back
  instead of failing.

## Alternatives considered

- **A webview file tree.** A plugin can draw its own Explorer in HTML today, with Material icons and `workspace.open`.
  It is a separate sidebar slot, loses the native context menus, drag and drop, Git letters and dirty marks, and has
  no directory-change event to refresh on. Rejected as a replacement for the Explorer.
- **More hardcoded icons.** Extending `fileIcon`'s switch helps only the names we think of and cannot carry a theme's
  colors or folder sets. Rejected.
- **SVG icons on view items.** Letting `views` items take an SVG would make plugin views prettier, but it still does
  not touch the Explorer, tabs or Quick Open. It can follow later on its own.
- **A Mayoi-only theme format.** It would be simpler to validate, and no existing theme could be used. Rejected; the
  VS Code format is the point.

## Prior art

VS Code's `contributes.iconThemes` is a JSON file of `iconDefinitions` and name, extension and folder maps, with
`light` overrides. Material Icon Theme is the best-known theme in that format, and its SVG build is the test case.
Mayoi already maps VS Code color themes in `cmd/mayoi/plugintheme.go`, so this adds no new compatibility story.

## Open questions

- Should a mono theme icon take `p.muted` or the language color `fileIcon` would give it? This plan says `p.muted`,
  because the theme chose monochrome; the fallback keeps language colors.
- Should Source Control's folder rows use `folderIcon` too? The plan themes the Explorer and breadcrumbs first.
- Should `languageIds` be read? It needs the file's language at the call site, which tabs and the editor know but
  Quick Open and search do not; the file name and extension maps cover the callers today.
- Should a bundled Material theme ship with Mayoi? This plan does not bundle one: icon themes are plugins, and the
  point is that the format lets any of them install.