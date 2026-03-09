package config

import (
	"path/filepath"
	"testing"
)

func TestDatabasePathDefaultsToHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "/tmp/callmeback-home")
	t.Setenv("CALLMEBACK_DB", "")

	got, err := DatabasePath()
	if err != nil {
		t.Fatalf("DatabasePath() error = %v", err)
	}

	want := filepath.Join("/tmp/callmeback-home", ".callmeback", "callmeback.db")
	if got != want {
		t.Fatalf("DatabasePath() = %q, want %q", got, want)
	}
}

func TestDatabasePathUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("HOME", "/tmp/callmeback-home")
	t.Setenv("CALLMEBACK_DB", "/tmp/custom/callmeback.db")

	got, err := DatabasePath()
	if err != nil {
		t.Fatalf("DatabasePath() error = %v", err)
	}

	want := "/tmp/custom/callmeback.db"
	if got != want {
		t.Fatalf("DatabasePath() = %q, want %q", got, want)
	}
}

func TestDatabaseDirMatchesResolvedPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/callmeback-home")
	t.Setenv("CALLMEBACK_DB", "/tmp/custom/callmeback.db")

	got, err := DatabaseDir()
	if err != nil {
		t.Fatalf("DatabaseDir() error = %v", err)
	}

	want := "/tmp/custom"
	if got != want {
		t.Fatalf("DatabaseDir() = %q, want %q", got, want)
	}
}
