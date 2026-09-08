package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validCheck() Check {
	return Check{
		Name:                     "check-ok",
		Target:                   "test-host",
		Command:                  "./test_checks/check-ok.sh",
		Timeout:                  time.Second,
		Interval:                 time.Minute,
		Attempts:                 3,
		CriticalReminderInterval: 10 * time.Minute,
	}
}

func TestValidateCheck(t *testing.T) {
	t.Run("valid check", func(t *testing.T) {
		if err := validateCheck(validCheck()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		check := validCheck()
		check.Name = ""
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("missing command", func(t *testing.T) {
		check := validCheck()
		check.Command = ""
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		check := validCheck()
		check.Timeout = 0
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("negative interval", func(t *testing.T) {
		check := validCheck()
		check.Interval = -time.Second
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("zero attempts", func(t *testing.T) {
		check := validCheck()
		check.Attempts = 0
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("timeout equal to interval", func(t *testing.T) {
		check := validCheck()
		check.Timeout = time.Minute
		check.Interval = time.Minute

		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})
}

func TestValidateAllChecks(t *testing.T) {
	t.Run("unique names ok", func(t *testing.T) {
		checks := []Check{
			{Name: "a"},
			{Name: "b"},
		}
		if err := validateAllChecks(checks); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate names error", func(t *testing.T) {
		checks := []Check{
			{Name: "a"},
			{Name: "a"},
		}
		if err := validateAllChecks(checks); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("empty list ok", func(t *testing.T) {
		if err := validateAllChecks(nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateDefaults(t *testing.T) {
	valid := DefaultsConfig{Timeout: "10s", Interval: "1m", Attempts: 3}

	t.Run("valid defaults", func(t *testing.T) {
		if err := validateDefaults(valid); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid timeout string", func(t *testing.T) {
		d := valid
		d.Timeout = "notaduration"

		err := validateDefaults(d)

		if err == nil {
			t.Fatal("expected error, got none")
		}

		if !strings.Contains(err.Error(), "invalid default timeout") {
			t.Errorf("unexpected error: %v", err)
		}

	})

	t.Run("zero timeout", func(t *testing.T) {
		d := valid
		d.Timeout = "0s"
		if err := validateDefaults(d); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("invalid interval string", func(t *testing.T) {
		d := valid
		d.Interval = "notaduration"

		err := validateDefaults(d)

		if err == nil {
			t.Fatal("expected error, got none")
		}

		if !strings.Contains(err.Error(), "invalid default interval") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("zero interval", func(t *testing.T) {
		d := valid
		d.Interval = "0s"
		if err := validateDefaults(d); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("zero attempts", func(t *testing.T) {
		d := valid
		d.Attempts = 0
		if err := validateDefaults(d); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("timeout equal to interval is invalid", func(t *testing.T) {
		d := DefaultsConfig{Timeout: "1m", Interval: "1m", Attempts: 1}
		if err := validateDefaults(d); err == nil {
			t.Errorf("expected error, got none")
		}
	})

	t.Run("timeout greater than interval is invalid", func(t *testing.T) {
		d := DefaultsConfig{Timeout: "2m", Interval: "1m", Attempts: 1}
		if err := validateDefaults(d); err == nil {
			t.Errorf("expected error, got none")
		}
	})
}

func TestBuildChecks(t *testing.T) {
	defaults := DefaultsConfig{Timeout: "10s", Interval: "1m", Attempts: 3}

	t.Run("applies defaults when unset", func(t *testing.T) {
		checks, err := buildChecks([]CheckConfig{
			{Name: "a", Target: "target", Command: "cmd"},
		}, defaults)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(checks) != 1 {
			t.Fatalf("got %d checks, want 1", len(checks))
		}

		if checks[0].Timeout != 10*time.Second {
			t.Errorf("Timeout = %v, want 10s", checks[0].Timeout)
		}
		if checks[0].Interval != time.Minute {
			t.Errorf("Interval = %v, want 1m", checks[0].Interval)
		}
		if checks[0].Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", checks[0].Attempts)
		}
	})

	t.Run("per-check overrides defaults", func(t *testing.T) {
		checks, err := buildChecks([]CheckConfig{
			{Name: "a", Target: "target", Command: "cmd", Timeout: "5s", Interval: "30s", Attempts: 1},
		}, defaults)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checks[0].Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want 5s", checks[0].Timeout)
		}
		if checks[0].Interval != 30*time.Second {
			t.Errorf("Interval = %v, want 30s", checks[0].Interval)
		}
		if checks[0].Attempts != 1 {
			t.Errorf("Attempts = %d, want 1", checks[0].Attempts)
		}
	})

	t.Run("invalid timeout names the check", func(t *testing.T) {
		_, err := buildChecks([]CheckConfig{
			{Name: "bad-check", Command: "cmd", Timeout: "notaduration"},
		}, defaults)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("invalid interval surfaces error", func(t *testing.T) {
		_, err := buildChecks([]CheckConfig{
			{Name: "bad-check", Command: "cmd", Interval: "notaduration"},
		}, defaults)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("propagates validation failure", func(t *testing.T) {
		_, err := buildChecks([]CheckConfig{
			{Name: "", Command: "cmd"},
		}, defaults)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("duplicate names error", func(t *testing.T) {
		_, err := buildChecks([]CheckConfig{
			{Name: "a", Target: "target", Command: "cmd"},
			{Name: "a", Target: "target", Command: "cmd"},
		}, defaults)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("uses built-in critical reminder interval", func(t *testing.T) {
		checks, err := buildChecks([]CheckConfig{
			{Name: "a", Target: "target", Command: "cmd"},
		}, defaults)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checks[0].CriticalReminderInterval != 10*time.Minute {
			t.Errorf(
				"CriticalReminderInterval = %v, want 10m",
				checks[0].CriticalReminderInterval,
			)
		}
	})

	t.Run("per-check critical reminder interval overrides default", func(t *testing.T) {
		defaults := DefaultsConfig{
			Timeout:                  "10s",
			Interval:                 "1m",
			Attempts:                 3,
			CriticalReminderInterval: "10m",
		}

		checks, err := buildChecks([]CheckConfig{
			{
				Name:                     "a",
				Target:                   "target",
				Command:                  "cmd",
				CriticalReminderInterval: "5m",
			},
		}, defaults)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checks[0].CriticalReminderInterval != 5*time.Minute {
			t.Errorf(
				"CriticalReminderInterval = %v, want 5m",
				checks[0].CriticalReminderInterval,
			)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("unable to write %q: %v", path, err)
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file errors", func(t *testing.T) {
		var cfg Config
		err := loadConfigFile(filepath.Join(dir, "missing.yml"), &cfg)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("unknown fields rejected", func(t *testing.T) {
		path := filepath.Join(dir, "unknown.yml")
		writeFile(t, path, "checks:\n  - name: a\n    command: cmd\n    bogus_field: true\n")

		var cfg ImportedConfig
		if err := loadConfigFile(path, &cfg); err == nil {
			t.Fatalf("expected error for unknown field, got none")
		}
	})

	t.Run("valid file decodes", func(t *testing.T) {
		path := filepath.Join(dir, "valid.yml")
		writeFile(t, path, "checks:\n  - name: a\n    command: cmd\n")

		var cfg ImportedConfig
		if err := loadConfigFile(path, &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.Checks) != 1 || cfg.Checks[0].Name != "a" {
			t.Errorf("got %+v", cfg.Checks)
		}
	})
}

func TestLoadImportedChecks(t *testing.T) {
	dir := t.TempDir()
	defaults := DefaultsConfig{Timeout: "10s", Interval: "1m", Attempts: 3}

	t.Run("no checks errors", func(t *testing.T) {
		path := filepath.Join(dir, "empty.yml")
		writeFile(t, path, "checks: []\n")

		_, err := loadImportedChecks(path, defaults)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("valid checks", func(t *testing.T) {
		path := filepath.Join(dir, "imported.yml")
		writeFile(t, path, "checks:\n  - name: imported-check\n    target: target\n    command: cmd\n")

		checks, err := loadImportedChecks(path, defaults)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(checks) != 1 || checks[0].Name != "imported-check" {
			t.Errorf("got %+v", checks)
		}
	})
}

func TestResolveImport(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.yml")
	writeFile(t, mainFile, "startup_spread: 0\n")

	t.Run("single file path", func(t *testing.T) {
		other := filepath.Join(dir, "other.yml")
		writeFile(t, other, "checks: []\n")

		files, err := resolveImport(other, mainFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0] != other {
			t.Errorf("got %v, want [%v]", files, other)
		}
	})

	t.Run("directory lists yml files and skips main file", func(t *testing.T) {
		subDir := filepath.Join(dir, "subdir")
		if err := os.Mkdir(subDir, 0o755); err != nil {
			t.Fatalf("unable to create dir: %v", err)
		}

		writeFile(t, filepath.Join(subDir, "a.yml"), "checks: []\n")
		writeFile(t, filepath.Join(subDir, "b.yaml"), "checks: []\n")
		writeFile(t, filepath.Join(subDir, "ignore.txt"), "not yaml\n")

		mainInSubdir := filepath.Join(subDir, "main.yml")
		writeFile(t, mainInSubdir, "startup_spread: 0\n")

		files, err := resolveImport(subDir, mainInSubdir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 2 {
			t.Fatalf("got %d files, want 2: %v", len(files), files)
		}

		for _, f := range files {
			if filepath.Base(f) == "main.yml" {
				t.Errorf("main file should have been skipped: %v", files)
			}
		}
	})

	t.Run("nonexistent path errors", func(t *testing.T) {
		_, err := resolveImport(filepath.Join(dir, "does-not-exist"), mainFile)
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})
}

func TestFindConfigFile(t *testing.T) {
	t.Run("finds config relative to working directory", func(t *testing.T) {
		path, err := findConfigFile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if path != filepath.Join("config", "greybeard.yml") {
			t.Errorf("path = %q, want %q", path, filepath.Join("config", "greybeard.yml"))
		}
	})

	t.Run("errors when nothing found", func(t *testing.T) {
		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("unable to get working directory: %v", err)
		}

		emptyDir := t.TempDir()
		homeDir := t.TempDir()

		if err := os.Chdir(emptyDir); err != nil {
			t.Fatalf("unable to chdir: %v", err)
		}
		defer func() {
			_ = os.Chdir(originalWD)
		}()

		t.Setenv("HOME", homeDir)

		if _, err := findConfigFile(); err == nil {
			t.Errorf("expected error, got none")
		}
	})
}

func TestBuildActions(t *testing.T) {
	t.Run("valid action", func(t *testing.T) {
		actions, err := buildActions([]ActionConfig{
			{
				Name:    "notify-admin",
				Command: "./notify.sh",
				Timeout: "10s",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		action := actions["notify-admin"]

		if action.Command != "./notify.sh" {
			t.Errorf("Command = %q, want %q", action.Command, "./notify.sh")
		}

		if action.Timeout != 10*time.Second {
			t.Errorf("Timeout = %v, want 10s", action.Timeout)
		}
	})

	t.Run("duplicate action names", func(t *testing.T) {
		_, err := buildActions([]ActionConfig{
			{Name: "notify", Command: "a", Timeout: "10s"},
			{Name: "notify", Command: "b", Timeout: "10s"},
		})

		if err == nil {
			t.Fatal("expected error, got none")
		}
	})

	t.Run("missing timeout", func(t *testing.T) {
		_, err := buildActions([]ActionConfig{
			{Name: "notify", Command: "./notify.sh"},
		})

		if err == nil {
			t.Fatal("expected error, got none")
		}
	})

	t.Run("invalid timeout", func(t *testing.T) {
		_, err := buildActions([]ActionConfig{
			{Name: "notify", Command: "./notify.sh", Timeout: "forever"},
		})

		if err == nil {
			t.Fatal("expected error, got none")
		}
	})
}

func TestValidateActionReferences(t *testing.T) {
	actions := map[string]Action{
		"notify": {Name: "notify"},
	}

	t.Run("defined action", func(t *testing.T) {
		checks := []Check{
			{
				Name: "web",
				Actions: CheckActions{
					Critical: []string{"notify"},
				},
			},
		}

		if err := validateActionReferences(checks, actions); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("undefined action", func(t *testing.T) {
		checks := []Check{
			{
				Name: "web",
				Actions: CheckActions{
					Critical: []string{"does-not-exist"},
				},
			},
		}

		if err := validateActionReferences(checks, actions); err == nil {
			t.Fatal("expected error, got none")
		}
	})

	t.Run("missing target", func(t *testing.T) {
		check := validCheck()
		check.Target = ""
		if err := validateCheck(check); err == nil {
			t.Errorf("expected error, got none")
		}
	})
}
