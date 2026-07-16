package scanner

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ReadSpecContent reads the .pin.xml file at filepath, locates the <spec>
// element whose id attribute matches the local part of specID (the suffix
// after the first dot), and returns its trimmed inner content.
//
// specID is the module-qualified spec ID (e.g. "core.validate"); the module
// prefix is stripped before matching against the local id attribute.
func ReadSpecContent(filepath, specID string) (string, error) {
	parts := strings.SplitN(specID, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid spec ID %q", specID)
	}
	localID := parts[1]

	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	type specElem struct {
		ID      string `xml:"id,attr"`
		Content string `xml:",innerxml"`
	}
	type pinFile struct {
		XMLName xml.Name
		Specs   []specElem `xml:"spec"`
	}

	var file pinFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return "", err
	}

	for _, s := range file.Specs {
		if s.ID == localID {
			return strings.TrimSpace(s.Content), nil
		}
	}
	return "", fmt.Errorf("spec %q not found in %s", specID, filepath)
}

var showMarkerPattern = regexp.MustCompile(`D!\s+id=\S+`)

// ReadMarkerContent reads the file at filepath, extracts the lines in the
// half-open range [startLine, endLine) (1-indexed), blanks any nested marker
// declarations (strips from "D!" to end of line), and returns the joined
// content with a trailing newline when non-empty.
//
// startLine is the line AFTER the range-start marker; endLine is the line
// OF the range-end marker (so the half-open range excludes both marker
// lines themselves).
func ReadMarkerContent(filepath string, startLine, endLine int) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	markerLines := make(map[int]bool)
	for i, line := range lines {
		if showMarkerPattern.MatchString(line) {
			markerLines[i] = true
		}
	}

	var contentLines []string
	for i := startLine; i < endLine-1; i++ {
		if i >= len(lines) {
			break
		}
		line := lines[i]
		if markerLines[i] {
			idx := strings.Index(line, "D!")
			if idx >= 0 {
				line = line[:idx]
			}
		}
		contentLines = append(contentLines, line)
	}

	content := strings.Join(contentLines, "\n")
	if len(contentLines) > 0 {
		content += "\n"
	}
	return content, nil
}
