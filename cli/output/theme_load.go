package output

// D! id=otload range-start

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

type themeXML struct {
	XMLName  xml.Name     `xml:"theme"`
	Elements []elementXML `xml:"element"`
}

type elementXML struct {
	ID    string `xml:"id,attr"`
	Color string `xml:"color,attr"`
	Bold  bool   `xml:"bold,attr"`
	Dim   bool   `xml:"dim,attr"`
}

// LoadCustomTheme reads dir/.drift/theme.xml and returns a Theme built from
// the 14 element entries. All 14 element IDs must be present (full override —
// no inheritance from built-in themes). Returns os.ErrNotExist if the file
// does not exist; returns a descriptive error if any element is missing.
func LoadCustomTheme(dir string) (Theme, error) {
	path := filepath.Join(dir, ".drift", "theme.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}

	var raw themeXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Theme{}, fmt.Errorf("theme.xml: %s", err)
	}

	// Build a map of element ID → Style from the parsed XML.
	styleMap := make(map[string]Style, len(raw.Elements))
	for _, e := range raw.Elements {
		styleMap[e.ID] = Style{Color: e.Color, Bold: e.Bold, Dim: e.Dim}
	}

	// Validate all 14 elements are present.
	for _, id := range AllElementIDs {
		if _, ok := styleMap[id]; !ok {
			return Theme{}, fmt.Errorf("theme.xml: missing element %q (all 14 elements are required)", id)
		}
	}

	return Theme{
		Name:          "custom",
		MarkerID:      styleMap["marker_id"],
		SpecID:        styleMap["spec_id"],
		Filepath:      styleMap["filepath"],
		LineNumber:    styleMap["line_number"],
		Hash:          styleMap["hash"],
		StatusOK:      styleMap["status_ok"],
		StatusWarn:    styleMap["status_warn"],
		StatusError:   styleMap["status_error"],
		SectionHeader: styleMap["section_header"],
		Command:       styleMap["command"],
		Hint:          styleMap["hint"],
		DiffAdd:       styleMap["diff_add"],
		DiffRemove:    styleMap["diff_remove"],
		DiffHunk:      styleMap["diff_hunk"],
	}, nil
}

// D! id=otload range-end
