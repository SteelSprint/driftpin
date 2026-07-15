package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"driftpin/internal/testutil"
	"driftpin/scanner"
)

func TestScannerEmptyProject(t *testing.T) {
	t.Run("no_files_returns_empty", func(t *testing.T) {
		dir := t.TempDir()
		scanner := scanner.NewFileScanner(dir)

		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)
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
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(result.Specs))
		}
		spec, ok := testutil.FindScanResultSpec(result.Specs, "validate_input")
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
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", `<specs>
			<spec id="validate_input">input must be validated</spec>
			<spec id="auth_check">auth must be checked</spec>
			<spec id="log_request">request must be logged</spec>
		</specs>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(result.Specs))
		}
		for _, id := range []string{"validate_input", "auth_check", "log_request"} {
			if _, ok := testutil.FindScanResultSpec(result.Specs, id); !ok {
				t.Fatalf("expected spec %s, not found", id)
			}
		}
	})

	t.Run("many_spec_files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteSpecFile(t, dir, "auth.pin.xml", `<specs><spec id="auth">auth spec</spec></specs>`)
		testutil.WriteSpecFile(t, dir, "api.pin.xml", `<specs><spec id="api">api spec</spec></specs>`)
		subdir := filepath.Join(dir, "sub")
		os.Mkdir(subdir, 0755)
		testutil.WriteSpecFile(t, subdir, "nested.pin.xml", `<specs><spec id="nested">nested spec</spec></specs>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(result.Specs))
		}
		for _, id := range []string{"auth", "api", "nested"} {
			if _, ok := testutil.FindScanResultSpec(result.Specs, id); !ok {
				t.Fatalf("expected spec %s, not found", id)
			}
		}
	})

	t.Run("spec_missing_id_attribute_errors", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", `<specs><spec>no id here</spec></specs>`)

		scanner := scanner.NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for spec missing id")
		}
	})

	t.Run("duplicate_spec_ids_error", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteSpecFile(t, dir, "a.pin.xml", `<specs><spec id="dup">first</spec></specs>`)
		testutil.WriteSpecFile(t, dir, "b.pin.xml", `<specs><spec id="dup">second</spec></specs>`)

		scanner := scanner.NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for duplicate spec id")
		}
	})

	t.Run("hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		content := `<specs><spec id="s1">deterministic content</spec></specs>`
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", content)

		scanner := scanner.NewFileScanner(dir)
		result1, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		result2, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		spec1, _ := testutil.FindScanResultSpec(result1.Specs, "s1")
		spec2, _ := testutil.FindScanResultSpec(result2.Specs, "s1")

		if spec1.Hash != spec2.Hash {
			t.Fatalf("hash not deterministic: %q vs %q", spec1.Hash, spec2.Hash)
		}
	})
}

func TestScannerMarkerDiscovery(t *testing.T) {
	t.Run("one_code_file_one_marker", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "main.go", `package main

`+testutil.MarkerLine("abc123")+`
func handleRequest() {
	doSomething()
}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
		marker, ok := testutil.FindScanResultMarker(result.Markers, "abc123")
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
		testutil.WriteCodeFile(t, dir, "main.go", `package main

`+testutil.MarkerLine("m1")+`
func handlerA() {
	a()
}

`+testutil.MarkerLine("m2")+`
func handlerB() {
	b()
}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
		for _, id := range []string{"m1", "m2"} {
			if _, ok := testutil.FindScanResultMarker(result.Markers, id); !ok {
				t.Fatalf("expected marker %s, not found", id)
			}
		}
	})

	t.Run("many_code_files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "a.go", testutil.MarkerLine("ma")+`
func a() { x() }
`)
		testutil.WriteCodeFile(t, dir, "b.go", testutil.MarkerLine("mb")+`
func b() { y() }
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
		for _, id := range []string{"ma", "mb"} {
			if _, ok := testutil.FindScanResultMarker(result.Markers, id); !ok {
				t.Fatalf("expected marker %s, not found", id)
			}
		}
	})

	t.Run("duplicate_marker_shortcodes_error", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "a.go", testutil.MarkerLine("dup")+`
func a() { }
`)
		testutil.WriteCodeFile(t, dir, "b.go", testutil.MarkerLine("dup")+`
func b() { }
`)

		scanner := scanner.NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for duplicate marker shortcode")
		}
	})

	t.Run("marker_hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("abc")+`
func handler() {
	doSomething()
}
`)

		scanner := scanner.NewFileScanner(dir)
		result1, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		result2, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		m1, _ := testutil.FindScanResultMarker(result1.Markers, "abc")
		m2, _ := testutil.FindScanResultMarker(result2.Markers, "abc")

		if m1.Hash != m2.Hash {
			t.Fatalf("hash not deterministic: %q vs %q", m1.Hash, m2.Hash)
		}
	})
}

func TestScannerMarkerHashingWindow(t *testing.T) {
	t.Run("hashes_exactly_10_lines_from_marker", func(t *testing.T) {
		dir := t.TempDir()
		code := testutil.MarkerLine("abc") + `
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
		testutil.WriteCodeFile(t, dir, "main.go", code)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		marker, _ := testutil.FindScanResultMarker(result.Markers, "abc")

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
		expectedHash := testutil.ExpectedSha1Hex(expectedContent)
		if marker.Hash != expectedHash {
			t.Fatalf("hash = %q, want %q (content hash of 10 lines after marker)", marker.Hash, expectedHash)
		}
	})

	t.Run("fewer_than_10_lines_hashes_all_remaining", func(t *testing.T) {
		dir := t.TempDir()
		code := testutil.MarkerLine("abc") + `
line2
line3
`
		testutil.WriteCodeFile(t, dir, "main.go", code)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		marker, _ := testutil.FindScanResultMarker(result.Markers, "abc")

		expectedContent := `line2
line3
`
		expectedHash := testutil.ExpectedSha1Hex(expectedContent)
		if marker.Hash != expectedHash {
			t.Fatalf("hash = %q, want %q", marker.Hash, expectedHash)
		}
	})
}

