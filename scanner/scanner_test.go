package scanner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"driftpin/internal/testutil"
	"driftpin/scanner"
)

func writeMainPin(t *testing.T, dir, content string) {
	t.Helper()
	testutil.WriteSpecFile(t, dir, "main.pin.xml", content)
}

func writeModuleFile(t *testing.T, dir, name, content string) {
	t.Helper()
	testutil.WriteSpecFile(t, dir, name, content)
}

func assertScanError(t *testing.T, scanner *scanner.FileScanner, errContains string) {
	t.Helper()
	_, err := scanner.Scan()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", errContains)
	}
	if strings.Contains(err.Error(), errContains) {
		return
	}
	t.Fatalf("expected error containing %q, got %q", errContains, err.Error())
}

func TestScannerEmptyProject(t *testing.T) {
	t.Run("missing_main_pin_xml_errors", func(t *testing.T) {
		dir := t.TempDir()
		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "main.pin.xml")
	})

	t.Run("empty_main_returns_no_specs", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)
		if len(result.Specs) != 0 {
			t.Fatalf("expected 0 specs, got %d", len(result.Specs))
		}
	})

	t.Run("empty_main_still_discovers_markers", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerLine("m1")+`
func a() {}
`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)
		if len(result.Markers) != 1 {
			t.Fatalf("expected 1 marker, got %d", len(result.Markers))
		}
	})

	t.Run("specs_wrapper_rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <specs>
    <spec id="validate">input must be validated</spec>
  </specs>
</main>`)

		scanner := scanner.NewFileScanner(dir)
		_, err := scanner.Scan()
		if err == nil {
			t.Fatalf("expected error for <specs> wrapper, got nil")
		}
		if !strings.Contains(err.Error(), "<specs>") {
			t.Fatalf("error should mention <specs> wrapper, got: %s", err.Error())
		}
	})
}

func TestScannerSpecDiscovery(t *testing.T) {
	t.Run("main_with_direct_specs_implicit_main_module", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(result.Specs))
		}
		spec, ok := testutil.FindScanResultSpec(result.Specs, "main.validate_input")
		if !ok {
			t.Fatalf("expected spec main.validate_input, not found. specs: %+v", result.Specs)
		}
		if spec.Filepath != filepath.Join(dir, "main.pin.xml") {
			t.Fatalf("filepath = %q, want %q", spec.Filepath, filepath.Join(dir, "main.pin.xml"))
		}
		if spec.Hash == "" {
			t.Fatalf("expected non-empty hash")
		}
	})

	t.Run("main_imports_one_module", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./core.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "core.pin.xml", `<module name="core">
  <spec id="validate">Validation must reject duplicates.</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(result.Specs))
		}
		spec, ok := testutil.FindScanResultSpec(result.Specs, "core.validate")
		if !ok {
			t.Fatalf("expected spec core.validate, not found. specs: %+v", result.Specs)
		}
		if spec.Filepath != filepath.Join(dir, "core.pin.xml") {
			t.Fatalf("filepath = %q, want %q", spec.Filepath, filepath.Join(dir, "core.pin.xml"))
		}
	})

	t.Run("main_imports_multiple_modules", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./auth.pin.xml" />
  <import path="./api.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "auth.pin.xml", `<module name="auth">
  <spec id="login">Login required.</spec>
</module>`)
		writeModuleFile(t, dir, "api.pin.xml", `<module name="api">
  <spec id="endpoint">API endpoint must be versioned.</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "auth.login"); !ok {
			t.Fatalf("expected spec auth.login, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "api.endpoint"); !ok {
			t.Fatalf("expected spec api.endpoint, not found")
		}
	})

	t.Run("main_with_direct_specs_and_imports", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./core.pin.xml" />
  <spec id="app_entry">App entry point must validate config.</spec>
</main>`)
		writeModuleFile(t, dir, "core.pin.xml", `<module name="core">
  <spec id="validate">Validation must reject duplicates.</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "main.app_entry"); !ok {
			t.Fatalf("expected spec main.app_entry, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "core.validate"); !ok {
			t.Fatalf("expected spec core.validate, not found")
		}
	})

	t.Run("one_module_many_specs", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./core.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "core.pin.xml", `<module name="core">
  <spec id="validate">validate spec</spec>
  <spec id="authenticate">auth spec</spec>
  <spec id="log">log spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(result.Specs))
		}
		for _, id := range []string{"core.validate", "core.authenticate", "core.log"} {
			if _, ok := testutil.FindScanResultSpec(result.Specs, id); !ok {
				t.Fatalf("expected spec %s, not found", id)
			}
		}
	})

	t.Run("spec_missing_id_attribute_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec>no id here</spec>
</main>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "id")
	})

	t.Run("duplicate_spec_ids_within_same_module_error", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="dup">first</spec>
  <spec id="dup">second</spec>
</main>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "duplicate")
	})

	t.Run("same_spec_id_in_different_modules_ok", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./a.pin.xml" />
  <import path="./b.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "a.pin.xml", `<module name="a">
  <spec id="shared">a version</spec>
</module>`)
		writeModuleFile(t, dir, "b.pin.xml", `<module name="b">
  <spec id="shared">b version</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "a.shared"); !ok {
			t.Fatalf("expected spec a.shared, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "b.shared"); !ok {
			t.Fatalf("expected spec b.shared, not found")
		}
	})

	t.Run("hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">deterministic content</spec>
</main>`)

		scanner := scanner.NewFileScanner(dir)
		result1, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		result2, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		spec1, _ := testutil.FindScanResultSpec(result1.Specs, "main.s1")
		spec2, _ := testutil.FindScanResultSpec(result2.Specs, "main.s1")

		if spec1.Hash != spec2.Hash {
			t.Fatalf("hash not deterministic: %q vs %q", spec1.Hash, spec2.Hash)
		}
	})
}

func TestScannerImportGraph(t *testing.T) {
	t.Run("transitive_imports_all_loaded", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./a.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "a.pin.xml", `<module name="a">
  <import path="./b.pin.xml" />
  <spec id="spec_a">a spec</spec>
</module>`)
		writeModuleFile(t, dir, "b.pin.xml", `<module name="b">
  <spec id="spec_b">b spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(result.Specs))
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "a.spec_a"); !ok {
			t.Fatalf("expected spec a.spec_a, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "b.spec_b"); !ok {
			t.Fatalf("expected spec b.spec_b, not found")
		}
	})

	t.Run("diamond_imports_deduplicated", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./a.pin.xml" />
  <import path="./b.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "a.pin.xml", `<module name="a">
  <import path="./shared.pin.xml" />
  <spec id="spec_a">a spec</spec>
</module>`)
		writeModuleFile(t, dir, "b.pin.xml", `<module name="b">
  <import path="./shared.pin.xml" />
  <spec id="spec_b">b spec</spec>
</module>`)
		writeModuleFile(t, dir, "shared.pin.xml", `<module name="shared">
  <spec id="spec_shared">shared spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 3 {
			t.Fatalf("expected 3 specs (shared loaded once), got %d: %+v", len(result.Specs), result.Specs)
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "a.spec_a"); !ok {
			t.Fatalf("expected spec a.spec_a, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "b.spec_b"); !ok {
			t.Fatalf("expected spec b.spec_b, not found")
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "shared.spec_shared"); !ok {
			t.Fatalf("expected spec shared.spec_shared, not found")
		}
	})

	t.Run("duplicate_module_names_error", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./a.pin.xml" />
  <import path="./b.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "a.pin.xml", `<module name="dup">
  <spec id="spec_a">a spec</spec>
</module>`)
		writeModuleFile(t, dir, "b.pin.xml", `<module name="dup">
  <spec id="spec_b">b spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "duplicate module")
	})

	t.Run("cycle_detection_errors_with_trace", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./a.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "a.pin.xml", `<module name="a">
  <import path="./b.pin.xml" />
  <spec id="spec_a">a spec</spec>
</module>`)
		writeModuleFile(t, dir, "b.pin.xml", `<module name="b">
  <import path="./a.pin.xml" />
  <spec id="spec_b">b spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "cycle")
	})

	t.Run("import_path_not_found_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./nonexistent.pin.xml" />
</main>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "nonexistent.pin.xml")
	})

	t.Run("imports_in_subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "sub")
		os.Mkdir(subdir, 0755)
		writeMainPin(t, dir, `<main>
  <import path="./sub/nested.pin.xml" />
</main>`)
		writeModuleFile(t, subdir, "nested.pin.xml", `<module name="nested">
  <spec id="deep">deep spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		result, err := scanner.Scan()
		testutil.AssertNoError(t, err)

		if len(result.Specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(result.Specs))
		}
		if _, ok := testutil.FindScanResultSpec(result.Specs, "nested.deep"); !ok {
			t.Fatalf("expected spec nested.deep, not found")
		}
	})

	t.Run("module_without_name_attribute_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./core.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "core.pin.xml", `<module>
  <spec id="validate">validate spec</spec>
</module>`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "name")
	})
}

func TestScannerMarkerDiscovery(t *testing.T) {
	t.Run("one_code_file_one_marker", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
		testutil.WriteCodeFile(t, dir, "a.go", testutil.MarkerLine("dup")+`
func a() { }
`)
		testutil.WriteCodeFile(t, dir, "b.go", testutil.MarkerLine("dup")+`
func b() { }
`)

		scanner := scanner.NewFileScanner(dir)
		assertScanError(t, scanner, "duplicate marker")
	})

	t.Run("marker_hash_is_sha1_deterministic", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main>
  <import path="./specs.pin.xml" />
</main>`)
		writeModuleFile(t, dir, "specs.pin.xml", `<module name="specs">
  <spec id="validate_input">input must be validated</spec>
  <spec id="auth_check">auth must be checked</spec>
</module>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
		writeMainPin(t, dir, `<main></main>`)
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
}

func TestScannerIgnoresNonPinXmlNonCodeFiles(t *testing.T) {
	t.Run("ignores_txt_md_json_files", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
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
