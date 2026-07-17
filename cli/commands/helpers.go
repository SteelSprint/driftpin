package commands

import (
	"os"
	"path/filepath"
)

var markerSyntax = "D" + "! id=<markerid>"

// D! id=cinit range-start
func writeInitFile(dir, template string) error {
	mainPath := filepath.Join(dir, "main.drift.xml")
	if _, err := os.Stat(mainPath); err == nil {
		return nil
	}
	return os.WriteFile(mainPath, []byte(template), 0644)
}

// D! id=cinit range-end
