package orchestrator_test

import (
	"path/filepath"
	"testing"

	"driftpin/core"
	"driftpin/internal/testutil"
	"driftpin/orchestrator"
	"driftpin/pinstore"
	"driftpin/scanner"
)

func writeSpecFile(t *testing.T, dir, name, content string) {
	t.Helper()
	testutil.WriteSpecFile(t, dir, name, content)
}

func writeCodeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	testutil.WriteCodeFile(t, dir, name, content)
}

// setupProject creates a temp dir with one spec (main.s1) and one marker (m1),
// runs init + link, and returns the orchestrator wired with real file-backed
// pin store, scanner, and baseline store.
func setupProject(t *testing.T) (dir string, orch *orchestrator.Orchestrator) {
	t.Helper()
	dir = t.TempDir()

	writeSpecFile(t, dir, "main.pin.xml", `<main>
<spec id="s1">Spec content line 1</spec>
</main>`)

	writeCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {
	// body
}
`+testutil.MarkerEnd("m1")+`
`)

	pin := pinstore.NewFilePinStore(dir)
	sc := scanner.NewFileScanner(dir)
	baselines := pinstore.NewBaselineStore(filepath.Join(dir, ".driftpin", "baselines"))
	orch = orchestrator.NewOrchestrator(pin, sc, baselines)

	if err := orch.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := orch.Link("m1", "main.s1"); err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	return dir, orch
}

func TestOrchestratorLinkWritesBaselines(t *testing.T) {
	t.Run("link_writes_spec_and_marker_baselines", func(t *testing.T) {
		dir, orch := setupProject(t)

		todo, err := orch.Todo()
		if err != nil {
			t.Fatal(err)
		}
		if len(todo.Todos) != 0 {
			t.Fatalf("expected no drift after link, got %d todos", len(todo.Todos))
		}

		// Baseline files should exist for both spec and marker.
		spec := testutil.FindSpecInEvaluatedState(t, todo, "main.s1")
		marker := testutil.FindMarkerInEvaluatedState(t, todo, "m1")

		baselines := pinstore.NewBaselineStore(filepath.Join(dir, ".driftpin", "baselines"))
		if _, ok := baselines.Read(spec.Hash); !ok {
			t.Fatalf("spec baseline file missing for hash %s", spec.Hash)
		}
		if _, ok := baselines.Read(marker.Hash); !ok {
			t.Fatalf("marker baseline file missing for hash %s", marker.Hash)
		}
	})
}

func TestOrchestratorDiffInSync(t *testing.T) {
	t.Run("diff_returns_baseline_and_current_matching", func(t *testing.T) {
		_, orch := setupProject(t)

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if !result.Spec.HasBaseline {
			t.Fatal("spec HasBaseline should be true after link")
		}
		if !result.Marker.HasBaseline {
			t.Fatal("marker HasBaseline should be true after link")
		}
		if result.Spec.Baseline != result.Spec.Current {
			t.Fatalf("spec baseline != current:\nbaseline: %q\ncurrent:  %q", result.Spec.Baseline, result.Spec.Current)
		}
		if result.Marker.Baseline != result.Marker.Current {
			t.Fatalf("marker baseline != current:\nbaseline: %q\ncurrent:  %q", result.Marker.Baseline, result.Marker.Current)
		}
		if result.Spec.Deleted || result.Marker.Deleted {
			t.Fatal("entities should not be deleted")
		}
	})
}

func TestOrchestratorDiffSpecDrifted(t *testing.T) {
	t.Run("diff_shows_changed_spec_content", func(t *testing.T) {
		dir, orch := setupProject(t)

		// Modify the spec content.
		writeSpecFile(t, dir, "main.pin.xml", `<main>
<spec id="s1">CHANGED spec content</spec>
</main>`)

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if !result.Spec.HasBaseline {
			t.Fatal("spec baseline should still exist from original link")
		}
		if result.Spec.Baseline == result.Spec.Current {
			t.Fatal("spec baseline should differ from current after drift")
		}
		if result.Spec.Current != "CHANGED spec content" {
			t.Fatalf("spec current = %q, want %q", result.Spec.Current, "CHANGED spec content")
		}
		if result.Spec.Baseline != "Spec content line 1" {
			t.Fatalf("spec baseline = %q, want %q", result.Spec.Baseline, "Spec content line 1")
		}
		// Marker should still be in sync (baseline == current).
		if result.Marker.Baseline != result.Marker.Current {
			t.Fatal("marker should be unchanged (baseline == current)")
		}
	})
}

func TestOrchestratorDiffMarkerDrifted(t *testing.T) {
	t.Run("diff_shows_changed_marker_content", func(t *testing.T) {
		dir, orch := setupProject(t)

		// Modify the marker body.
		writeCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {
	// CHANGED body
}
`+testutil.MarkerEnd("m1")+`
`)

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if !result.Marker.HasBaseline {
			t.Fatal("marker baseline should still exist")
		}
		if result.Marker.Baseline == result.Marker.Current {
			t.Fatal("marker baseline should differ from current after drift")
		}
		if result.Spec.Baseline != result.Spec.Current {
			t.Fatal("spec should be unchanged")
		}
	})
}

