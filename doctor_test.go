package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSpecVersion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "with version",
			content: `<spec name="test" version="0.1.0">`,
			want:    "0.1.0",
		},
		{
			name:    "without version",
			content: `<spec name="test">`,
			want:    "0.0.0",
		},
		{
			name:    "with version and other attrs",
			content: `<spec name="test" version="0.2.0" other="value">`,
			want:    "0.2.0",
		},
		{
			name:    "multiline spec tag",
			content: "<spec\n  name=\"test\"\n  version=\"0.1.0\">",
			want:    "0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSpecVersion(tt.content)
			if got != tt.want {
				t.Errorf("DetectSpecVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLatestVersion(t *testing.T) {
	got := LatestVersion()
	if got == "" {
		t.Error("LatestVersion() should not be empty")
	}
}

func TestGetPendingMigrations(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		wantCount      int
	}{
		{
			name:           "from beginning",
			currentVersion: "0.0.0",
			wantCount:      len(migrations),
		},
		{
			name:           "from first version",
			currentVersion: "0.1.0",
			wantCount:      len(migrations) - 1,
		},
		{
			name:           "at latest",
			currentVersion: LatestVersion(),
			wantCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPendingMigrations(tt.currentVersion)
			if len(got) != tt.wantCount {
				t.Errorf("GetPendingMigrations(%q) returned %d migrations, want %d",
					tt.currentVersion, len(got), tt.wantCount)
			}
		})
	}
}

func TestMigrateSpec(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "add version to unversioned spec",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<spec name="test">
  <clause id="first">A clause.</clause>
</spec>`,
			wantErr: false,
		},
		{
			name: "already at latest",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<spec name="test" version="` + LatestVersion() + `">
  <clause id="first">A clause.</clause>
</spec>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, newVersion, err := MigrateSpec(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("MigrateSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Error("MigrateSpec() returned empty content")
			}
			if !tt.wantErr && newVersion == "" {
				t.Error("MigrateSpec() returned empty version")
			}
		})
	}
}

func TestRunMigrateSpec(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.xml")

	// Create an unversioned spec
	specContent := `<?xml version="1.0" encoding="UTF-8"?>
<spec name="test">
  <clause id="first">A clause.</clause>
</spec>
`
	os.WriteFile(specPath, []byte(specContent), 0644)

	// Run migrate-spec
	err := runMigrateSpec([]string{"--spec=" + specPath})
	if err != nil {
		t.Fatalf("runMigrateSpec() error = %v", err)
	}

	// Read migrated spec
	migrated, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("cannot read migrated spec: %v", err)
	}

	// Verify version was added
	version := DetectSpecVersion(string(migrated))
	if version == "" {
		t.Error("migrated spec should have a version")
	}
}

func TestRunMigrateSpecAlreadyLatest(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.xml")

	// Create a spec at latest version
	specContent := `<?xml version="1.0" encoding="UTF-8"?>
<spec name="test" version="` + LatestVersion() + `">
  <clause id="first">A clause.</clause>
</spec>
`
	os.WriteFile(specPath, []byte(specContent), 0644)

	// Run migrate-spec
	err := runMigrateSpec([]string{"--spec=" + specPath})
	if err != nil {
		t.Fatalf("runMigrateSpec() error = %v", err)
	}
}

func TestRunMigrateSpecMissingFile(t *testing.T) {
	err := runMigrateSpec([]string{"--spec=nonexistent.xml"})
	if err == nil {
		t.Error("runMigrateSpec() should fail for missing file")
	}
}

func TestRunDoctorNoSubcommand(t *testing.T) {
	err := runDoctor([]string{})
	if err == nil {
		t.Error("runDoctor() should fail with no subcommand")
	}
}

func TestRunDoctorUnknownSubcommand(t *testing.T) {
	err := runDoctor([]string{"unknown"})
	if err == nil {
		t.Error("runDoctor() should fail with unknown subcommand")
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"0.0.0", true},
		{"1.0.0", true},
		{"0.1.0", true},
		{"0.2.0", true},
		{"10.20.30", true},
		{"", false},
		{"1", false},
		{"1.0", false},
		{"1.0.0.0", false},
		{"v1.0.0", false},
		{"1.0.0-beta", false},
		{"a.b.c", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := IsValidVersion(tt.version)
			if got != tt.want {
				t.Errorf("IsValidVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
