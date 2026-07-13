package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Check(t *testing.T) {
	binary := buildBinary(t)

	tests := []struct {
		name     string
		spec     string
		lock     string
		fixture  string
		wantFail bool
	}{
		{"valid", "testdata/golden.spec.xml", "testdata/fixture_new_valid.filament", "testdata/fixture_new_valid.go", false},
		{"spec_drift", "testdata/golden.spec.xml", "testdata/fixture_new_spec_drift.filament", "testdata/fixture_new_spec_drift.go", true},
		{"site_drift", "testdata/golden.spec.xml", "testdata/fixture_new_site_drift.filament", "testdata/fixture_new_site_drift.go", true},
		{"missing_lock", "testdata/golden.spec.xml", "", "testdata/fixture_new_valid.go", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Copy spec to temp dir
			specData, err := os.ReadFile(tt.spec)
			if err != nil {
				t.Fatal(err)
			}
			specPath := filepath.Join(dir, "spec.xml")
			os.WriteFile(specPath, specData, 0644)

			// Copy lock file if provided
			if tt.lock != "" {
				lockData, err := os.ReadFile(tt.lock)
				if err != nil {
					t.Fatal(err)
				}
				os.WriteFile(filepath.Join(dir, ".filament"), lockData, 0644)
			}

			// Copy fixture
			fixData, err := os.ReadFile(tt.fixture)
			if err != nil {
				t.Fatal(err)
			}
			fixPath := filepath.Join(dir, "fixture.go")
			os.WriteFile(fixPath, fixData, 0644)

			cmd := exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
			err = cmd.Run()
			if tt.wantFail && err == nil {
				t.Errorf("expected check to fail, but it succeeded")
			}
			if !tt.wantFail && err != nil {
				t.Errorf("expected check to succeed, but it failed: %v", err)
			}
		})
	}
}

func TestCLI_Subcommands(t *testing.T) {
	binary := buildBinary(t)

	for _, sub := range []string{"add", "resolve"} {
		cmd := exec.Command(binary, sub)
		if err := cmd.Run(); err == nil {
			t.Errorf("subcommand %s with no args should fail, but it succeeded", sub)
		}
	}
}

func TestCLI_Add(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "add", "--spec=testdata/golden.spec.xml", "--quiet", "x")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("add command failed: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "#F id:") {
		t.Errorf("add output should contain '#F id:', got: %s", got)
	}
	if !strings.Contains(got, "x") {
		t.Errorf("add output should contain clause 'x', got: %s", got)
	}
}

func TestCLI_AddMultipleClauses(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "add", "--spec=testdata/golden.spec.xml", "--quiet", "x", "y")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("add command failed: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "x") || !strings.Contains(got, "y") {
		t.Errorf("add output should contain both clauses, got: %s", got)
	}
}

func TestCLI_AddNonexistent(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "add", "--spec=testdata/golden.spec.xml", "--quiet", "nonexistent")
	if err := cmd.Run(); err == nil {
		t.Errorf("add for nonexistent clause should fail, but it succeeded")
	}
}

func TestCLI_Skill(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "skill", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("skill command failed: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "WHAT IS FILAMENT") {
		t.Errorf("skill output should contain 'WHAT IS FILAMENT', got: %s", got[:100])
	}
	if !strings.Contains(got, "THE MARKER FORMAT") {
		t.Errorf("skill output should contain 'THE MARKER FORMAT'")
	}
}

func TestCLI_SkillQuiet(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "skill", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("skill --quiet failed: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "This is a tooltip") {
		t.Errorf("skill --quiet should not contain tooltip, got: %s", got[:200])
	}
}

// #F id:ocwydxem self_hosting.test
func TestCLISelfHosting(t *testing.T) {
	binary := buildBinary(t)

	dir := t.TempDir()

	// Copy spec
	specData, err := os.ReadFile("filament.spec.xml")
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	// Create fixture with markers for three clauses.
	// Split "#F" from "id:" so the scanner doesn't match this string literal.
	fixture := filepath.Join(dir, "selftest.go")
	markerDir := "#F" // filament marker directive
	fixtureContent := "package filament_test\n\n// " + markerDir + " id:selfhos1 tool.name public_api.subcommands path_format.structure\nfunc ExampleSelfHost() {\n\t_ = \"self-hosting test\"\n}\n"
	os.WriteFile(fixture, []byte(fixtureContent), 0644)

	// Build file list: fixture + source files + docs + test with real markers
	filamentDir, _ := filepath.Abs(".")
	fileList := []string{
		filepath.Join(dir, "selftest.go"),
		filepath.Join(filamentDir, "main.go"),
		filepath.Join(filamentDir, "comment.go"),
		filepath.Join(filamentDir, "spec.go"),
		filepath.Join(filamentDir, "parser.go"),
		filepath.Join(filamentDir, "util.go"),
		filepath.Join(filamentDir, "skill.go"),
		filepath.Join(filamentDir, "marker.go"),
		filepath.Join(filamentDir, "lock.go"),
		filepath.Join(filamentDir, "site.go"),
		filepath.Join(filamentDir, "doctor.go"),
		filepath.Join(filamentDir, "cli_test.go"),
		filepath.Join(filamentDir, "CONTRIBUTING.md"),
		filepath.Join(filamentDir, "scripts", "install.sh"),
		filepath.Join(filamentDir, "scripts", "install.ps1"),
	}

	// Init to create .filament
	initArgs := append([]string{"init", "--spec=" + specPath, "--quiet"}, fileList...)
	initCmd := exec.Command(binary, initArgs...)
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Check should pass
	checkArgs := append([]string{"check", "--spec=" + specPath, "--quiet"}, fileList...)
	checkCmd := exec.Command(binary, checkArgs...)
	checkCmd.Dir = dir
	if out, err := checkCmd.CombinedOutput(); err != nil {
		t.Errorf("self-hosting check failed: %v\n%s", err, out)
	}
}

