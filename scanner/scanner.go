package scanner

import (
	"bytes"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"drift/core"
)

// D! id=scode range-start

// binaryExtensions is a blocklist of file extensions that are virtually always
// binary. The scanner skips these without reading them (fast path). Anything
// else is checked by content sampling (null-byte heuristic) before scanning.
var binaryExtensions = map[string]bool{
	// images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".tiff": true, ".tif": true, ".ico": true, ".webp": true, ".svgz": true,
	".heic": true, ".heif": true, ".raw": true, ".psd": true, ".ai": true,
	// audio
	".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true,
	".oga": true, ".wma": true, ".m4a": true, ".aiff": true, ".aif": true,
	".opus": true, ".au": true,
	// video
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4v": true, ".mpg": true, ".mpeg": true,
	".vob": true, ".3gp": true, ".3g2": true,
	// archives
	".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true,
	".tbz": true, ".xz": true, ".txz": true, ".7z": true, ".rar": true,
	".zst": true, ".lz": true, ".lzma": true, ".cab": true, ".iso": true,
	".dmg": true, ".jar": true, ".war": true, ".ear": true, ".apk": true,
	".aab": true, ".ipa": true, ".deb": true, ".rpm": true, ".msi": true,
	// executables / binaries
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".o": true, ".a": true, ".lib": true, ".class": true, ".pyc": true,
	".pyo": true, ".wasm": true, ".node": true, ".ko": true,
	// databases
	".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true, ".accdb": true,
	".dbf": true,
	// documents (binary)
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	".epub": true, ".mobi": true, ".azw": true, ".azw3": true,
	// fonts
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	// models / 3d / media
	".blend": true, ".fbx": true, ".obj": true, ".stl": true, ".gltf": true,
	".glb": true, ".dae": true, ".3ds": true, ".dwg": true, ".dxf": true,
	// other binary
	".dat": true, ".pak": true, ".vcd": true, ".img": true, ".rom": true,
	".sav": true, ".cache": true, ".lockb": true,
}

// isTextFile reports whether path should be scanned for markers. It uses a
// two-stage check: (1) extension blocklist — fast path, skips known binary
// formats without reading the file; (2) content sampling — reads the first
// 8KB and rejects files containing null bytes (the classic heuristic for
// binary detection, used by grep/git). This makes drift language-agnostic:
// any text file of any extension is a valid marker host.
func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if binaryExtensions[ext] {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return false
	}
	return !bytes.Contains(buf[:n], []byte{0})
}

// D! id=scode range-end

// D! id=mregex range-start
// markerPattern matches D! id=<shortcode> followed by a REQUIRED range-start
// or range-end suffix. The shortcode must be a valid identifier: alphanumeric
// plus underscores (no dots, no punctuation). This prevents documentation
// that demonstrates the marker syntax (e.g. `D! id=...` or `D! id=<shortcode>`)
// from being mistaken for a real marker. The suffix is also mandatory so a
// bare `D! id=foo` without a range is not matched.
var markerPattern = regexp.MustCompile(`D!\s+id=([A-Za-z][A-Za-z0-9_]*)\s+(range-start|range-end)`)
// D! id=mregex range-end

type ScanResult struct {
	Specs   []core.Spec
	Markers []core.Marker
	Edges   []core.Edge
}

type Scanner interface {
	Scan() (ScanResult, error)
	Dir() string
}

type FileScanner struct {
	dir string
}

func NewFileScanner(dir string) *FileScanner {
	return &FileScanner{dir: dir}
}

func (s *FileScanner) Dir() string {
	return s.dir
}

func (s *FileScanner) Scan() (ScanResult, error) {
	ignore, err := loadDriftIgnore(s.dir)
	if err != nil {
		return ScanResult{}, err
	}
	specs, edges, err := s.scanSpecs()
	if err != nil {
		return ScanResult{}, err
	}
	markers, err := s.scanMarkers(ignore)
	if err != nil {
		return ScanResult{}, err
	}
	return ScanResult{Specs: specs, Markers: markers, Edges: edges}, nil
}

type specFileXML struct {
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

// D! id=sspec range-start
func (s *FileScanner) scanSpecs() ([]core.Spec, []core.Edge, error) {
	mainPath := filepath.Join(s.dir, "main.drift.xml")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("main.drift.xml not found in %s", s.dir)
	}

	loader := &importLoader{
		seenIDs:    make(map[string]bool),
		seenFiles:  make(map[string]bool),
		seenNames:  make(map[string]string),
		visitStack: nil,
	}
	absRoot, err := filepath.Abs(s.dir)
	if err != nil {
		return nil, nil, err
	}
	loader.rootDir = absRoot
	return loader.load(mainPath)
}

