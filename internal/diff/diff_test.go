package diff_test

import (
	"strings"
	"testing"

	"driftpin/internal/diff"
)

func TestUnifiedDiffIdentity(t *testing.T) {
	got := diff.UnifiedDiff("hello\nworld\n", "hello\nworld\n")
	if got != "" {
		t.Fatalf("expected empty diff for identical input, got:\n%s", got)
	}
}

func TestUnifiedDiffEmpty(t *testing.T) {
	got := diff.UnifiedDiff("", "")
	if got != "" {
		t.Fatalf("expected empty diff for both empty, got:\n%s", got)
	}
}

func TestUnifiedDiffInsert(t *testing.T) {
	old := "a\nb\n"
	new := "a\nx\nb\n"
	got := diff.UnifiedDiff(old, new)
	if !strings.Contains(got, "\n+x") {
		t.Fatalf("expected +x line in diff:\n%s", got)
	}
	if hasRemovedLine(got) {
		t.Fatalf("expected no removed lines in insert-only diff:\n%s", got)
	}
}

func TestUnifiedDiffDelete(t *testing.T) {
	old := "a\nx\nb\n"
	new := "a\nb\n"
	got := diff.UnifiedDiff(old, new)
	if !strings.Contains(got, "\n-x") {
		t.Fatalf("expected -x line in diff:\n%s", got)
	}
	if hasAddedLine(got) {
		t.Fatalf("expected no added lines in delete-only diff:\n%s", got)
	}
}

func TestUnifiedDiffModify(t *testing.T) {
	old := "a\nx\nb\n"
	new := "a\ny\nb\n"
	got := diff.UnifiedDiff(old, new)
	if !strings.Contains(got, "\n-x") || !strings.Contains(got, "\n+y") {
		t.Fatalf("expected -x and +y in diff:\n%s", got)
	}
}

func TestUnifiedDiffReplaceBlock(t *testing.T) {
	old := "a\nx\ny\nb\n"
	new := "a\np\nq\nb\n"
	got := diff.UnifiedDiff(old, new)
	if !strings.Contains(got, "\n-x") || !strings.Contains(got, "\n-y") {
		t.Fatalf("expected -x -y in diff:\n%s", got)
	}
	if !strings.Contains(got, "\n+p") || !strings.Contains(got, "\n+q") {
		t.Fatalf("expected +p +q in diff:\n%s", got)
	}
}

func TestUnifiedDiffMultiHunk(t *testing.T) {
	old := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\n"
	new := "l1\nl2\nl3\nl4\nMOD\nl6\nl7\nl8\nl9\nl10\nl11\nl12\n"
	got := diff.UnifiedDiff(old, new)
	if !strings.Contains(got, "MOD") {
		t.Fatalf("expected MOD in diff:\n%s", got)
	}
	// single change in middle, small file → should still produce one hunk
	hunks := strings.Count(got, "@@ -")
	if hunks != 1 {
		t.Fatalf("expected one hunk, got %d:\n%s", hunks, got)
	}
}

func TestUnifiedDiffMultiHunkDisjoint(t *testing.T) {
	old := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n"
	new := "CHANGED1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nCHANGED2\n"
	got := diff.UnifiedDiff(old, new)
	hunks := strings.Count(got, "@@ -")
	if hunks < 2 {
		t.Fatalf("expected at least 2 hunks for disjoint changes, got %d:\n%s", hunks, got)
	}
}

func TestUnifiedDiffSingleLine(t *testing.T) {
	got := diff.UnifiedDiff("a\n", "b\n")
	if !strings.Contains(got, "\n-a") || !strings.Contains(got, "\n+b") {
		t.Fatalf("expected -a +b in single-line diff:\n%s", got)
	}
}

func TestUnifiedDiffDeletionDrift(t *testing.T) {
	// Marker deleted → new content is empty.
	got := diff.UnifiedDiff("line1\nline2\nline3\n", "")
	if !strings.Contains(got, "\n-line1") || !strings.Contains(got, "\n-line2") {
		t.Fatalf("expected all-removed lines in deletion drift:\n%s", got)
	}
	if hasAddedLine(got) {
		t.Fatalf("expected no added lines in deletion drift:\n%s", got)
	}
}

func TestUnifiedDiffNewOnly(t *testing.T) {
	// Old empty, new has content (e.g. marker just added).
	got := diff.UnifiedDiff("", "line1\nline2\n")
	if !strings.Contains(got, "\n+line1") || !strings.Contains(got, "\n+line2") {
		t.Fatalf("expected all-added lines:\n%s", got)
	}
	if hasRemovedLine(got) {
		t.Fatalf("expected no removed lines:\n%s", got)
	}
}

func TestUnifiedDiffHunkHeader(t *testing.T) {
	got := diff.UnifiedDiff("a\nb\nc\nd\n", "a\nb\nNEW\nd\n")
	if !strings.HasPrefix(got, "@@ ") {
		t.Fatalf("expected diff to start with hunk header:\n%s", got)
	}
	if !strings.Contains(got, "@@ -1,4 +1,4 @@") && !strings.Contains(got, "@@ -2") {
		t.Fatalf("expected hunk header with line counts:\n%s", got)
	}
}

func hasRemovedLine(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "-") {
			return true
		}
	}
	return false
}

func hasAddedLine(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "+") {
			return true
		}
	}
	return false
}