func TestCLI_Init(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Copy spec and fixture
	specData, _ := os.ReadFile("testdata/golden.spec.xml")
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	fixData, _ := os.ReadFile("testdata/fixture_new_valid.go")
	fixPath := filepath.Join(dir, "fixture.go")
	os.WriteFile(fixPath, fixData, 0644)

	// Init should create .filament
	cmd := exec.Command(binary, "init", "--spec="+specPath, "--quiet", fixPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// .filament should exist
	lockPath := filepath.Join(dir, ".filament")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Errorf(".filament was not created")
	}

	// Check should pass after init
	cmd = exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
	if err := cmd.Run(); err != nil {
		t.Errorf("check after init should pass, but failed: %v", err)
	}
}

func TestCLI_ResolveSite(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Set up: spec + fixture + init
	specData, _ := os.ReadFile("testdata/golden.spec.xml")
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	fixData, _ := os.ReadFile("testdata/fixture_new_valid.go")
	fixPath := filepath.Join(dir, "fixture.go")
	os.WriteFile(fixPath, fixData, 0644)

	// Init
	initCmd := exec.Command(binary, "init", "--spec="+specPath, "--quiet", fixPath)
	initCmd.Dir = dir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Modify the fixture content (site drift)
	modified := strings.Replace(string(fixData), `const X = "a"`, `const X = "modified"`, 1)
	os.WriteFile(fixPath, []byte(modified), 0644)

	// Check should fail (site drift)
	check1 := exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
	check1.Dir = dir
	if err := check1.Run(); err == nil {
		t.Fatalf("check should fail after content change, but succeeded")
	}

	// Resolve site drift for the marker in the fixture
	// We need to find the marker id. Scan the fixture for #F id:
	markerID := ""
	for _, line := range strings.Split(string(fixData), "\n") {
		if m := markerPattern.FindStringSubmatch(line); m != nil {
			markerID = m[1]
			break
		}
	}
	if markerID == "" {
		t.Fatal("no marker found in fixture")
	}

	resolveCmd := exec.Command(binary, "resolve", "--spec="+specPath, "--quiet", "--site", markerID)
	resolveCmd.Dir = dir
	if err := resolveCmd.Run(); err != nil {
		t.Fatalf("resolve --site failed: %v", err)
	}

	// Check should pass now
	check2 := exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
	check2.Dir = dir
	if err := check2.Run(); err != nil {
		t.Errorf("check after resolve --site should pass, but failed: %v", err)
	}
}

func TestCLI_ResolveSiteCreatesStateEntries(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Set up: spec + fixture + init
	specData, _ := os.ReadFile("testdata/golden.spec.xml")
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	fixData, _ := os.ReadFile("testdata/fixture_new_valid.go")
	fixPath := filepath.Join(dir, "fixture.go")
	os.WriteFile(fixPath, fixData, 0644)

	// Init
	initCmd := exec.Command(binary, "init", "--spec="+specPath, "--quiet", fixPath)
	initCmd.Dir = dir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Read the lock file and delete state entries for marker aaaa1111
	lockPath := filepath.Join(dir, ".filament")
	lockData, _ := os.ReadFile(lockPath)
	lockStr := string(lockData)
	// Remove state entries for aaaa1111
	var newLines []string
	for _, line := range strings.Split(lockStr, "\n") {
		if strings.HasPrefix(line, "aaaa1111:") {
			continue
		}
		newLines = append(newLines, line)
	}
	os.WriteFile(lockPath, []byte(strings.Join(newLines, "\n")), 0644)

	// Verify state entries are gone
	lockData, _ = os.ReadFile(lockPath)
	if strings.Contains(string(lockData), "aaaa1111:") {
		t.Fatal("state entries for aaaa1111 should have been deleted")
	}

	// Resolve --site should recreate the state entries
	resolveCmd := exec.Command(binary, "resolve", "--spec="+specPath, "--quiet", "--site", "aaaa1111")
	resolveCmd.Dir = dir
	if err := resolveCmd.Run(); err != nil {
		t.Fatalf("resolve --site failed: %v", err)
	}

	// Verify state entries were recreated
	lockData, _ = os.ReadFile(lockPath)
	if !strings.Contains(string(lockData), "aaaa1111:") {
		t.Errorf("resolve --site should have created state entries for aaaa1111")
	}
}

