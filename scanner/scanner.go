package scanner

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"driftpin/core"
)

const markerLineWindow = 10

// D! id=scode
var codeExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true,
	".jsx": true, ".tsx": true, ".java": true, ".c": true,
	".cpp": true, ".h": true, ".hpp": true, ".rs": true,
	".rb": true, ".php": true, ".swift": true, ".kt": true,
	".cs": true, ".scala": true, ".sh": true, ".bash": true,
	".lua": true, ".dart": true, ".vue": true, ".svelte": true,
}

var markerPattern = regexp.MustCompile(`D!\s+id=(\S+)`)

type ScanResult struct {
	Specs   []core.Spec
	Markers []core.Marker
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
	ignore, err := loadDriftIgnore(s.dir)
	if err != nil {
		return ScanResult{}, err
	}
	specs, err := s.scanSpecs()
	if err != nil {
		return ScanResult{}, err
	}
	markers, err := s.scanMarkers(ignore)
	if err != nil {
		return ScanResult{}, err
	}
	return ScanResult{Specs: specs, Markers: markers}, nil
}

type pinFileXML struct {
	XMLName xml.Name
	Name    string       `xml:"name,attr"`
	Imports []importElem `xml:"import"`
	Specs   []specElem   `xml:"spec"`
	Wrapped []specElem   `xml:"specs>spec"`
}

type importElem struct {
	XMLName xml.Name `xml:"import"`
	Path    string   `xml:"path,attr"`
}

type specElem struct {
	XMLName xml.Name
	Attr    []xml.Attr `xml:",any,attr"`
	Content string     `xml:",innerxml"`
}

// D! id=sspec
func (s *FileScanner) scanSpecs() ([]core.Spec, error) {
	mainPath := filepath.Join(s.dir, "main.pin.xml")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("main.pin.xml not found in %s", s.dir)
	}

	loader := &importLoader{
		seenIDs:    make(map[string]bool),
		seenFiles:  make(map[string]bool),
		seenNames:  make(map[string]string),
		visitStack: nil,
	}
	return loader.load(mainPath)
}

type importLoader struct {
	seenIDs    map[string]bool
	seenFiles  map[string]bool
	seenNames  map[string]string
	visitStack []string
}

func (l *importLoader) load(absPath string) ([]core.Spec, error) {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, err
	}

	for _, visited := range l.visitStack {
		if visited == absPath {
			var parts []string
			for _, p := range l.visitStack {
				parts = append(parts, filepath.Base(p))
			}
			parts = append(parts, filepath.Base(absPath))
			return nil, fmt.Errorf("cycle detected: %s", strings.Join(parts, " → "))
		}
	}

	if l.seenFiles[absPath] {
		return nil, nil
	}

	l.visitStack = append(l.visitStack, absPath)
	defer func() {
		l.visitStack = l.visitStack[:len(l.visitStack)-1]
	}()

	l.seenFiles[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var file pinFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}

	isMain := file.XMLName.Local == "main"
	isModule := file.XMLName.Local == "module"

	if !isMain && !isModule {
		return nil, fmt.Errorf("%s: expected <main> or <module> root element, got <%s>", absPath, file.XMLName.Local)
	}

	// D! id=swrap
	if len(file.Wrapped) > 0 {
		return nil, fmt.Errorf("%s: found <spec> elements nested inside a <specs> wrapper — specs must be direct children of <%s>, not inside <specs>", absPath, file.XMLName.Local)
	}

	var moduleName string
	if isMain {
		moduleName = "main"
	} else {
		moduleName = file.Name
		// D! id=smname
		if moduleName == "" {
			return nil, fmt.Errorf("%s: module element missing name attribute", absPath)
		}
	}

	if existingPath, ok := l.seenNames[moduleName]; ok {
		return nil, fmt.Errorf("duplicate module name %q (defined in %s and %s)", moduleName, filepath.Base(existingPath), filepath.Base(absPath))
	}
	l.seenNames[moduleName] = absPath
	l.seenFiles[absPath] = true

	dir := filepath.Dir(absPath)

	var specs []core.Spec
	for _, elem := range file.Specs {
		var id string
		for _, attr := range elem.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
				break
			}
		}
		// D! id=smiss
		if id == "" {
			return nil, fmt.Errorf("%s: spec element missing id attribute", absPath)
		}
		// D! id=sidfmt
		if strings.Contains(id, ".") {
			return nil, fmt.Errorf("%s: spec id %q must not contain a dot (dots are reserved for module qualification)", absPath, id)
		}
		qualifiedID := moduleName + "." + id
		// D! id=sdups
		if l.seenIDs[qualifiedID] {
			return nil, fmt.Errorf("duplicate spec id %q", qualifiedID)
		}
		l.seenIDs[qualifiedID] = true
		content := strings.TrimSpace(elem.Content)
		hash := sha1Hex(content)
		specs = append(specs, core.Spec{
			ID:         qualifiedID,
			Module:     moduleName,
			Hash:       hash,
			Filepath:   absPath,
			LineNumber: 0,
		})
	}

	for _, imp := range file.Imports {
		importPath := filepath.Join(dir, imp.Path)
		if _, err := os.Stat(importPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("import path not found: %s", imp.Path)
		}
		importedSpecs, err := l.load(importPath)
		if err != nil {
			return nil, err
		}
		specs = append(specs, importedSpecs...)
	}

	return specs, nil
}

// D! id=smark
func (s *FileScanner) scanMarkers(ignore *driftIgnore) ([]core.Marker, error) {
	var markers []core.Marker
	seenIDs := make(map[string]bool)

	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(s.dir, path)
		if ignore.matches(relPath, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
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
			// D! id=sdupm
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

func parseMarkerFile(path string) ([]core.Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var markers []core.Marker

	for i, line := range lines {
		match := markerPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		shortcode := match[1]
		lineNumber := i + 1

		// D! id=midfmt
		if strings.Contains(shortcode, ".") {
			return nil, fmt.Errorf("%s:%d: marker id %q must not contain a dot (dots are reserved for spec ID qualification)", path, lineNumber, shortcode)
		}

		var contentLines []string
		for j := i + 1; j < len(lines) && j <= i+markerLineWindow; j++ {
			contentLines = append(contentLines, lines[j])
		}
		content := strings.Join(contentLines, "\n")
		if len(contentLines) > 0 {
			content += "\n"
		}
		hash := sha1Hex(content)

		markers = append(markers, core.Marker{
			ID:         shortcode,
			Hash:       hash,
			Filepath:   path,
			LineNumber: lineNumber,
		})
	}
	return markers, nil
}

// D! id=shash
func sha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}

type driftIgnore struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	raw      string
	dirOnly  bool
	hasSlash bool
}

func loadDriftIgnore(dir string) (*driftIgnore, error) {
	path := filepath.Join(dir, "drift.ignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &driftIgnore{}, nil
		}
		return nil, err
	}

	ig := &driftIgnore{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := ignorePattern{raw: line}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			p.raw = strings.TrimSuffix(line, "/")
		}
		if strings.Contains(p.raw, "/") {
			p.hasSlash = true
		}
		ig.patterns = append(ig.patterns, p)
	}
	return ig, nil
}

func (ig *driftIgnore) matches(relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)
	base := filepath.Base(relPath)
	for _, p := range ig.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		var match bool
		if p.hasSlash {
			match, _ = filepath.Match(p.raw, relPath)
			if !match && !p.dirOnly && isDir && strings.HasPrefix(relPath, p.raw+"/") {
				return true
			}
		} else {
			match, _ = filepath.Match(p.raw, base)
		}
		if match {
			return true
		}
	}
	return false
}
