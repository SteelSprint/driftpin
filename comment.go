package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ScanMarkers scans a file for #F markers and returns all matches.
func ScanMarkers(path string) ([]MarkerMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []MarkerMatch
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		m := ParseMarker(raw)
		if m != nil {
			m.Line = lineNum
			m.Col = 1
			matches = append(matches, *m)
		}
	}
	return matches, scanner.Err()
}

// MalformedMatch represents a line that looks like a marker attempt
// (contains #F followed by whitespace and id:) but doesn't conform
// to the well_formed_marker syntax.
type MalformedMatch struct {
	Raw  string
	Line int
}

// ScanMalformed scans a file for lines that look like marker attempts
// but don't conform to the marker syntax.
func ScanMalformed(path string) ([]MalformedMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []MalformedMatch
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		if attemptPattern.MatchString(raw) && !markerPattern.MatchString(raw) {
			matches = append(matches, MalformedMatch{Raw: raw, Line: lineNum})
		}
	}
	return matches, scanner.Err()
}

// WalkPaths walks the given paths and returns all text files.
func WalkPaths(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if isTextFile(path) {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func isTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return false
		}
	}
	return true
}

// Finding describes one issue found by Check.
type Finding struct {
	Status   string // SPEC_DRIFT, SITE_DRIFT, MISSING, ORPHAN, MALFORMED, NOT_IN_STATE, STATE_FILE_MISSING, DUPLICATE_MARKER
	MarkerID string
	ClauseID string
	File     string
	Line     int
	Reason   string // extra context for DUPLICATE_MARKER
}

