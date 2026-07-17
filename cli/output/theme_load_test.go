package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCustomTheme(t *testing.T) {
	t.Run("file_not_exist_returns_ErrNotExist", func(t *testing.T) {
		dir := t.TempDir()
		_, err := LoadCustomTheme(dir)
		if !os.IsNotExist(err) {
			t.Fatalf("expected os.ErrNotExist, got: %v", err)
		}
	})

	t.Run("valid_full_theme", func(t *testing.T) {
		dir := t.TempDir()
		xml := `<theme>
  <element id="marker_id" color="94" bold="true"/>
  <element id="spec_id" color="95" bold="true"/>
  <element id="filepath" dim="true"/>
  <element id="line_number" dim="true"/>
  <element id="hash" dim="true"/>
  <element id="status_ok" color="92"/>
  <element id="status_warn" color="93"/>
  <element id="status_error" color="91"/>
  <element id="section_header" bold="true"/>
  <element id="command" color="92"/>
  <element id="hint" color="96"/>
  <element id="diff_add" color="92"/>
  <element id="diff_remove" color="91"/>
  <element id="diff_hunk" color="96" bold="true"/>
  <element id="code_comment" dim="true"/>
  <element id="code_string" color="92"/>
  <element id="code_keyword" color="96"/>
  <element id="code_number" color="93"/>
</theme>`
		os.MkdirAll(filepath.Join(dir, ".drift"), 0755)
		os.WriteFile(filepath.Join(dir, ".drift", "theme.xml"), []byte(xml), 0644)

		theme, err := LoadCustomTheme(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if theme.Name != "custom" {
			t.Errorf("Name = %q, want %q", theme.Name, "custom")
		}
		if theme.MarkerID.Color != "94" || !theme.MarkerID.Bold {
			t.Errorf("MarkerID = %+v, want {Color:94 Bold:true}", theme.MarkerID)
		}
		if !theme.Filepath.Dim {
			t.Errorf("Filepath.Dim = false, want true")
		}
		if theme.StatusOK.Color != "92" {
			t.Errorf("StatusOK.Color = %q, want %q", theme.StatusOK.Color, "92")
		}
		if theme.DiffHunk.Color != "96" || !theme.DiffHunk.Bold {
			t.Errorf("DiffHunk = %+v, want {Color:96 Bold:true}", theme.DiffHunk)
		}
	})

	t.Run("missing_element_returns_error_naming_it", func(t *testing.T) {
		dir := t.TempDir()
		xml := `<theme>
  <element id="marker_id" color="94" bold="true"/>
  <element id="spec_id" color="95" bold="true"/>
</theme>`
		os.MkdirAll(filepath.Join(dir, ".drift"), 0755)
		os.WriteFile(filepath.Join(dir, ".drift", "theme.xml"), []byte(xml), 0644)

		_, err := LoadCustomTheme(dir)
		if err == nil {
			t.Fatal("expected error for missing elements, got nil")
		}
		// Error should name the first missing element
		if !contains(err.Error(), "filepath") {
			t.Errorf("error should mention 'filepath' (first missing element), got: %s", err.Error())
		}
	})

	t.Run("256_color_values", func(t *testing.T) {
		dir := t.TempDir()
		xml := `<theme>
  <element id="marker_id" color="38;5;33" bold="true"/>
  <element id="spec_id" color="38;5;162" bold="true"/>
  <element id="filepath" dim="true"/>
  <element id="line_number" dim="true"/>
  <element id="hash" dim="true"/>
  <element id="status_ok" color="38;5;37"/>
  <element id="status_warn" color="38;5;136"/>
  <element id="status_error" color="38;5;124"/>
  <element id="section_header" bold="true"/>
  <element id="command" color="38;5;37"/>
  <element id="hint" color="38;5;44"/>
  <element id="diff_add" color="38;5;37"/>
  <element id="diff_remove" color="38;5;124"/>
  <element id="diff_hunk" color="38;5;44" bold="true"/>
  <element id="code_comment" dim="true"/>
  <element id="code_string" color="38;5;100"/>
  <element id="code_keyword" color="38;5;33"/>
  <element id="code_number" color="38;5;136"/>
</theme>`
		os.MkdirAll(filepath.Join(dir, ".drift"), 0755)
		os.WriteFile(filepath.Join(dir, ".drift", "theme.xml"), []byte(xml), 0644)

		theme, err := LoadCustomTheme(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if theme.MarkerID.Color != "38;5;33" {
			t.Errorf("MarkerID.Color = %q, want %q", theme.MarkerID.Color, "38;5;33")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
