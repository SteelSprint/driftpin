package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// #F id:xr2m4kqt tool.location
// #F id:lp8n3vwb tool.language
// #F id:qm5t9xkj tool.design
// #F id:ws7j2yhv tool.binary
// #F id:8tf919gk versioning.source
// #F id:cdi0ftqy versioning.amendments
// #F id:ocwydxem self_hosting.test

func printViolations(violations []Violation, quiet bool) {
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "PARSER_VIOLATION  %s: %s\n", v.Rule, v.Detail)
		if !quiet {
			explanation := ruleExplanation(v.Rule)
			if explanation != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", explanation)
			}
		}
	}
	if quiet && len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Run without --quiet for full parser rule explanations.")
	}
}

// #F id:b3vm90d1 public_api.subcommands
const usage = `filament — spec-driven drift detection

Commands:
  filament check [file-or-dir]...
    Verify that every #F marker is in sync with the spec. Exits 1 if any
    drift, missing, orphan, or malformed marker is found. Use in CI/CD as a
    failure gate. Default is current directory.

  filament status [file-or-dir]...
    Show every marker and its drift state. Always exits 0.

  filament init [file-or-dir]...
    Create .filament from the current spec and source markers.

  filament add <clause_id> [clause_id]...
    Print a #F marker line with a new marker id.

  filament resolve --spec <marker_id> [marker_id]...
    Clear spec drift for the given marker(s).

  filament resolve --site <marker_id> [marker_id]...
    Clear site drift for the given marker(s).

  filament sync
    Refresh the [spec] section from the current spec XML.

  filament migrate [file-or-dir]...
    Convert old filament:hash comments to #F markers.

  filament skill
    Print the full usage guide for LLMs and new users.

Options:
  --spec=<path>    Path to spec XML (default: ./filament.spec.xml)
  --quiet          Suppress tooltip preamble
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	args := os.Args[1:]
	cmd, rest := args[0], args[1:]

	// Parse global flags
	specPath := "./filament.spec.xml"
	quiet := false
	var filtered []string
	for _, a := range rest {
		if strings.HasPrefix(a, "--spec=") {
			specPath = strings.TrimPrefix(a, "--spec=")
		} else if a == "--quiet" {
			quiet = true
		} else {
			filtered = append(filtered, a)
		}
	}

	var err error
	switch cmd {
	case "check":
		err = runCheck(specPath, filtered, quiet)
	case "status":
		err = runStatus(specPath, filtered, quiet)
	case "init":
		err = runInit(specPath, filtered, quiet)
	case "add":
		err = runAdd(specPath, filtered, quiet)
	case "resolve":
		err = runResolve(specPath, filtered, quiet)
	case "sync":
		err = runSync(specPath, quiet)
	case "migrate":
		err = runMigrate(specPath, filtered, quiet)
	case "skill":
		err = runSkill(quiet)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// #F id:yd9c6bpz public_api.check
func runCheck(specPath string, paths []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		os.Exit(1)
	}

	lockPath := LockFilePath(specPath)
	lock, err := ReadLockFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprint(os.Stderr, FormatFinding(Finding{Status: "STATE_FILE_MISSING"}))
			os.Exit(1)
		}
		return err
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	windowSize := defaultContentWindow
	findings, err := Check(spec, lock, paths, windowSize)
	if err != nil {
		return err
	}

	if len(findings) == 0 {
		fmt.Println("All markers in sync. No drift detected.")
		return nil
	}

	hasIssues := false
	for _, f := range findings {
		if f.Status != "" {
			hasIssues = true
			fmt.Fprintln(os.Stderr, FormatFinding(f))
			fmt.Fprintln(os.Stderr)
		}
	}
	if hasIssues {
		os.Exit(1)
	}
	return nil
}

// #F id:n9lv604a public_api.status
func runStatus(specPath string, paths []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	lockPath := LockFilePath(specPath)
	lock, err := ReadLockFile(lockPath)
	lockMissing := false
	if err != nil {
		if os.IsNotExist(err) {
			lockMissing = true
			lock = NewLockFile()
		} else {
			return err
		}
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	files, err := WalkPaths(paths)
	if err != nil {
		return err
	}

	specHashes := ComputeAllHashes(spec)
	windowSize := defaultContentWindow

	if lockMissing {
		fmt.Println("No .filament file found. Run 'filament init' to create one.")
		fmt.Println()
	}

	var markerCount int
	for _, f := range files {
		markers, err := ScanMarkers(f)
		if err != nil {
			return err
		}
		for _, m := range markers {
			markerCount++
			status := "OK"

			// Check spec drift
			for _, cid := range m.ClauseIDs {
				stateKey := m.MarkerID + ":" + cid
				reviewedHash, inState := lock.State[stateKey]
				if inState {
					currentHash := specHashes[cid]
					if reviewedHash != currentHash {
						status = "SPEC_DRIFT"
					}
				} else {
					status = "NOT_IN_STATE"
				}
			}

			// Check site drift
			if status == "OK" {
				contentHash, err := ComputeContentHash(f, m.Line, windowSize)
				if err == nil {
					storedHash, inSite := lock.Site[m.MarkerID]
					if inSite && contentHash != storedHash {
						status = "SITE_DRIFT"
					} else if !inSite {
						status = "NOT_IN_STATE"
					}
				}
			}

			fmt.Println(FormatStatusResult(m.MarkerID, m.ClauseIDs, f, m.Line, status))
		}
	}

	if markerCount == 0 {
		fmt.Println("No markers found in scanned files.")
	}

	return nil
}

// #F id:exag17d2 public_api.init
func runInit(specPath string, paths []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	specHashes := ComputeAllHashes(spec)
	lock := NewLockFile()
	windowSize := defaultContentWindow

	// Populate spec section
	for clauseID, hash := range specHashes {
		lock.Spec[clauseID] = hash
	}

	// Scan workspace for markers
	files, err := WalkPaths(paths)
	if err != nil {
		return err
	}

	markerCount := 0
	for _, f := range files {
		markers, err := ScanMarkers(f)
		if err != nil {
			return err
		}
		for _, m := range markers {
			if !MarkerIDIsValid(m.MarkerID) {
				continue
			}
			markerCount++

			// Compute content hash
			contentHash, err := ComputeContentHash(f, m.Line, windowSize)
			if err != nil {
				return fmt.Errorf("computing content hash for %s:%d: %w", f, m.Line, err)
			}
			lock.Site[m.MarkerID] = contentHash

			// Set reviewed spec hash for each clause
			for _, cid := range m.ClauseIDs {
				stateKey := m.MarkerID + ":" + cid
				lock.State[stateKey] = specHashes[cid]
			}
		}
	}

	lockPath := LockFilePath(specPath)
	if err := WriteLockFile(lockPath, lock); err != nil {
		return err
	}

	fmt.Printf("Created %s with %d spec hashes, %d site entries.\n", lockPath, len(lock.Spec), markerCount)
	fmt.Println()
	fmt.Println("The state file is the source of truth for drift detection. Run")
	fmt.Println("'filament check' to verify everything is in sync.")
	return nil
}

// #F id:s39n8x8x public_api.add
func runAdd(specPath string, clauseIDs []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	if len(clauseIDs) == 0 {
		return fmt.Errorf("add requires at least one clause id")
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	defined := spec.DefinedIDs()
	for _, cid := range clauseIDs {
		if !defined[cid] {
			return fmt.Errorf("clause %q is not defined in the spec", cid)
		}
	}

	// Load existing lock to check for collisions
	lockPath := LockFilePath(specPath)
	lock, _ := ReadLockFile(lockPath)
	existing := make(map[string]bool)
	if lock != nil {
		for k := range lock.Site {
			existing[k] = true
		}
	}

	markerID, err := GenerateMarkerID(existing)
	if err != nil {
		return err
	}

	line := FormatMarkerLine(markerID, clauseIDs)
	fmt.Println(line)
	fmt.Println()
	fmt.Printf("%s is your new filament marker. Paste it into any text file —\n", markerID)
	fmt.Println("source code, documentation, configuration, or plain text — above or")
	fmt.Println("near the content that covers these clauses. Markers create a traceable")
	fmt.Println("link from spec to file; without one, filament can't tell whether a")
	fmt.Println("clause is actually addressed anywhere. After pasting, run 'filament")
	fmt.Println("init' (if you don't have a state file yet) or 'filament resolve --site")
	fmt.Printf("%s' to register the marker's content hash.\n", markerID)
	return nil
}

// #F id:pbof4ddy public_api.resolve
func runResolve(specPath string, args []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	if len(args) < 2 {
		return fmt.Errorf("resolve requires --spec or --site flag and at least one marker id")
	}

	scope := args[0]
	markerIDs := args[1:]

	if scope != "--spec" && scope != "--site" {
		return fmt.Errorf("resolve requires --spec or --site as first argument, got %q", scope)
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	lockPath := LockFilePath(specPath)
	lock, err := ReadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("cannot read state file: %w", err)
	}

	specHashes := ComputeAllHashes(spec)

	if scope == "--spec" {
		for _, mid := range markerIDs {
			// Update reviewed spec hash for all clauses this marker references
			for key := range lock.State {
				parts := strings.SplitN(key, ":", 2)
				if len(parts) == 2 && parts[0] == mid {
					clauseID := parts[1]
					lock.State[key] = specHashes[clauseID]
				}
			}
		}
		fmt.Printf("Cleared spec drift for %d marker(s): %s\n", len(markerIDs), strings.Join(markerIDs, ", "))
		fmt.Println()
		fmt.Println("The reviewed spec hashes for these sites now match the current spec.")
		fmt.Println("Each site was cleared individually — if other markers reference the")
		fmt.Println("same clause, they remain flagged until you review and clear them too.")
		fmt.Println("Run 'filament check' to verify.")
	} else {
		// --site: update content hash
		paths := []string{"."}
		files, err := WalkPaths(paths)
		if err != nil {
			return err
		}
		windowSize := defaultContentWindow

		for _, mid := range markerIDs {
			found := false
			for _, f := range files {
				markers, err := ScanMarkers(f)
				if err != nil {
					continue
				}
				for _, m := range markers {
					if m.MarkerID == mid {
						contentHash, err := ComputeContentHash(f, m.Line, windowSize)
						if err != nil {
							return fmt.Errorf("computing content hash for %s:%d: %w", f, m.Line, err)
						}
						lock.Site[mid] = contentHash

						// Create state entries for clauses that don't have them yet
						for _, cid := range m.ClauseIDs {
							stateKey := mid + ":" + cid
							if _, exists := lock.State[stateKey]; !exists {
								lock.State[stateKey] = specHashes[cid]
							}
						}

						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf("marker %q not found in scanned files", mid)
			}
		}
		fmt.Printf("Cleared site drift for %d marker(s): %s\n", len(markerIDs), strings.Join(markerIDs, ", "))
		fmt.Println()
		fmt.Println("The content hashes for these markers have been updated to match the")
		fmt.Println("current text. If the spec itself changed, that will show up separately")
		fmt.Println("in 'filament check' — resolving site drift only clears the")
		fmt.Println("content-changed signal.")
	}

	if err := WriteLockFile(lockPath, lock); err != nil {
		return err
	}

	return nil
}

// #F id:38zr23u2 public_api.sync
func runSync(specPath string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	lockPath := LockFilePath(specPath)
	lock, err := ReadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("cannot read state file: %w", err)
	}

	specHashes := ComputeAllHashes(spec)
	updated := 0
	for clauseID, hash := range specHashes {
		if lock.Spec[clauseID] != hash {
			updated++
		}
		lock.Spec[clauseID] = hash
	}

	if err := WriteLockFile(lockPath, lock); err != nil {
		return err
	}

	fmt.Printf("Refreshed [spec] section: %d clause hashes updated from %s.\n", updated, specPath)
	fmt.Println()
	fmt.Println("The spec section of the state file now reflects the current spec.")
	fmt.Println("Any site whose reviewed spec hash doesn't match will show SPEC_DRIFT")
	fmt.Println("in 'filament check'. Review each drifted site and clear it with")
	fmt.Println("'filament resolve --spec <marker_id>'.")
	return nil
}


// #F id:bookg95x public_api.migrate
func runMigrate(specPath string, paths []string, quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	spec, violations, err := ParseSpecFile(specPath)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		printViolations(violations, quiet)
		return fmt.Errorf("spec has %d violation(s)", len(violations))
	}

	specHashes := ComputeAllHashes(spec)
	lock := NewLockFile()
	windowSize := defaultContentWindow

	// Populate spec section
	for clauseID, hash := range specHashes {
		lock.Spec[clauseID] = hash
	}

	// Scan for old filament:hash comments and convert them
	files, err := WalkPaths(paths)
	if err != nil {
		return err
	}

	totalMigrated := 0
	totalFiles := 0
	existingIDs := make(map[string]bool)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		changed := false

		// Find groups of adjacent filament:hash lines
		var groups [][]int // each group is a slice of line indices
		var currentGroup []int
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "// filament:hash ") || strings.HasPrefix(trimmed, "# filament:hash ") || strings.HasPrefix(trimmed, "-- filament:hash ") || strings.HasPrefix(trimmed, "<!-- filament:hash ") {
				currentGroup = append(currentGroup, i)
			} else {
				if len(currentGroup) > 0 {
					groups = append(groups, currentGroup)
					currentGroup = nil
				}
			}
		}
		if len(currentGroup) > 0 {
			groups = append(groups, currentGroup)
		}

		if len(groups) == 0 {
			continue
		}

		// Process each group
		for _, group := range groups {
			// Extract clause ids from the group
			var clauseIDs []string
			for _, lineIdx := range group {
				trimmed := strings.TrimSpace(lines[lineIdx])
				// Parse: directive clause_id=hash
				parts := strings.SplitN(trimmed, " ", 3)
				if len(parts) >= 3 {
					clauseHash := parts[2]
					clauseParts := strings.SplitN(clauseHash, "=", 2)
					if len(clauseParts) >= 1 {
						clauseIDs = append(clauseIDs, clauseParts[0])
					}
				}
			}

			if len(clauseIDs) == 0 {
				continue
			}

			// Generate marker id
			markerID, err := GenerateMarkerID(existingIDs)
			if err != nil {
				return err
			}
			existingIDs[markerID] = true

			// Build new marker line with comment prefix preserved
			indent := ""
			prefix := ""
			if len(group) > 0 {
				line := lines[group[0]]
				// Extract indentation
				i := 0
				for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
					i++
				}
				indent = line[:i]
				// Extract comment prefix (everything before "filament:hash")
				rest := line[i:]
				directiveIdx := strings.Index(rest, "filament:hash")
				if directiveIdx >= 0 {
					prefix = rest[:directiveIdx]
				}
			}

			newLine := indent + prefix + FormatMarkerLine(markerID, clauseIDs)

			// Replace first line of group with new marker, blank out rest
			lines[group[0]] = newLine
			for _, lineIdx := range group[1:] {
				lines[lineIdx] = ""
			}

			// Register in lock file
			for _, cid := range clauseIDs {
				stateKey := markerID + ":" + cid
				if h, ok := specHashes[cid]; ok {
					lock.State[stateKey] = h
				}
			}

			totalMigrated += len(clauseIDs)
			changed = true
		}

		if changed {
			totalFiles++
			if err := os.WriteFile(f, []byte(strings.Join(lines, "\n")), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", f, err)
			}
		}
	}

	// Now scan all files to populate site hashes
	files, err = WalkPaths(paths)
	if err != nil {
		return err
	}
	for _, f := range files {
		markers, err := ScanMarkers(f)
		if err != nil {
			continue
		}
		for _, m := range markers {
			if !MarkerIDIsValid(m.MarkerID) {
				continue
			}
			contentHash, err := ComputeContentHash(f, m.Line, windowSize)
			if err != nil {
				continue
			}
			lock.Site[m.MarkerID] = contentHash
		}
	}

	lockPath := LockFilePath(specPath)
	if err := WriteLockFile(lockPath, lock); err != nil {
		return err
	}

	fmt.Printf("Migrated %d filament:hash comments to #F markers across %d files.\n", totalMigrated, totalFiles)
	fmt.Printf("Created %s with %d spec hashes, %d site entries.\n", lockPath, len(lock.Spec), len(lock.Site))
	fmt.Println()
	fmt.Println("The old filament:hash format embedded 64-character hashes directly in")
	fmt.Println("source files. The new #F format uses marker ids that reference the state")
	fmt.Println("file, keeping source clean and enabling bidirectional drift detection.")
	fmt.Println("Run 'filament check' to verify the migration.")
	return nil
}

// #F id:6uv349nx public_api.skill
func runSkill(quiet bool) error {
	if !quiet {
		fmt.Fprint(os.Stderr, Tooltip)
	}

	fmt.Print(SkillText)
	return nil
}

// windowSizeFromEnv reads the content window size from the environment,
// falling back to defaultContentWindow.
// #F id:u1usd213 public_api.file_walk
func windowSizeFromEnv() int {
	s := os.Getenv("FILAMENT_WINDOW_SIZE")
	if s == "" {
		return defaultContentWindow
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultContentWindow
	}
	return n
}
