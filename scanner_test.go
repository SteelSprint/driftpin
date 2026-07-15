package driftpin

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeSpecFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec file %s: %v", name, err)
	}
}

func writeCodeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write code file %s: %v", name, err)
	}
}

func expectedSha1Hex(content string) string {
	h := sha1.Sum([]byte(content))
	return fmt.Sprintf("%x", h)
}

func findScanResultSpec(results []Spec, id string) (Spec, bool) {
	for _, s := range results {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

func findScanResultMarker(results []Marker, id string) (Marker, bool) {
	for _, m := range results {
		if m.ID == id {
			return m, true
		}
	}
	return Marker{}, false
}

func TestScannerEmptyProject(t *testing.T) {
	t.Run("no_files_returns_empty", func(t *testing.T) {
		dir := t.TempDir()
		scanner := NewFileScanner(dir)

		result, err := scanner.Scan()
		assertNoError(t, err)
		if len(result.Specs) != 0 {
			t.Fatalf("expected 0 specs, got %d", len(result.Specs))
		}
		if len(result.Markers) != 0 {
			t.Fatalf("expected 0 markers, got %d", len(result.Markers))
		}
	})
}

func TestScannerSpecDiscovery(t *testing.T) {
	t.Run("one_spec_file_one_spec_element", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(result.Specs))
		}
		spec, ok := findScanResultSpec(result.Specs, "validate_input")
		if !ok {
			t.Fatalf("expected spec validate_input, not found")
		}
		if spec.Filepath != filepath.Join(dir, "specs.pin.xml") {
			t.Fatalf("filepath = %q, want %q", spec.Filepath, filepath.Join(dir, "specs.pin.xml"))
		}
		if spec.Hash == "" {
			t.Fatalf("expected non-empty hash")
		}
	})

	t.Run("one_spec_file_many_spec_elements", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "specs.pin.xml", `<specs>
			<spec id="validate_input">input must be validated</spec>
			<spec id="auth_check">auth must be checked</spec>
			<spec id="log_request">request must be logged</spec>
		</specs>`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(result.Specs))
		}
		for _, id := range []string{"validate_input", "auth_check", "log_request"} {
			if _, ok := findScanResultSpec(result.Specs, id); !ok {
				t.Fatalf("expected spec %s, not found", id)
			}
		}
	})

	t.Run("many_spec_files", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "auth.pin.xml", `<specs><spec id="auth">auth spec</spec></specs>`)
		writeSpecFile(t, dir, "api.pin.xml", `<specs><spec id="api">api spec</spec></specs>`)
		subdir := filepath.Join(dir, "sub")
		os.Mkdir(subdir, 0755)
		writeSpecFile(t, subdir, "nested.pin.xml", `<specs><spec id="nested">nested spec</spec></specs>`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(result.Specs))
		}
		for _, id := range []string{"auth", "api", "nested"} {
			if _, ok := findScanResultSpec(result.Specs, id); !ok {
				t.Fatalf("expected spec %s, not found", id)
			}
		}
	})

	t.Run("spec_missing_id_attribute_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec>no id here</spec></specs>`)

		scanner := NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for spec missing id")
		}
	})

	t.Run("duplicate_spec_ids_error", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "a.pin.xml", `<specs><spec id="dup">first</spec></specs>`)
		writeSpecFile(t, dir, "b.pin.xml", `<specs><spec id="dup">second</spec></specs>`)

		scanner := NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for duplicate spec id")
		}
	})

	t.Run("hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		content := `<specs><spec id="s1">deterministic content</spec></specs>`
		writeSpecFile(t, dir, "specs.pin.xml", content)

		scanner := NewFileScanner(dir)
		result1, err := scanner.Scan()
		assertNoError(t, err)

		result2, err := scanner.Scan()
		assertNoError(t, err)

		spec1, _ := findScanResultSpec(result1.Specs, "s1")
		spec2, _ := findScanResultSpec(result2.Specs, "s1")

		if spec1.Hash != spec2.Hash {
			t.Fatalf("hash not deterministic: %q vs %q", spec1.Hash, spec2.Hash)
		}
	})
}