// Check runs the full drift detection against the spec, lock file, and workspace.
func Check(spec *Spec, lock *LockFile, paths []string, windowSize int) ([]Finding, error) {
	files, err := WalkPaths(paths)
	if err != nil {
		return nil, err
	}

	specHashes := ComputeAllHashes(spec)
	defined := spec.DefinedIDs()
	referenced := make(map[string]bool) // clause_id -> has at least one marker
	seen := make(map[string]string)     // marker_id -> first file where seen
	var findings []Finding

	// If lock file is missing, report it and return.
	if lock == nil {
		// #F id:wqg2e13l drift.state_file_missing
		return []Finding{{Status: "STATE_FILE_MISSING"}}, nil
	}

	for _, f := range files {
		markers, err := ScanMarkers(f)
		if err != nil {
			return nil, err
		}

		// Detect malformed markers: lines that look like marker attempts
		// (contain #F + whitespace + id:) but don't conform to the full syntax.
		// #F id:i8mfd9mm drift.malformed
		malformed, err := ScanMalformed(f)
		if err != nil {
			return nil, err
		}
		for _, mm := range malformed {
			findings = append(findings, Finding{
				Status: "MALFORMED",
				File:   f,
				Line:   mm.Line,
				Reason: mm.Raw,
			})
		}

		for _, m := range markers {
			// Check for duplicate marker ids across files
			if firstFile, exists := seen[m.MarkerID]; exists {
				if firstFile != f {
					// #F id:qf8r2m6d drift.duplicate_marker
					findings = append(findings, Finding{
						Status:   "DUPLICATE_MARKER",
						MarkerID: m.MarkerID,
						File:     f,
						Line:     m.Line,
						Reason:   fmt.Sprintf("marker %s already used in %s", m.MarkerID, firstFile),
					})
					continue
				}
			} else {
				seen[m.MarkerID] = f
			}

			// Validate clause_ids exist in spec
			validClauses := true
			for _, cid := range m.ClauseIDs {
				if !defined[cid] {
					// #F id:0snjuwm2 drift.orphan
					findings = append(findings, Finding{
						Status:   "ORPHAN",
						MarkerID: m.MarkerID,
						ClauseID: cid,
						File:     f,
						Line:     m.Line,
					})
					validClauses = false
				}
			}
			if !validClauses {
				continue
			}

			// Mark clauses as referenced
			for _, cid := range m.ClauseIDs {
				referenced[cid] = true
			}

			// Check if marker is in state file
			_, inSite := lock.Site[m.MarkerID]
			if !inSite {
				// #F id:rf5b9vfe drift.not_in_state
				findings = append(findings, Finding{
					Status:   "NOT_IN_STATE",
					MarkerID: m.MarkerID,
					File:     f,
					Line:     m.Line,
				})
				continue
			}

			// Check spec drift per clause
			// #F id:zq057zar drift.spec_drift
			for _, cid := range m.ClauseIDs {
				stateKey := m.MarkerID + ":" + cid
				reviewedHash, inState := lock.State[stateKey]
				if !inState {
					findings = append(findings, Finding{
						Status:   "NOT_IN_STATE",
						MarkerID: m.MarkerID,
						ClauseID: cid,
						File:     f,
						Line:     m.Line,
					})
					continue
				}
				currentHash := specHashes[cid]
				if reviewedHash != currentHash {
					findings = append(findings, Finding{
						Status:   "SPEC_DRIFT",
						MarkerID: m.MarkerID,
						ClauseID: cid,
						File:     f,
						Line:     m.Line,
					})
				}
			}

			// Check site drift
			// #F id:wl417xku drift.site_drift
			contentHash, err := ComputeContentHash(f, m.Line, windowSize)
			if err != nil {
				return nil, fmt.Errorf("computing content hash for %s:%d: %w", f, m.Line, err)
			}
			storedHash := lock.Site[m.MarkerID]
			if contentHash != storedHash {
				// #F id:gbql4h6j drift.both_drift
				findings = append(findings, Finding{
					Status:   "SITE_DRIFT",
					MarkerID: m.MarkerID,
					File:     f,
					Line:     m.Line,
				})
			}
		}
	}

	// Detect missing clauses: in spec but not covered (directly or transitively).
	// #F id:ttsbgeqq drift.missing
	// #F id:x2k9m3vf drift.transitive_coverage

	// Build reference graph: for each element, what does it reference?
	refsGraph := make(map[string][]string)
	for _, e := range spec.All() {
		if e.Kind == KindSection {
			continue
		}
		refsGraph[e.ID] = ReferencesInOrder(e, defined)
	}

	// Compute transitively covered elements via BFS from markered clauses.
	covered := make(map[string]bool)
	for cid := range referenced {
		covered[cid] = true
	}
	var queue []string
	for cid := range covered {
		queue = append(queue, cid)
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, r := range refsGraph[curr] {
			if !covered[r] {
				covered[r] = true
				queue = append(queue, r)
			}
		}
	}

	for _, e := range spec.All() {
		if e.Kind != KindClause {
			continue
		}
		if defined[e.ID] && !covered[e.ID] {
			findings = append(findings, Finding{
				Status:   "MISSING",
				ClauseID: e.ID,
			})
		}
	}

	return findings, nil
}

