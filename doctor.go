package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// #F id:5k4j8r5y doctor.migration_pipeline

// semverPattern matches MAJOR.MINOR.PATCH where each component is a non-negative integer.
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// Migration represents a spec migration from one version to another.
type Migration struct {
	FromVersion string
	ToVersion   string
	Description string
	Migrate     func(specContent string) (string, error)
}

// migrations is the ordered list of all spec migrations.
var migrations = []Migration{
	{
		FromVersion: "0.0.0",
		ToVersion:   "0.1.0",
		Description: "Initial version with <ref> element support",
		Migrate:     migrateToV010,
	},
	{
		FromVersion: "0.1.0",
		ToVersion:   "0.2.0",
		Description: "Add transitive coverage semantics",
		Migrate:     migrateToV020,
	},
}

// LatestVersion returns the latest spec version.
func LatestVersion() string {
	if len(migrations) == 0 {
		return "0.0.0"
	}
	return migrations[len(migrations)-1].ToVersion
}

// #F id:d4ctv001 doctor.version_detection
// DetectSpecVersion reads the version from spec XML content.
// If no version attribute is present, returns "0.0.0" (pre-versioned).
func DetectSpecVersion(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<spec") {
			idx := strings.Index(trimmed, "version=\"")
			if idx >= 0 {
				start := idx + len("version=\"")
				end := strings.Index(trimmed[start:], "\"")
				if end >= 0 {
					return trimmed[start : start+end]
				}
			}
			return "0.0.0"
		}
	}
	return "0.0.0"
}

// IsValidVersion checks whether a version string conforms to
// semantic versioning (MAJOR.MINOR.PATCH).
func IsValidVersion(version string) bool {
	return semverPattern.MatchString(version)
}

// GetPendingMigrations returns migrations needed to go from current to latest.
func GetPendingMigrations(currentVersion string) []Migration {
	var pending []Migration
	found := currentVersion == "0.0.0" // Start from beginning if pre-versioned

	for _, m := range migrations {
		if found {
			pending = append(pending, m)
		}
		if m.ToVersion == currentVersion {
			found = true
		}
	}
	return pending
}

// MigrateSpec applies all pending migrations to spec content.
func MigrateSpec(content string) (string, string, error) {
	currentVersion := DetectSpecVersion(content)
	pending := GetPendingMigrations(currentVersion)

	if len(pending) == 0 {
		return content, currentVersion, nil
	}

	result := content
	for _, m := range pending {
		fmt.Printf("Migrating %s -> %s: %s\n", m.FromVersion, m.ToVersion, m.Description)
		var err error
		result, err = m.Migrate(result)
		if err != nil {
			return "", currentVersion, fmt.Errorf("migration %s -> %s failed: %w", m.FromVersion, m.ToVersion, err)
		}
	}

	return result, pending[len(pending)-1].ToVersion, nil
}

// migrateToV010 migrates from pre-versioning to v0.1.0.
// This adds the version attribute to <spec> and converts implicit prose
// references to explicit <ref> elements.
func migrateToV010(content string) (string, error) {
	// Add version attribute to <spec> tag if not present
	// Check only the <spec> tag line, not the entire content (XML declaration has version= too)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<spec") && strings.HasSuffix(trimmed, ">") && !strings.Contains(trimmed, "version=") {
			lines[i] = strings.TrimSuffix(line, ">") + " version=\"0.1.0\">"
			break
		}
	}
	content = strings.Join(lines, "\n")

	return content, nil
}

// migrateToV020 migrates from v0.1.0 to v0.2.0.
// This adds transitive coverage semantics.
func migrateToV020(content string) (string, error) {
	// Update version to 0.2.0
	content = strings.Replace(content, "version=\"0.1.0\"", "version=\"0.2.0\"", 1)

	// Add new terms if not present
	if !strings.Contains(content, "<term text=\"covered\">") {
		// Add covered term after missing term
		coveredTerm := `    <term text="covered">An element is covered if it has a well_formed_marker (clauses only), or if a covered element references it via a ref. Coverage is transitive through both clauses and terms. Only clauses can be MISSING; terms and sections are never reported regardless of coverage.</term>`
		content = strings.Replace(content,
			`<term text="missing">A clause in the spec that has no well_formed_marker in any scanned file. Only clauses are subject to this condition; sections and terms are structural or vocabulary and are never reported.</term>`,
			`<term text="missing">A clause in the spec that is not covered. Only clauses are subject to this condition; sections and terms are structural or vocabulary and are never reported.</term>
`+coveredTerm,
			1)
	}

	// Add ref term if not present
	if !strings.Contains(content, "<term text=\"ref\">") {
		refTerm := `    <term text="ref">An inline element inside a clause or term that creates a reference to another element. The text content of a ref must be a defined id.</term>`
		content = strings.Replace(content,
			`<term text="reference">A maximal run of [a-z0-9_.]+ in a clause's or term's prose content that, after stripping any trailing characters not in [a-z0-9_], exactly matches the id of a defined clause, section, or term in the same spec.</term>`,
			`<term text="reference">An inline ref element inside a clause or term. The text content of the ref is the id of a defined clause, section, or term in the same spec.</term>
`+refTerm,
			1)
	}

	// Update drift.missing clause
	if strings.Contains(content, "A clause in the spec that has no well_formed_marker") {
		content = strings.Replace(content,
			`<clause id="drift.missing">A clause in the spec that has no well_formed_marker in any scanned file MUST be reported as a finding with the status MISSING.</clause>`,
			`<clause id="drift.missing">A clause in the spec that is not covered MUST be reported as a finding with the status MISSING.</clause>
    <clause id="drift.transitive_coverage">A clause is covered if it has a well_formed_marker in any scanned file, or if a covered clause or term references it via a ref. The tool MUST compute the transitive closure of coverage starting from all clauses that have well_formed_markers.</clause>`,
			1)
	}

	return content, nil
}

// #F id:d4ctr001 doctor.subcommand
// runDoctor handles the doctor command and its subcommands.
func runDoctor(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("doctor requires a subcommand. Use 'filament doctor migrate-spec' to migrate spec XML")
	}

	subcommand := args[0]
	switch subcommand {
	case "migrate-spec":
		return runMigrateSpec(args[1:])
	default:
		return fmt.Errorf("unknown doctor subcommand: %s", subcommand)
	}
}

// #F id:d4ctm001 doctor.migrate_spec
// runMigrateSpec handles the migrate-spec subcommand.
func runMigrateSpec(args []string) error {
	specPath := "./filament.spec.xml"
	for _, arg := range args {
		if strings.HasPrefix(arg, "--spec=") {
			specPath = strings.TrimPrefix(arg, "--spec=")
		}
	}

	// Read spec file
	content, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("cannot read spec file: %w", err)
	}

	// Detect current version
	currentVersion := DetectSpecVersion(string(content))
	if !IsValidVersion(currentVersion) {
		return fmt.Errorf("invalid spec version %q: expected MAJOR.MINOR.PATCH format", currentVersion)
	}
	fmt.Printf("Current spec version: %s\n", currentVersion)
	fmt.Printf("Latest spec version: %s\n", LatestVersion())

	// Apply migrations
	migrated, newVersion, err := MigrateSpec(string(content))
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	if newVersion == currentVersion {
		fmt.Println("Spec is already at the latest version.")
		return nil
	}

	// Write migrated spec
	if err := os.WriteFile(specPath, []byte(migrated), 0644); err != nil {
		return fmt.Errorf("cannot write migrated spec: %w", err)
	}

	fmt.Printf("Spec migrated from %s to %s\n", currentVersion, newVersion)
	return nil
}
