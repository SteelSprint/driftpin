package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"drift/cli/commands"
	"drift/cli/output"
	"drift/internal/fileio"
)

// beginConfigTestSession creates a project dir + .drift/ and returns a
// commands.Context with Args preset and a live Session for I/O.
func beginConfigTestSession(t *testing.T, args []string) (commands.Context, func()) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".drift"), 0755); err != nil {
		t.Fatal(err)
	}
	sess, err := fileio.Begin(dir)
	if err != nil {
		t.Fatalf("fileio.Begin: %v", err)
	}
	ctx := commands.Context{Args: args, Dir: dir, Sess: sess}
	return ctx, func() { sess.Close() }
}

func TestConfigTheme(t *testing.T) {
	t.Run("set_theme_writes_user_settings", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "theme", "gruvbox"})
		defer cleanup()

		result, code := commands.ConfigCommand{}.Run(ctx)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, result: %+v", code, result)
		}

		// Verify user-settings.xml was written
		settings, err := output.LoadUserSettings(ctx.Sess)
		if err != nil {
			t.Fatalf("LoadUserSettings: %v", err)
		}
		if settings.Theme != "gruvbox" {
			t.Errorf("settings.Theme = %q, want %q", settings.Theme, "gruvbox")
		}
	})

	t.Run("print_theme_no_args", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "theme"})
		defer cleanup()

		result, code := commands.ConfigCommand{}.Run(ctx)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}

		ok, okType := result.(output.OkResult)
		if !okType {
			t.Fatalf("expected OkResult, got %T", result)
		}
		if !strings.Contains(ok.Message, "default") {
			t.Errorf("message should mention 'default', got: %s", ok.Message)
		}
	})

	t.Run("print_theme_after_setting", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "theme", "nord"})
		defer cleanup()

		// Set theme first
		commands.ConfigCommand{}.Run(ctx)
		// Now read it
		ctx.Args = []string{"config", "theme"}
		result, code := commands.ConfigCommand{}.Run(ctx)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}

		ok, _ := result.(output.OkResult)
		if !strings.Contains(ok.Message, "nord") {
			t.Errorf("message should mention 'nord', got: %s", ok.Message)
		}
	})

	t.Run("invalid_theme_name_rejected", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "theme", "nonexistent"})
		defer cleanup()

		result, code := commands.ConfigCommand{}.Run(ctx)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}

		errResult, _ := result.(output.ErrorResult)
		if !strings.Contains(errResult.Message, "nonexistent") {
			t.Errorf("error should mention the invalid name, got: %s", errResult.Message)
		}
		if !strings.Contains(errResult.Message, "default") {
			t.Errorf("error should list available themes including 'default', got: %s", errResult.Message)
		}
	})

	t.Run("no_subcommand_usage_error", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config"})
		defer cleanup()

		_, code := commands.ConfigCommand{}.Run(ctx)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1 for bare config", code)
		}
	})

	t.Run("unknown_config_key", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "unknown_key"})
		defer cleanup()

		_, code := commands.ConfigCommand{}.Run(ctx)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})

	t.Run("gitignore_created_on_first_setting", func(t *testing.T) {
		ctx, cleanup := beginConfigTestSession(t, []string{"config", "theme", "dracula"})
		defer cleanup()

		commands.ConfigCommand{}.Run(ctx)

		gitignorePath := filepath.Join(ctx.Dir, ".drift", ".gitignore")
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf(".drift/.gitignore should exist: %v", err)
		}
		if !strings.Contains(string(data), "user-settings.xml") {
			t.Errorf(".drift/.gitignore should list user-settings.xml, got: %s", data)
		}
	})
}