// D! id=sspec range-end

type importLoader struct {
	seenIDs    map[string]bool
	seenFiles  map[string]bool
	seenNames  map[string]string
	visitStack []string
	rootDir    string
}

func (l *importLoader) load(absPath string) ([]core.Spec, []core.Edge, error) {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, nil, err
	}

	for _, visited := range l.visitStack {
		if visited == absPath {
			var parts []string
			for _, p := range l.visitStack {
				parts = append(parts, filepath.Base(p))
			}
			parts = append(parts, filepath.Base(absPath))
			return nil, nil, fmt.Errorf("cycle detected: %s", strings.Join(parts, " → "))
		}
	}

	if l.seenFiles[absPath] {
		return nil, nil, nil
	}

	l.visitStack = append(l.visitStack, absPath)
	defer func() {
		l.visitStack = l.visitStack[:len(l.visitStack)-1]
	}()

	l.seenFiles[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, err
	}

	var file specFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", absPath, err)
	}

	isMain := file.XMLName.Local == "main"
	isModule := file.XMLName.Local == "module"

	if !isMain && !isModule {
		return nil, nil, fmt.Errorf("%s: expected <main> or <module> root element, got <%s>", absPath, file.XMLName.Local)
	}

	// D! id=swrap range-start
	if len(file.Wrapped) > 0 {
		return nil, nil, fmt.Errorf("%s: found <spec> elements nested inside a <specs> wrapper — specs must be direct children of <%s>, not inside <specs>", absPath, file.XMLName.Local)
	}
	// D! id=swrap range-end

	var moduleName string
	if isMain {
		moduleName = "main"
	} else {
		moduleName = file.Name
		// D! id=smname range-start
		if moduleName == "" {
			return nil, nil, fmt.Errorf("%s: module element missing name attribute", absPath)
		}
		// D! id=smname range-end
	}

	if existingPath, ok := l.seenNames[moduleName]; ok {
		return nil, nil, fmt.Errorf("duplicate module name %q (defined in %s and %s)", moduleName, filepath.Base(existingPath), filepath.Base(absPath))
	}
	l.seenNames[moduleName] = absPath
	l.seenFiles[absPath] = true

	dir := filepath.Dir(absPath)

	var specs []core.Spec
	var edges []core.Edge
	for _, elem := range file.Specs {
		var id string
		for _, attr := range elem.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
				break
			}
		}
		// D! id=smiss range-start
		if id == "" {
			return nil, nil, fmt.Errorf("%s: spec element missing id attribute", absPath)
		}
		// D! id=smiss range-end
		// D! id=sidfmt range-start
		if strings.Contains(id, ".") {
			return nil, nil, fmt.Errorf("%s: spec id %q must not contain a dot (dots are reserved for module qualification)", absPath, id)
		}
		// D! id=sidfmt range-end
		qualifiedID := moduleName + "." + id
		// D! id=sdups range-start
		if l.seenIDs[qualifiedID] {
			return nil, nil, fmt.Errorf("duplicate spec id %q", qualifiedID)
		}
		// D! id=sdups range-end
		l.seenIDs[qualifiedID] = true
		canonical, refTargets, err := processSpecContent(elem.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: spec %q: %w", absPath, qualifiedID, err)
		}
		hash := sha1Hex(canonical)
		specs = append(specs, core.Spec{
			ID:         qualifiedID,
			Module:     moduleName,
			Hash:       hash,
			Filepath:   relPath(l.rootDir, absPath),
			LineNumber: 0,
		})
		// Emit spec-spec edges in source order. Duplicate (from, to) pairs
		// within the same spec are deduped here so validation never sees
		// them.
		seenTargets := make(map[string]bool)
		for _, target := range refTargets {
			if seenTargets[target] {
				continue
			}
			seenTargets[target] = true
			edges = append(edges, core.Edge{
				From: qualifiedID,
				To:   target,
			})
		}
	}

	for _, imp := range file.Imports {
		importPath := filepath.Join(dir, imp.Path)
		if _, err := os.Stat(importPath); os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("import path not found: %s", imp.Path)
		}
		importedSpecs, importedEdges, err := l.load(importPath)
		if err != nil {
			return nil, nil, err
		}
		specs = append(specs, importedSpecs...)
		edges = append(edges, importedEdges...)
	}

	return specs, edges, nil
}

