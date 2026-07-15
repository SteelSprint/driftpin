package driftpin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInit(t *testing.T) {
	t.Run("init_creates_drift_pin", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		pinPath := filepath.Join(dir, "drift.pin")
		if _, err := os.Stat(pinPath); os.IsNotExist(err) {
			t.Fatalf("drift.pin not created")
		}
	})

	t.Run("init_then_todo_no_changes", func(t *testing.T) {
		dir := t.TempDir()

		_, code := Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("init failed with non-zero exit code")
		}

		output, code := Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		expected := "No changes detected."
		if output != expected {
			t.Fatalf("output = %q, want %q", output, expected)
		}
	})

	t.Run("init_creates_valid_empty_pin", func(t *testing.T) {
		dir := t.TempDir()
		_, code := Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("init failed")
		}

		store := NewFilePinStore(dir)
		state, err := store.Load()
		assertNoError(t, err)
		assertPinStateEquals(t, state, PinState{})
	})
}

func TestCLITodoWithoutInit(t *testing.T) {
	t.Run("todo_without_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{"todo"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
		if !strings.Contains(strings.ToLower(output), "init") {
			t.Fatalf("error message should mention init, got: %s", output)
		}
	})
}

func TestCLIResetWithoutInit(t *testing.T) {
	t.Run("reset_without_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{"reset", "m1:s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func TestCLINoArgs(t *testing.T) {
	t.Run("no_args_shows_usage", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for no args")
		}
		if !strings.Contains(strings.ToLower(output), "usage") {
			t.Fatalf("output should contain usage, got: %s", output)
		}
	})
}

func TestCLIUnknownCommand(t *testing.T) {
	t.Run("unknown_command_errors", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{"frobnicate"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for unknown command")
		}
		if !strings.Contains(strings.ToLower(output), "unknown") {
			t.Fatalf("output should mention unknown command, got: %s", output)
		}
	})
}

func TestCLIResetBadFormat(t *testing.T) {
	t.Run("reset_without_argument", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)
		output, code := Run([]string{"reset"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("reset_bad_format_no_colon", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)
		output, code := Run([]string{"reset", "no_colon_here"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func TestCLIFullFlowSpecMarkerLinkDrift(t *testing.T) {
	t.Run("init_create_spec_create_marker_todo_no_links_no_drift", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)

		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)
		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomething()
}
`)

		output, code := Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if output != "No changes detected." {
			t.Fatalf("output = %q, want %q", output, "No changes detected.")
		}
	})

	t.Run("link_then_todo_no_drift", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)

		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)
		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomething()
}
`)

		Run([]string{"todo"}, dir)

		output, code := Run([]string{"link", "abc123:validate_input"}, dir)
		if code != 0 {
			t.Fatalf("link failed, exit code = %d, output: %s", code, output)
		}

		output, code = Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if output != "No changes detected." {
			t.Fatalf("output = %q, want %q", output, "No changes detected.")
		}
	})

	t.Run("link_then_modify_code_then_todo_shows_drift", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)

		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)
		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomething()
}
`)

		Run([]string{"todo"}, dir)
		Run([]string{"link", "abc123:validate_input"}, dir)
		Run([]string{"todo"}, dir)

		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomethingElse()
}
`)

		output, code := Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "1 marker has unchecked changes") {
			t.Fatalf("output should mention 1 marker with unchecked changes, got: %s", output)
		}
		if !strings.Contains(output, "abc123") {
			t.Fatalf("output should contain marker id abc123, got: %s", output)
		}
		if !strings.Contains(output, "validate_input") {
			t.Fatalf("output should contain spec id validate_input, got: %s", output)
		}
		if !strings.Contains(output, "drift reset abc123:validate_input") {
			t.Fatalf("output should contain reset command, got: %s", output)
		}
	})

	t.Run("drift_then_reset_clears_drift", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)

		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="validate_input">input must be validated</spec></specs>`)
		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomething()
}
`)

		Run([]string{"todo"}, dir)
		Run([]string{"link", "abc123:validate_input"}, dir)
		Run([]string{"todo"}, dir)

		writeCodeFile(t, dir, "main.go", `// #F abc123
func handleRequest() {
	doSomethingElse()
}
`)

		Run([]string{"todo"}, dir)

		_, code := Run([]string{"reset", "abc123:validate_input"}, dir)
		if code != 0 {
			t.Fatalf("reset failed with non-zero exit code")
		}

		output, code := Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if output != "No changes detected." {
			t.Fatalf("output = %q, want %q", output, "No changes detected.")
		}
	})
}

func TestCLILinkErrors(t *testing.T) {
	t.Run("link_nonexistent_marker", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)
		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="s1">spec</spec></specs>`)

		output, code := Run([]string{"link", "nonexistent:s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_nonexistent_spec", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)
		writeCodeFile(t, dir, "main.go", `// #F m1
func a() {}
`)

		output, code := Run([]string{"link", "m1:nonexistent"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_duplicate", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)
		writeSpecFile(t, dir, "specs.pin.xml", `<specs><spec id="s1">spec</spec></specs>`)
		writeCodeFile(t, dir, "main.go", `// #F m1
func a() {}
`)

		Run([]string{"todo"}, dir)
		Run([]string{"link", "m1:s1"}, dir)

		output, code := Run([]string{"link", "m1:s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for duplicate link, got 0, output: %s", output)
		}
	})

	t.Run("link_bad_format", func(t *testing.T) {
		dir := t.TempDir()
		Run([]string{"init"}, dir)

		output, code := Run([]string{"link", "no_colon"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_without_init", func(t *testing.T) {
		dir := t.TempDir()
		output, code := Run([]string{"link", "m1:s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}
