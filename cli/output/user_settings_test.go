package output

import (
	"os"
	"path/filepath"
	"testing"

	"drift/internal/fileio"
)

// beginSettingsSession creates a project dir + .drift/ and returns a Session
// rooted there. Closed via t.Cleanup.
func beginSettingsSession(t *testing.T, dir string) *fileio.Session {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".drift"), 0755); err != nil {
		t.Fatal(err)
	}
	sess, err := fileio.Begin(dir)
	if err != nil {
		t.Fatalf("fileio.Begin: %v", err)
	}
	t.Cleanup(func() { sess.Close() })
	return sess
}

func TestLoadUserSettings(t *testing.T) {
	t.Run("not_exist_returns_empty", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)
		settings, err := LoadUserSettings(sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.Theme != "" {
			t.Errorf("Theme = %q, want empty", settings.Theme)
		}
	})

	t.Run("valid_theme", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)
		os.WriteFile(filepath.Join(dir, ".drift", "user-settings.xml"),
			[]byte(`<settings><theme>gruvbox</theme></settings>`), 0644)

		settings, err := LoadUserSettings(sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.Theme != "gruvbox" {
			t.Errorf("Theme = %q, want %q", settings.Theme, "gruvbox")
		}
	})

	t.Run("empty_theme_element", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)
		os.WriteFile(filepath.Join(dir, ".drift", "user-settings.xml"),
			[]byte(`<settings><theme></theme></settings>`), 0644)

		settings, err := LoadUserSettings(sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.Theme != "" {
			t.Errorf("Theme = %q, want empty", settings.Theme)
		}
	})

	t.Run("malformed_xml_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)
		os.WriteFile(filepath.Join(dir, ".drift", "user-settings.xml"),
			[]byte(`<settings><theme>gruvbox`), 0644)

		_, err := LoadUserSettings(sess)
		if err == nil {
			t.Fatal("expected error for malformed XML, got nil")
		}
	})
}

func TestSaveUserSettings(t *testing.T) {
	t.Run("write_and_read_back", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)

		err := SaveUserSettings(sess, UserSettings{Theme: "nord"})
		if err != nil {
			t.Fatalf("SaveUserSettings failed: %v", err)
		}

		settings, err := LoadUserSettings(sess)
		if err != nil {
			t.Fatalf("LoadUserSettings failed: %v", err)
		}
		if settings.Theme != "nord" {
			t.Errorf("Theme = %q, want %q", settings.Theme, "nord")
		}
	})

	t.Run("overwrite_existing", func(t *testing.T) {
		dir := t.TempDir()
		sess := beginSettingsSession(t, dir)

		SaveUserSettings(sess, UserSettings{Theme: "gruvbox"})
		SaveUserSettings(sess, UserSettings{Theme: "dracula"})

		settings, _ := LoadUserSettings(sess)
		if settings.Theme != "dracula" {
			t.Errorf("Theme = %q, want %q after overwrite", settings.Theme, "dracula")
		}
	})

	t.Run("creates_drift_dir_if_missing", func(t *testing.T) {
		dir := t.TempDir()
		// Don't create .drift/ — Session.Begin creates it
		sess, err := fileio.Begin(dir)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		defer sess.Close()
		err = SaveUserSettings(sess, UserSettings{Theme: "solarized-dark"})
		if err != nil {
			t.Fatalf("SaveUserSettings should succeed via Session, got: %v", err)
		}
		settings, _ := LoadUserSettings(sess)
		if settings.Theme != "solarized-dark" {
			t.Errorf("Theme = %q, want %q", settings.Theme, "solarized-dark")
		}
	})
}