// #F id:aq2yi92y output.prose
// #F id:b2c6rx5p output.finding_prose
// FormatFinding formats a Finding as prose output.
func FormatFinding(f Finding) string {
	switch f.Status {
	case "SPEC_DRIFT":
		return fmt.Sprintf(
			"SPEC_DRIFT  marker=%s  clause=%s  %s:%d\n"+
				"  The spec clause %q changed since this marker was last reviewed.\n"+
				"  The text at %s:%d traces to this clause — verify it still matches\n"+
				"  the spec's current wording, then run 'filament resolve --spec %s'.",
			f.MarkerID, f.ClauseID, f.File, f.Line,
			f.ClauseID, f.File, f.Line, f.MarkerID)
	case "SITE_DRIFT":
		return fmt.Sprintf(
			"SITE_DRIFT  marker=%s  %s:%d\n"+
				"  The content near this marker changed since the spec was last reviewed.\n"+
				"  Read the spec clauses this marker traces to, compare against the text\n"+
				"  at %s:%d, then run 'filament resolve --site %s'.",
			f.MarkerID, f.File, f.Line,
			f.File, f.Line, f.MarkerID)
	case "MISSING":
		return fmt.Sprintf(
			"MISSING  clause=%s\n"+
				"  This clause is in the spec but has no #F marker in any scanned file.\n"+
				"  Add one with 'filament add %s'.",
			f.ClauseID, f.ClauseID)
	case "ORPHAN":
		return fmt.Sprintf(
			"ORPHAN  marker=%s  clause=%s  %s:%d\n"+
				"  This marker references clause %s, which is not in the spec.\n"+
				"  The spec may have removed or renamed it — remove the marker or\n"+
				"  correct the clause id.",
			f.MarkerID, f.ClauseID, f.File, f.Line,
			f.ClauseID)
	case "MALFORMED":
		return fmt.Sprintf(
			"MALFORMED  %s:%d\n"+
				"  This line looks like a marker but has invalid syntax: %q\n"+
				"  Expected: #F id:<8-char-id> <clause_id> [clause_id]...\n"+
				"  Fix the line or remove it if it was not meant to be a marker.",
			f.File, f.Line, f.Reason)
	case "DUPLICATE_MARKER":
		return fmt.Sprintf(
			"DUPLICATE_MARKER  marker=%s  %s:%d\n"+
				"  This marker id is already used in another file.\n"+
				"  %s\n"+
				"  Each marker id must be unique across the workspace. Generate a new\n"+
				"  marker with 'filament add' and replace this one.",
			f.MarkerID, f.File, f.Line, f.Reason)
	case "NOT_IN_STATE":
		return fmt.Sprintf(
			"NOT_IN_STATE  marker=%s  %s:%d\n"+
				"  This marker is not in the state file. Run 'filament init' or\n"+
				"  'filament resolve --site %s' to register it.",
			f.MarkerID, f.File, f.Line, f.MarkerID)
	case "STATE_FILE_MISSING":
		return "STATE_FILE_MISSING\n" +
			"  No .filament file found. Run 'filament init' to create one."
	default:
		return fmt.Sprintf("UNKNOWN  status=%s", f.Status)
	}
}

// FormatStatusResult formats a marker's status for the status subcommand.
func FormatStatusResult(markerID string, clauseIDs []string, file string, line int, status string) string {
	clauses := strings.Join(clauseIDs, ", ")
	switch status {
	case "OK":
		return fmt.Sprintf("  OK          %-8s  %s:%d  clauses: %s", markerID, file, line, clauses)
	case "SPEC_DRIFT":
		return fmt.Sprintf("  SPEC_DRIFT  %-8s  %s:%d  clauses: %s", markerID, file, line, clauses)
	case "SITE_DRIFT":
		return fmt.Sprintf("  SITE_DRIFT  %-8s  %s:%d  clauses: %s", markerID, file, line, clauses)
	case "NOT_IN_STATE":
		return fmt.Sprintf("  NOT_IN_STATE  %-8s  %s:%d  clauses: %s", markerID, file, line, clauses)
	default:
		return fmt.Sprintf("  %-12s  %-8s  %s:%d  clauses: %s", status, markerID, file, line, clauses)
	}
}

// #F id:gegp90b7 output.result_prose
// FormatMarkerLine generates a #F marker line for the given marker_id and clause_ids.
func FormatMarkerLine(markerID string, clauseIDs []string) string {
	return fmt.Sprintf("#F id:%s %s", markerID, strings.Join(clauseIDs, " "))
}

// #F id:a47bacif output.tooltip
// #F id:zmtuvlt0 output.neutral_language
// #F id:m4rkr002 best_practices.multiple_markers
// Tooltip is the preamble printed at the top of every command's output.
const Tooltip = `filament tracks whether a workspace's files stay aligned with their spec.
(Suppress this preamble with --quiet.) Specs are the source of truth; #F
markers in workspace files trace to spec clauses. A clause may have
multiple markers — one at every site where it's implemented. Drift
means a clause and the content referencing it may have diverged — each
finding requires review, not just a command. Read the full guide with
'filament skill'.
`