func TestCLI_ResolveSpec(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Set up: spec + fixture + init
	specData, _ := os.ReadFile("testdata/golden.spec.xml")
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	fixData, _ := os.ReadFile("testdata/fixture_new_valid.go")
	fixPath := filepath.Join(dir, "fixture.go")
	os.WriteFile(fixPath, fixData, 0644)

	// Init
	initCmd := exec.Command(binary, "init", "--spec="+specPath, "--quiet", fixPath)
	initCmd.Dir = dir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Modify the spec (spec drift)
	modifiedSpec := strings.Replace(string(specData), `<clause id="x">a</clause>`, `<clause id="x">a modified</clause>`, 1)
	os.WriteFile(specPath, []byte(modifiedSpec), 0644)

	// Check should fail (spec drift)
	check1 := exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
	check1.Dir = dir
	if err := check1.Run(); err == nil {
		t.Fatalf("check should fail after spec change, but succeeded")
	}

	// Find all marker ids
	var markerIDs []string
	for _, line := range strings.Split(string(fixData), "\n") {
		if m := markerPattern.FindStringSubmatch(line); m != nil {
			markerIDs = append(markerIDs, m[1])
		}
	}
	if len(markerIDs) == 0 {
		t.Fatal("no markers found in fixture")
	}

	// Sync spec hashes
	syncCmd := exec.Command(binary, "sync", "--spec="+specPath, "--quiet")
	syncCmd.Dir = dir
	if err := syncCmd.Run(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Resolve spec drift for all markers
	args := []string{"resolve", "--spec=" + specPath, "--quiet", "--spec"}
	args = append(args, markerIDs...)
	resolveCmd := exec.Command(binary, args...)
	resolveCmd.Dir = dir
	if err := resolveCmd.Run(); err != nil {
		t.Fatalf("resolve --spec failed: %v", err)
	}

	// Check should pass now
	check2 := exec.Command(binary, "check", "--spec="+specPath, "--quiet", fixPath)
	check2.Dir = dir
	if err := check2.Run(); err != nil {
		t.Errorf("check after resolve --spec should pass, but failed: %v", err)
	}
}

func TestCLI_Sync(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Set up: spec + fixture + init
	specData, _ := os.ReadFile("testdata/golden.spec.xml")
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, specData, 0644)

	fixData, _ := os.ReadFile("testdata/fixture_new_valid.go")
	fixPath := filepath.Join(dir, "fixture.go")
	os.WriteFile(fixPath, fixData, 0644)

	// Init
	cmd := exec.Command(binary, "init", "--spec="+specPath, "--quiet", fixPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Modify the spec
	modifiedSpec := strings.Replace(string(specData), `<clause id="x">a</clause>`, `<clause id="x">a modified</clause>`, 1)
	os.WriteFile(specPath, []byte(modifiedSpec), 0644)

	// Sync should refresh [spec] section
	cmd = exec.Command(binary, "sync", "--spec="+specPath, "--quiet")
	if err := cmd.Run(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Read the lock file and verify spec hash for x was updated
	lockData, err := os.ReadFile(filepath.Join(dir, ".filament"))
	if err != nil {
		t.Fatal(err)
	}
	lockStr := string(lockData)
	if !strings.Contains(lockStr, "x=") {
		t.Errorf("lock file should contain clause x")
	}
}

func TestCLI_Migrate(t *testing.T) {
	binary := buildBinary(t)
	dir := t.TempDir()

	// Create a spec
	specContent := `<?xml version="1.0" encoding="UTF-8"?>
<spec name="migrate_test">
  <clause id="alpha">first clause</clause>
  <clause id="beta">second clause</clause>
</spec>
`
	specPath := filepath.Join(dir, "spec.xml")
	os.WriteFile(specPath, []byte(specContent), 0644)

	// Create a file with old filament:hash comments
	oldContent := `package main

// filament:hash alpha=ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb
// filament:hash beta=3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d
func main() {}
`
	srcPath := filepath.Join(dir, "main.go")
	os.WriteFile(srcPath, []byte(oldContent), 0644)

	// Migrate
	cmd := exec.Command(binary, "migrate", "--spec="+specPath, "--quiet", srcPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Verify old comments are replaced with #F markers
	migrated, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	migratedStr := string(migrated)
	if strings.Contains(migratedStr, "filament:hash") {
		t.Errorf("migrated file should not contain 'filament:hash'")
	}
	if !strings.Contains(migratedStr, "#F id:") {
		t.Errorf("migrated file should contain '#F id:'")
	}

	// Check should pass on migrated file
	cmd = exec.Command(binary, "check", "--spec="+specPath, "--quiet", srcPath)
	if err := cmd.Run(); err != nil {
		t.Errorf("check after migrate should pass, but failed: %v", err)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "filament")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}
	return binary
}