func TestScannerMarkerDiscovery(t *testing.T) {
	t.Run("one_code_file_one_marker", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "main.go", `package main

// #F abc123
func handleRequest() {
	doSomething()
}
`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
		marker, ok := findScanResultMarker(result.Markers, "abc123")
		if !ok {
			t.Fatalf("expected marker abc123, not found")
		}
		if marker.Filepath != filepath.Join(dir, "main.go") {
			t.Fatalf("filepath = %q, want %q", marker.Filepath, filepath.Join(dir, "main.go"))
		}
		if marker.Hash == "" {
			t.Fatalf("expected non-empty hash")
		}
	})

	t.Run("one_code_file_many_markers", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "main.go", `package main

// #F m1
func handlerA() {
	a()
}

// #F m2
func handlerB() {
	b()
}
`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
		for _, id := range []string{"m1", "m2"} {
			if _, ok := findScanResultMarker(result.Markers, id); !ok {
				t.Fatalf("expected marker %s, not found", id)
			}
		}
	})

	t.Run("many_code_files", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "a.go", `// #F ma
func a() { x() }
`)
		writeCodeFile(t, dir, "b.go", `// #F mb
func b() { y() }
`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
		for _, id := range []string{"ma", "mb"} {
			if _, ok := findScanResultMarker(result.Markers, id); !ok {
				t.Fatalf("expected marker %s, not found", id)
			}
		}
	})

	t.Run("duplicate_marker_shortcodes_error", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "a.go", `// #F dup
func a() { }
`)
		writeCodeFile(t, dir, "b.go", `// #F dup
func b() { }
`)

		scanner := NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for duplicate marker shortcode")
		}
	})

	t.Run("marker_hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "main.go", `// #F abc
func handler() {
	doSomething()
}
`)

		scanner := NewFileScanner(dir)
		result1, err := scanner.Scan()
		assertNoError(t, err)

		result2, err := scanner.Scan()
		assertNoError(t, err)

		m1, _ := findScanResultMarker(result1.Markers, "abc")
		m2, _ := findScanResultMarker(result2.Markers, "abc")

		if m1.Hash != m2.Hash {
			t.Fatalf("hash not deterministic: %q vs %q", m1.Hash, m2.Hash)
		}
	})
}

func TestScannerMarkerHashingWindow(t *testing.T) {
	t.Run("hashes_exactly_10_lines_from_marker", func(t *testing.T) {
		dir := t.TempDir()
		code := `// #F abc
line2
line3
line4
line5
line6
line7
line8
line9
line10
line11
line12
`
		writeCodeFile(t, dir, "main.go", code)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		marker, _ := findScanResultMarker(result.Markers, "abc")

		expectedContent := `line2
line3
line4
line5
line6
line7
line8
line9
line10
line11
`
		expectedHash := expectedSha1Hex(expectedContent)
		if marker.Hash != expectedHash {
			t.Fatalf("hash = %q, want %q (content hash of 10 lines after marker)", marker.Hash, expectedHash)
		}
	})

	t.Run("fewer_than_10_lines_hashes_all_remaining", func(t *testing.T) {
		dir := t.TempDir()
		code := `// #F abc
line2
line3
`
		writeCodeFile(t, dir, "main.go", code)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		marker, _ := findScanResultMarker(result.Markers, "abc")

		expectedContent := `line2
line3
`
		expectedHash := expectedSha1Hex(expectedContent)
		if marker.Hash != expectedHash {
			t.Fatalf("hash = %q, want %q", marker.Hash, expectedHash)
		}
	})
}

func TestScannerMixedSpecsAndMarkers(t *testing.T) {
	t.Run("specs_and_markers_across_multiple_files", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecFile(t, dir, "specs.pin.xml", `<specs>
			<spec id="validate_input">input must be validated</spec>
			<spec id="auth_check">auth must be checked</spec>
		</specs>`)
		writeCodeFile(t, dir, "auth.go", `// #F m1
func auth() { check() }
`)
		writeCodeFile(t, dir, "input.go", `// #F m2
func validate() { check() }
`)

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
	})
}

func TestScannerIgnoresNonPinXmlNonCodeFiles(t *testing.T) {
	t.Run("ignores_txt_md_json_files", func(t *testing.T) {
		dir := t.TempDir()
		writeCodeFile(t, dir, "notes.txt", "// #F should_not_find\n")
		writeCodeFile(t, dir, "readme.md", "// #F should_not_find_either\n")
		writeCodeFile(t, dir, "data.json", "// #F nope\n")

		scanner := NewFileScanner(dir)
		result, err := scanner.Scan()
		assertNoError(t, err)

		if len(result.Markers) != 0 {
			t.Fatalf("expected 0 markers from non-code files, got %d", len(result.Markers))
		}
		if len(result.Specs) != 0 {
			t.Fatalf("expected 0 specs, got %d", len(result.Specs))
		}
	})
}