func TestOrchestratorDiffNoBaseline(t *testing.T) {
	t.Run("diff_with_missing_baseline_snapshot", func(t *testing.T) {
		dir := t.TempDir()

		writeSpecFile(t, dir, "main.pin.xml", `<main>
<spec id="s1">Spec content</spec>
</main>`)
		writeCodeFile(t, dir, "code.go", `package main

`+testutil.MarkerStart("m1")+`
func f() {}
`+testutil.MarkerEnd("m1")+`
`)

		// Use a baseline store in a dir that won't have files (simulate pre-migration).
		pin := pinstore.NewFilePinStore(dir)
		sc := scanner.NewFileScanner(dir)
		baselines := pinstore.NewBaselineStore(filepath.Join(dir, ".driftpin", "baselines"))
		orch := orchestrator.NewOrchestrator(pin, sc, baselines)

		if err := orch.Init(); err != nil {
			t.Fatal(err)
		}
		// Manually save a state with a link but DON'T write baseline files
		// (simulate state.xml that predates the baselines/ directory).
		pin.Save(pinstore.PinState{
			Specs:   []core.Spec{{ID: "main.s1", Hash: "fakehash", Filepath: filepath.Join(dir, "main.pin.xml")}},
			Markers: []core.Marker{{ID: "m1", Hash: "fakehash2", Filepath: filepath.Join(dir, "code.go"), LineNumber: 3, EndLineNumber: 5}},
			Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
		})

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if result.Spec.HasBaseline {
			t.Fatal("spec HasBaseline should be false when no snapshot exists")
		}
		if result.Marker.HasBaseline {
			t.Fatal("marker HasBaseline should be false when no snapshot exists")
		}
		if result.Spec.Current == "" {
			t.Fatal("spec Current should still be populated from disk")
		}
		if result.Marker.Current == "" {
			t.Fatal("marker Current should still be populated from disk")
		}
	})
}

func TestOrchestratorDiffSpecDeleted(t *testing.T) {
	t.Run("diff_with_deleted_spec", func(t *testing.T) {
		dir, orch := setupProject(t)

		// Remove the spec entirely.
		writeSpecFile(t, dir, "main.pin.xml", `<main></main>`)

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if !result.Spec.Deleted {
			t.Fatal("spec should be marked deleted")
		}
		if result.Spec.Current != "" {
			t.Fatalf("spec Current should be empty when deleted, got %q", result.Spec.Current)
		}
		if !result.Spec.HasBaseline {
			t.Fatal("spec baseline should still exist from before deletion")
		}
		if result.Spec.Baseline == "" {
			t.Fatal("spec baseline content should be non-empty")
		}
	})
}

func TestOrchestratorDiffMarkerDeleted(t *testing.T) {
	t.Run("diff_with_deleted_marker", func(t *testing.T) {
		dir, orch := setupProject(t)

		// Remove the marker entirely.
		writeCodeFile(t, dir, "code.go", `package main
`)

		result, err := orch.Diff("m1", "main.s1")
		if err != nil {
			t.Fatalf("Diff failed: %v", err)
		}

		if !result.Marker.Deleted {
			t.Fatal("marker should be marked deleted")
		}
		if result.Marker.Current != "" {
			t.Fatalf("marker Current should be empty when deleted, got %q", result.Marker.Current)
		}
		if !result.Marker.HasBaseline {
			t.Fatal("marker baseline should still exist from before deletion")
		}
	})
}

func TestOrchestratorResetWritesNewBaselines(t *testing.T) {
	t.Run("reset_collapse_writes_new_baseline_files", func(t *testing.T) {
		dir, orch := setupProject(t)

		// Drift the spec.
		writeSpecFile(t, dir, "main.pin.xml", `<main>
<spec id="s1">NEW spec content</spec>
</main>`)

		todoBefore, err := orch.Todo()
		if err != nil {
			t.Fatal(err)
		}
		if len(todoBefore.Todos) != 1 {
			t.Fatalf("expected 1 todo after drift, got %d", len(todoBefore.Todos))
		}

		// Reset to accept the new baseline.
		if _, err := orch.Reset("m1", "main.s1"); err != nil {
			t.Fatalf("Reset failed: %v", err)
		}

		todoAfter, err := orch.Todo()
		if err != nil {
			t.Fatal(err)
		}
		if len(todoAfter.Todos) != 0 {
			t.Fatalf("expected 0 todos after reset, got %d", len(todoAfter.Todos))
		}

		// New baseline file should exist at the new hash.
		spec := testutil.FindSpecInEvaluatedState(t, todoAfter, "main.s1")
		baselines := pinstore.NewBaselineStore(filepath.Join(dir, ".driftpin", "baselines"))
		content, ok := baselines.Read(spec.Hash)
		if !ok {
			t.Fatalf("new spec baseline missing for hash %s", spec.Hash)
		}
		if content != "NEW spec content" {
			t.Fatalf("new baseline content = %q, want %q", content, "NEW spec content")
		}
	})
}

func TestOrchestratorDiffEntityNotFound(t *testing.T) {
	t.Run("diff_unknown_marker", func(t *testing.T) {
		_, orch := setupProject(t)
		_, err := orch.Diff("nonexistent", "main.s1")
		if err == nil {
			t.Fatal("expected error for unknown marker")
		}
	})
	t.Run("diff_unknown_spec", func(t *testing.T) {
		_, orch := setupProject(t)
		_, err := orch.Diff("m1", "main.nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown spec")
		}
	})
}
