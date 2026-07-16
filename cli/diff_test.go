package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"driftpin/cli"
	"driftpin/internal/testutil"
)

func setupDiffProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeMainPin(t, dir, `<main>
<spec id="s1">Spec content line 1</spec>
</main>`)

	testutil.WriteCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {
	// body
}
`+testutil.MarkerEnd("m1")+`
`)

	_, code := cli.Run([]string{"init"}, dir)
	if code != 0 {
		t.Fatalf("init failed")
	}
	_, code = cli.Run([]string{"link", "m1", "main.s1"}, dir)
	if code != 0 {
		t.Fatalf("link failed")
	}
	return dir
}

func TestCLIDiffInSync(t *testing.T) {
	t.Run("diff_in_sync_shows_in_sync", func(t *testing.T) {
		dir := setupDiffProject(t)
		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "in sync") {
			t.Fatalf("expected 'in sync' in output:\n%s", output)
		}
	})
}

func TestCLIDiffSpecDrifted(t *testing.T) {
	t.Run("diff_shows_spec_changes", func(t *testing.T) {
		dir := setupDiffProject(t)

		writeMainPin(t, dir, `<main>
<spec id="s1">CHANGED spec content</spec>
</main>`)

		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "CHANGED spec content") {
			t.Fatalf("expected current content in diff:\n%s", output)
		}
		if !strings.Contains(output, "Spec content line 1") {
			t.Fatalf("expected baseline content in diff:\n%s", output)
		}
		if !strings.Contains(output, "+CHANGED spec content") {
			t.Fatalf("expected +CHANGED line:\n%s", output)
		}
		if !strings.Contains(output, "-Spec content line 1") {
			t.Fatalf("expected -Spec content line:\n%s", output)
		}
	})
}

func TestCLIDiffMarkerDrifted(t *testing.T) {
	t.Run("diff_shows_marker_changes", func(t *testing.T) {
		dir := setupDiffProject(t)

		testutil.WriteCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {
	// CHANGED body
}
`+testutil.MarkerEnd("m1")+`
`)

		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "CHANGED body") {
			t.Fatalf("expected changed content in diff:\n%s", output)
		}
		if !strings.Contains(output, "+") {
			t.Fatalf("expected + lines:\n%s", output)
		}
		if !strings.Contains(output, "-") {
			t.Fatalf("expected - lines:\n%s", output)
		}
	})
}

func TestCLIDiffOneArgMarker(t *testing.T) {
	t.Run("diff_one_arg_marker_expands", func(t *testing.T) {
		dir := setupDiffProject(t)

		writeMainPin(t, dir, `<main>
<spec id="s1">CHANGED</spec>
</main>`)

		output, code := cli.Run([]string{"diff", "m1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "Spec:") {
			t.Fatalf("expected Spec: header:\n%s", output)
		}
		if !strings.Contains(output, "Marker:") {
			t.Fatalf("expected Marker: header:\n%s", output)
		}
	})
}

func TestCLIDiffOneArgSpec(t *testing.T) {
	t.Run("diff_one_arg_spec_expands", func(t *testing.T) {
		dir := setupDiffProject(t)

		writeMainPin(t, dir, `<main>
<spec id="s1">CHANGED</spec>
</main>`)

		output, code := cli.Run([]string{"diff", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "Spec:") {
			t.Fatalf("expected Spec: header:\n%s", output)
		}
		if !strings.Contains(output, "Marker:") {
			t.Fatalf("expected Marker: header:\n%s", output)
		}
	})
}

func TestCLIDiffNoBaseline(t *testing.T) {
	t.Run("diff_no_baseline_snapshot", func(t *testing.T) {
		dir := t.TempDir()

		writeMainPin(t, dir, `<main>
<spec id="s1">Spec content</spec>
</main>`)
		testutil.WriteCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {}
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"init"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)

		// Delete baseline files to simulate pre-migration.
		baselinesDir := filepath.Join(dir, ".driftpin", "baselines")
		entries, _ := os.ReadDir(baselinesDir)
		for _, e := range entries {
			os.Remove(filepath.Join(baselinesDir, e.Name()))
		}

		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "no baseline snapshot") {
			t.Fatalf("expected 'no baseline snapshot' in output:\n%s", output)
		}
	})
}

func TestCLIDiffSpecDeleted(t *testing.T) {
	t.Run("diff_deleted_spec_shows_all_removed", func(t *testing.T) {
		dir := setupDiffProject(t)

		writeMainPin(t, dir, `<main></main>`)

		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "deleted") {
			t.Fatalf("expected 'deleted' status:\n%s", output)
		}
	})
}

func TestCLIDiffHelp(t *testing.T) {
	t.Run("diff_help", func(t *testing.T) {
		dir := t.TempDir()
		output, code := cli.Run([]string{"diff", "--help"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(output, "Usage:") {
			t.Fatalf("expected usage in help:\n%s", output)
		}
	})
}

func TestCLIDiffMissingArgs(t *testing.T) {
	t.Run("diff_no_args", func(t *testing.T) {
		dir := t.TempDir()
		output, code := cli.Run([]string{"diff"}, dir)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(output, "usage:") {
			t.Fatalf("expected usage message:\n%s", output)
		}
	})
}

func TestCLIDiffFormat(t *testing.T) {
	t.Run("diff_has_separator_between_sides", func(t *testing.T) {
		dir := setupDiffProject(t)

		writeMainPin(t, dir, `<main>
<spec id="s1">CHANGED</spec>
</main>`)
		testutil.WriteCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {
	// CHANGED
}
`+testutil.MarkerEnd("m1")+`
`)

		output, code := cli.Run([]string{"diff", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "---\n") {
			t.Fatalf("expected --- separator between spec and marker:\n%s", output)
		}
		if !strings.Contains(output, "--- baseline") {
			t.Fatalf("expected --- baseline header:\n%s", output)
		}
		if !strings.Contains(output, "+++ current") {
			t.Fatalf("expected +++ current header:\n%s", output)
		}
	})
}