func TestScannerMixedSpecsAndMarkers(t *testing.T) {
	t.Run("specs_and_markers_across_multiple_files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", `<specs>
			<spec id="validate_input">input must be validated</spec>
			<spec id="auth_check">auth must be checked</spec>
		</specs>`)
		testutil.WriteCodeFile(t, dir, "auth.go", testutil.MarkerLine("m1")+`
func auth() { check() }
`)
		testutil.WriteCodeFile(t, dir, "input.go", testutil.MarkerLine("m2")+`
func validate() { check() }
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
	})
}

func TestScannerDriftIgnore(t *testing.T) {
	t.Run("no_drift_ignore_scans_all", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("keep")+`
func a() {}
`)
		testutil.WriteCodeFile(t, dir, "main_test.go", testutil.MarkerLine("drop")+`
func b() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers without drift.ignore, got %d", len(result.Markers))
		}
	})

	t.Run("star_test_go_excludes_test_files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteIgnoreFile(t, dir, "*_test.go\n")
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("keep")+`
func a() {}
`)
		testutil.WriteCodeFile(t, dir, "main_test.go", testutil.MarkerLine("drop")+`
func b() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "keep"); !ok {
			t.Fatalf("expected marker 'keep', not found")
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "drop"); ok {
			t.Fatalf("marker 'drop' should have been excluded")
		}
	})

	t.Run("trailing_slash_skips_directory_subtree", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteIgnoreFile(t, dir, ".git/\n")
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("keep")+`
func a() {}
`)
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0755)
		testutil.WriteCodeFile(t, gitDir, "hook.go", testutil.MarkerLine("drop")+`
func b() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "keep"); !ok {
			t.Fatalf("expected marker 'keep', not found")
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "drop"); ok {
			t.Fatalf("marker 'drop' from .git/ should have been excluded")
		}
	})

	t.Run("comments_and_empty_lines_ignored", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteIgnoreFile(t, dir, "# this is a comment\n\n*_test.go\n# another comment\n")
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("keep")+`
func a() {}
`)
		testutil.WriteCodeFile(t, dir, "main_test.go", testutil.MarkerLine("drop")+`
func b() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "keep"); !ok {
			t.Fatalf("expected marker 'keep', not found")
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "drop"); ok {
			t.Fatalf("marker 'drop' should have been excluded")
		}
	})

	t.Run("path_pattern_excludes_specific_file", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteIgnoreFile(t, dir, "sub/skip.go\n")
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("keep")+`
func a() {}
`)
		subDir := filepath.Join(dir, "sub")
		os.Mkdir(subDir, 0755)
		testutil.WriteCodeFile(t, subDir, "skip.go", testutil.MarkerLine("drop")+`
func b() {}
`)
		testutil.WriteCodeFile(t, subDir, "keep.go", testutil.MarkerLine("also_keep")+`
func c() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 2 {
			t.Fatalf("expected 2 markers, got %d", len(result.Markers))
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "drop"); ok {
			t.Fatalf("marker 'drop' should have been excluded by path pattern")
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "keep"); !ok {
			t.Fatalf("expected marker 'keep', not found")
		}
		if _, ok := testutil.FindScanResultMarker(result.Markers, "also_keep"); !ok {
			t.Fatalf("expected marker 'also_keep', not found")
		}
	})

	t.Run("ignore_applies_to_spec_files_too", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteIgnoreFile(t, dir, "*.pin.xml\n")
		testutil.WriteSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="s1">spec content</spec></specs>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 0 {
			t.Fatalf("expected 0 specs (all ignored), got %d", len(result.Specs))
		}
	})
}

func TestScannerIgnoresNonPinXmlNonCodeFiles(t *testing.T) {
	t.Run("ignores_txt_md_json_files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteCodeFile(t, dir, "notes.txt", testutil.MarkerLine("should_not_find")+"\n")
		testutil.WriteCodeFile(t, dir, "readme.md", testutil.MarkerLine("should_not_find_either")+"\n")
		testutil.WriteCodeFile(t, dir, "data.json", testutil.MarkerLine("nope")+"\n")

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Markers) != 0 {
			t.Fatalf("expected 0 markers from non-code files, got %d", len(result.Markers))
		}
		if len(result.Specs) != 0 {
			t.Fatalf("expected 0 specs, got %d", len(result.Specs))
		}
	})
}
