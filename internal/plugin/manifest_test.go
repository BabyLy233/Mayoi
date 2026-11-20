package plugin

import (
	"fmt"
	"testing"
)

func TestFileIconThemeManifest(t *testing.T) {
	m, err := ParseManifest([]byte(`{
		"name": "demo",
		"contributes": {
			"fileIconThemes": [{"id": "demo.icons", "label": "Demo Icons", "path": "icons.json"}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Contributes.FileIconThemes) != 1 || m.Contributes.FileIconThemes[0].Label != "Demo Icons" {
		t.Fatalf("file icon themes = %+v", m.Contributes.FileIconThemes)
	}
}

func TestFileIconThemeManifestChecks(t *testing.T) {
	const valid = `"name": "demo", "contributes": {"fileIconThemes": [%s]}`
	for _, tc := range []struct {
		name  string
		theme string
	}{
		{"without the plugin's prefix", `{"id": "icons", "label": "Icons", "path": "icons.json"}`},
		{"without a label", `{"id": "demo.icons", "path": "icons.json"}`},
		{"without a path", `{"id": "demo.icons", "label": "Icons"}`},
		{"with an absolute path", `{"id": "demo.icons", "label": "Icons", "path": "/icons.json"}`},
		{"with a path outside the plugin", `{"id": "demo.icons", "label": "Icons", "path": "../icons.json"}`},
		{"twice", `{"id": "demo.icons", "label": "Icons", "path": "icons.json"},
			{"id": "demo.icons", "label": "Icons Again", "path": "icons.json"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(`{` + fmt.Sprintf(valid, tc.theme) + `}`)); err == nil {
				t.Error("the manifest parses")
			}
		})
	}
}