// D! id=smark range-start
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
		if !isTextFile(path) {
			return nil
		}
		// spec files (*.drift.xml) are parsed for specs, not markers — their
		// spec text may legitimately mention the D! marker syntax as documentation
		if strings.HasSuffix(relPath, ".drift.xml") {
			return nil
		}
		fileMarkers, err := parseMarkerFile(path, relPath)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, marker := range fileMarkers {
			// D! id=sdupm range-start
			if seenIDs[marker.ID] {
				return fmt.Errorf("duplicate marker shortcode %q", marker.ID)
			}
			seenIDs[marker.ID] = true
			// D! id=sdupm range-end
			markers = append(markers, marker)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// D! id=smark range-end

type rawMarkerDecl struct {
	id     string
	suffix string // "range-start" or "range-end"
	line   int    // 1-indexed
	index  int    // 0-indexed line position in file
}

func parseMarkerFile(path, storePath string) ([]core.Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Pass 1: Find all marker declarations
	var decls []rawMarkerDecl
	for i, line := range lines {
		match := markerPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		shortcode := match[1]
		suffix := match[2]
		lineNumber := i + 1

		decls = append(decls, rawMarkerDecl{
			id:     shortcode,
			suffix: suffix,
			line:   lineNumber,
			index:  i,
		})
	}

	// Validate pairs (all-at-once)
	starts := make(map[string]rawMarkerDecl)
	ends := make(map[string]rawMarkerDecl)
	for _, d := range decls {
		if d.suffix == "range-start" {
			if existing, ok := starts[d.id]; ok {
				return nil, fmt.Errorf("%s:%d: duplicate range-start for marker %q (first at line %d)", path, d.line, d.id, existing.line)
			}
			starts[d.id] = d
		} else {
			if existing, ok := ends[d.id]; ok {
				return nil, fmt.Errorf("%s:%d: duplicate range-end for marker %q (first at line %d)", path, d.line, d.id, existing.line)
			}
			ends[d.id] = d
		}
	}

	var unpaired []string
	for id, s := range starts {
		e, ok := ends[id]
		if !ok {
			unpaired = append(unpaired, fmt.Sprintf("%s:%d: marker %q has range-start but no matching range-end in the same file", path, s.line, id))
			continue
		}
		if e.line <= s.line {
			unpaired = append(unpaired, fmt.Sprintf("%s:%d: marker %q has range-end at line %d before range-start at line %d", path, e.line, id, e.line, s.line))
		}
	}
	for id, e := range ends {
		if _, ok := starts[id]; !ok {
			unpaired = append(unpaired, fmt.Sprintf("%s:%d: marker %q has range-end but no matching range-start in the same file", path, e.line, id))
		}
	}
	if len(unpaired) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(unpaired, "\n"))
	}

	// Pass 2: Compute hashes with blanking
	// Build a set of all marker declaration line indices for blanking
	markerLines := make(map[int]bool)
	for _, d := range decls {
		markerLines[d.index] = true
	}

	var markers []core.Marker
	for id, s := range starts {
		e := ends[id]

		var contentLines []string
		for j := s.index + 1; j < e.index; j++ {
			line := lines[j]
			if markerLines[j] {
				line = blankMarkerDecl(line)
			}
			contentLines = append(contentLines, line)
		}
		content := strings.Join(contentLines, "\n")
		if len(contentLines) > 0 {
			content += "\n"
		}
		hash := sha1Hex(content)

		markers = append(markers, core.Marker{
			ID:            id,
			Hash:          hash,
			Filepath:      storePath,
			LineNumber:    s.line,
			EndLineNumber: e.line,
		})
	}
	return markers, nil
}

// blankMarkerDecl strips the D! declaration from a line, leaving only the comment prefix.
// e.g. a marker declaration line "id=foo range-start" becomes just the comment prefix
func blankMarkerDecl(line string) string {
	idx := strings.Index(line, "D!")
	if idx < 0 {
		return line
	}
	return line[:idx]
}

// D! id=shash range-start
func sha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}

// D! id=shash range-end

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

func relPath(rootDir, absPath string) string {
	rel, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}
