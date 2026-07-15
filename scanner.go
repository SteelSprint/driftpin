package driftpin

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const markerLineWindow = 10

var codeExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true,
	".jsx": true, ".tsx": true, ".java": true, ".c": true,
	".cpp": true, ".h": true, ".hpp": true, ".rs": true,
	".rb": true, ".php": true, ".swift": true, ".kt": true,
	".cs": true, ".scala": true, ".sh": true, ".bash": true,
	".lua": true, ".dart": true, ".vue": true, ".svelte": true,
}

var markerPattern = regexp.MustCompile(`#F\s+(\S+)`)

type ScanResult struct {
	Specs   []Spec
	Markers []Marker
}

type Scanner interface {
	Scan() (ScanResult, error)
}

type FileScanner struct {
	dir string
}

func NewFileScanner(dir string) *FileScanner {
	return &FileScanner{dir: dir}
}

func (s *FileScanner) Scan() (ScanResult, error) {
	specs, err := s.scanSpecs()
	if err != nil {
		return ScanResult{}, err
	}
	markers, err := s.scanMarkers()
	if err != nil {
		return ScanResult{}, err
	}
	return ScanResult{Specs: specs, Markers: markers}, nil
}

type specFileXML struct {
	XMLName xml.Name   `xml:"specs"`
	Specs   []specElem `xml:"spec"`
}

type specElem struct {
	XMLName xml.Name
	Attr    []xml.Attr `xml:",any,attr"`
	Content string     `xml:",innerxml"`
}

func (s *FileScanner) scanSpecs() ([]Spec, error) {
	var specs []Spec
	seenIDs := make(map[string]bool)

	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".pin.xml") {
			return nil
		}
		fileSpecs, err := parseSpecFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, spec := range fileSpecs {
			if seenIDs[spec.ID] {
				return fmt.Errorf("duplicate spec id %q", spec.ID)
			}
			seenIDs[spec.ID] = true
			specs = append(specs, spec)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return specs, nil
}

func parseSpecFile(path string) ([]Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file specFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	var specs []Spec
	for _, elem := range file.Specs {
		var id string
		for _, attr := range elem.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("spec element missing id attribute")
		}
		content := strings.TrimSpace(elem.Content)
		hash := sha1Hex(content)
		specs = append(specs, Spec{
			ID:         id,
			Hash:       hash,
			Filepath:   path,
			LineNumber: 0,
		})
	}
	return specs, nil
}

func (s *FileScanner) scanMarkers() ([]Marker, error) {
	var markers []Marker
	seenIDs := make(map[string]bool)

	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if !codeExtensions[ext] {
			return nil
		}
		fileMarkers, err := parseMarkerFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, marker := range fileMarkers {
			if seenIDs[marker.ID] {
				return fmt.Errorf("duplicate marker shortcode %q", marker.ID)
			}
			seenIDs[marker.ID] = true
			markers = append(markers, marker)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return markers, nil
}

func parseMarkerFile(path string) ([]Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var markers []Marker

	for i, line := range lines {
		match := markerPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		shortcode := match[1]
		lineNumber := i + 1

		var contentLines []string
		for j := i + 1; j < len(lines) && j <= i+markerLineWindow; j++ {
			contentLines = append(contentLines, lines[j])
		}
		content := strings.Join(contentLines, "\n")
		if len(contentLines) > 0 {
			content += "\n"
		}
		hash := sha1Hex(content)

		markers = append(markers, Marker{
			ID:         shortcode,
			Hash:       hash,
			Filepath:   path,
			LineNumber: lineNumber,
		})
	}
	return markers, nil
}

func sha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}
