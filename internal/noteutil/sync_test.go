package noteutil

import "testing"

func TestSyncReplacementProtectsRichStructure(t *testing.T) {
	previous := DocumentFromHTML(`<p>Text</p><ul data-type="checklist"><li data-type="taskItem" data-checked="true">Done</li></ul><div data-type="drawing" data-attachment-id="draw" data-preview-id="preview"></div>`)
	cases := []struct {
		name, html string
		valid      bool
	}{
		{"roundtrip", ToHTML(previous), true},
		{"text edit", `<p>Changed</p><ul data-type="checklist"><li data-type="taskItem" data-checked="false">Todo</li></ul><div data-type="drawing" data-attachment-id="draw" data-preview-id="preview"></div>`, true},
		{"flattened", "<p>Text Done</p>", false},
		{"lost drawing", `<ul data-type="checklist"><li data-type="taskItem">Done</li></ul>`, false},
		{"lost checklist", `<p>Done</p><div data-type="drawing" data-attachment-id="draw" data-preview-id="preview"></div>`, false},
		{"clear", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSyncReplacement(previous, DocumentFromHTML(tc.html)); (err == nil) != tc.valid {
				t.Fatal(err)
			}
		})
	}
}

func TestSyncHTMLDoesNotSilentlyDiscardImagesOrTables(t *testing.T) {
	for _, source := range []string{`<img src="cid:picture">`, `<p>Text</p><table><tr><td>Value</td></tr></table>`, `<svg><path/></svg>`} {
		if _, err := DocumentFromSyncHTML(source); err == nil {
			t.Fatalf("unsupported content accepted: %s", source)
		}
	}
	if _, err := DocumentFromSyncHTML(`<html><body><p>Text</p><ul><li>Item</li></ul></body></html>`); err != nil {
		t.Fatal(err)
	}
}
