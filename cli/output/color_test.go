package output

import (
	"regexp"
	"testing"

	"drift/core"
	"drift/orchestrator"
)

// stripANSI removes all ANSI escape sequences from s.
func stripANSI(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "")
}

// TestGuardrailProperty asserts the core correctness invariant:
// stripANSI(ColorPresenter.X(r)) == PlainPresenter.X(r) for every Result r
// and every Presenter method X. If Color and Plain ever diverge in layout
// (not just ANSI codes), this test fails. See output.guardrail_property spec.
func TestGuardrailProperty(t *testing.T) {
	plain := PlainPresenter{}

	// The guardrail is parameterized across all 12 built-in themes: every
	// theme's output, when ANSI-stripped, must equal Plain byte-for-byte.
	for themeName, theme := range AllThemes {
		color := ColorPresenter{Theme: theme}

	// Build representative Result values covering the key paths.
	results := []struct {
		name string
		render func(Presenter) string
	}{
		// --- TodoResult: empty ---
		{"todo_empty", func(p Presenter) string {
			return p.Todo(TodoResult{State: core.EvaluatedState{}})
		}},
		// --- TodoResult: synced ---
		{"todo_synced", func(p Presenter) string {
			return p.Todo(TodoResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml"}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 1, EndLineNumber: 3}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
			}})
		}},
		// --- TodoResult: drifted ---
		{"todo_drifted", func(p Presenter) string {
			return p.Todo(TodoResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml", LineNumber: 5}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 10, EndLineNumber: 20}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
				Todos:   []core.Todo{{SpecID: "main.s1", MarkerID: "m1", MarkerChanged: true, SpecFilepath: "main.drift.xml", SpecLineNumber: 5, MarkerFilepath: "code.go", MarkerLineNumber: 10}},
			}})
		}},
		// --- TodoResult: deleted spec ---
		{"todo_spec_deleted", func(p Presenter) string {
			return p.Todo(TodoResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml", LineNumber: 5}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 10, EndLineNumber: 20}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
				Todos:   []core.Todo{{SpecID: "main.s1", MarkerID: "m1", SpecDeleted: true, SpecFilepath: "main.drift.xml", SpecLineNumber: 5, MarkerFilepath: "code.go", MarkerLineNumber: 10}},
			}})
		}},
		// --- TodoResult: unlinked markers ---
		{"todo_unlinked", func(p Presenter) string {
			return p.Todo(TodoResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml"}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 1, EndLineNumber: 3}, {ID: "orphan", Hash: "xyz", Filepath: "other.go", LineNumber: 1, EndLineNumber: 3}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
			}})
		}},
		// --- ListResult: basic ---
		{"list_basic", func(p Presenter) string {
			return p.List(ListResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml"}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 1, EndLineNumber: 3}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
			}})
		}},
		// --- ListResult: empty ---
		{"list_empty", func(p Presenter) string {
			return p.List(ListResult{State: core.EvaluatedState{}})
		}},
		// --- ListResult: drifted link ---
		{"list_drifted", func(p Presenter) string {
			return p.List(ListResult{State: core.EvaluatedState{
				Specs:   []core.Spec{{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml", LineNumber: 5}},
				Markers: []core.Marker{{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 1, EndLineNumber: 3}},
				Links:   []core.Link{{SpecID: "main.s1", MarkerID: "m1"}},
				Todos:   []core.Todo{{SpecID: "main.s1", MarkerID: "m1", MarkerChanged: true, SpecFilepath: "main.drift.xml", SpecLineNumber: 5, MarkerFilepath: "code.go", MarkerLineNumber: 1}},
			}})
		}},
		// --- ShowResult: spec not found ---
		{"show_spec_not_found", func(p Presenter) string {
			return p.Show(ShowResult{IsSpec: true, ID: "main.missing"})
		}},
		// --- ShowResult: marker not found ---
		{"show_marker_not_found", func(p Presenter) string {
			return p.Show(ShowResult{IsSpec: false, ID: "missing"})
		}},
		// --- ShowResult: spec found ---
		{"show_spec_found", func(p Presenter) string {
			spec := &core.Spec{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml"}
			return p.Show(ShowResult{IsSpec: true, ID: "main.s1", Spec: spec, Content: "spec text"})
		}},
		// --- ShowResult: marker found with linked specs ---
		{"show_marker_found", func(p Presenter) string {
			marker := &core.Marker{ID: "m1", Hash: "def", Filepath: "code.go", LineNumber: 1, EndLineNumber: 3}
			return p.Show(ShowResult{
				IsSpec: false, ID: "m1", Marker: marker, Content: "code here",
				LinkedSpecs: []LinkedSpec{{Spec: core.Spec{ID: "main.s1", Hash: "abc", Filepath: "main.drift.xml"}, Content: "spec text"}},
			})
		}},
		// --- DiffEdgeResult: in sync ---
		{"diff_edge_synced", func(p Presenter) string {
			return p.DiffEdge(DiffEdgeResult{Result: orchestrator.DiffResult{
				Spec:   orchestrator.DiffSide{ID: "main.s1", Filepath: "main.drift.xml", BaselineHash: "abc", CurrentHash: "abc", HasBaseline: true, Baseline: "same", Current: "same"},
				Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "def", CurrentHash: "def", HasBaseline: true, Baseline: "same", Current: "same"},
			}})
		}},
		// --- DiffEdgeResult: drifted with patch ---
		{"diff_edge_drifted", func(p Presenter) string {
			return p.DiffEdge(DiffEdgeResult{Result: orchestrator.DiffResult{
				Spec:   orchestrator.DiffSide{ID: "main.s1", Filepath: "main.drift.xml", BaselineHash: "abc", CurrentHash: "xyz", HasBaseline: true, Baseline: "line1\nline2\n", Current: "line1\nline2\nline3\n"},
				Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "def", CurrentHash: "ghi", HasBaseline: true, Baseline: "old\n", Current: "new\n"},
			}})
		}},
		// --- DiffEdgeResult: deleted ---
		{"diff_edge_deleted", func(p Presenter) string {
			return p.DiffEdge(DiffEdgeResult{Result: orchestrator.DiffResult{
				Spec:   orchestrator.DiffSide{ID: "main.s1", Filepath: "main.drift.xml", BaselineHash: "abc", CurrentHash: "", HasBaseline: true, Baseline: "text", Current: "", Deleted: true},
				Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "def", CurrentHash: "def", HasBaseline: true, Baseline: "same", Current: "same"},
			}})
		}},
		// --- DiffEdgeResult: no baseline ---
		{"diff_edge_no_baseline", func(p Presenter) string {
			return p.DiffEdge(DiffEdgeResult{Result: orchestrator.DiffResult{
				Spec:   orchestrator.DiffSide{ID: "main.s1", Filepath: "main.drift.xml", BaselineHash: "abc", CurrentHash: "abc", HasBaseline: false},
				Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "def", CurrentHash: "def", HasBaseline: true, Baseline: "same", Current: "same"},
			}})
		}},
		// --- DiffExpandedResult ---
		{"diff_expanded", func(p Presenter) string {
			return p.DiffExpanded(DiffExpandedResult{
				ID: "m1",
				Edges: []orchestrator.DiffResult{
					{Spec: orchestrator.DiffSide{ID: "main.s1", Filepath: "f.xml", BaselineHash: "a", CurrentHash: "a", HasBaseline: true, Baseline: "x", Current: "x"},
						Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "b", CurrentHash: "b", HasBaseline: true, Baseline: "y", Current: "y"}},
				},
			})
		}},
		// --- DiffAllResult: no drift ---
		{"diff_all_empty", func(p Presenter) string {
			return p.DiffAll(DiffAllResult{Edges: nil})
		}},
		// --- DiffAllResult: with edges ---
		{"diff_all_edges", func(p Presenter) string {
			return p.DiffAll(DiffAllResult{
				Edges: []orchestrator.DiffResult{
					{Spec: orchestrator.DiffSide{ID: "main.s1", Filepath: "f.xml", BaselineHash: "a", CurrentHash: "b", HasBaseline: true, Baseline: "old\n", Current: "new\n"},
						Marker: orchestrator.DiffSide{ID: "m1", Filepath: "code.go", Lines: "1-3", BaselineHash: "c", CurrentHash: "c", HasBaseline: true, Baseline: "same", Current: "same"}},
				},
			})
		}},
		// --- OkResult ---
		{"ok", func(p Presenter) string {
			return p.Ok(OkResult{Command: "link", Message: `Linked marker "m1" to spec "main.s1"`})
		}},
		// --- ErrorResult: no hint ---
		{"error_no_hint", func(p Presenter) string {
			return p.Error(ErrorResult{Command: "init", Message: "some error", Exit: 1})
		}},
		// --- ErrorResult: with hint ---
		{"error_with_hint", func(p Presenter) string {
			return p.Error(ErrorResult{Command: "diff", Message: "unknown command", Hint: "run drift help", Exit: 1})
		}},
		// --- TextResult ---
		{"text", func(p Presenter) string {
			return p.Text(TextResult{Text: "some help text"})
		}},
		// --- VersionResult ---
		{"version", func(p Presenter) string {
			return p.Version(VersionResult{Version: "1.0.0"})
		}},
	}

	for _, tc := range results {
		t.Run(themeName+"/"+tc.name, func(t *testing.T) {
			plainOut := tc.render(plain)
			colorOut := tc.render(color)
			colorStripped := stripANSI(colorOut)
			if plainOut != colorStripped {
				t.Errorf("guardrail violated for theme=%s case=%s\nplain:    %q\ncolor raw: %q\ncolor str: %q",
					themeName, tc.name, plainOut, colorOut, colorStripped)
			}
		})
	}
	}
}
