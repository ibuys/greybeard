package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFlag(t *testing.T) {
	cmd := exec.Command(
		"go", "run", ".",
		"--validate",
		"-c", "config/greybeard.yml",
	)

	cmd.Env = append(os.Environ(), "GREYBEARD_DATABASE_URL=")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if !strings.Contains(string(output), "OK - configuration valid:") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestValidateFlagInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yml")
	writeFile(t, path, "checks: [\n")

	cmd := exec.Command(
		"go", "run", ".",
		"--validate",
		"-c", path,
	)

	cmd.Env = append(os.Environ(), "GREYBEARD_DATABASE_URL=")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected validation to fail, got: %s", output)
	}

	if !strings.Contains(string(output), "unable to parse config") {
		t.Errorf("unexpected output: %s", output)
	}

	if !strings.Contains(string(output), "UNKNOWN") {
		t.Errorf("unexpected output: %s", output)
	}
}


func TestValidateFlagInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yml")
	
	badvalues := `startup_spread: 0s

defaults:
  timeout: 10s
  interval: 1m
  attempts: 3

checks:
  - name: test-check
    target: localhost
    command: /bin/true
    timeout: 90s`
	
	
	writeFile(t, path, badvalues)

	cmd := exec.Command(
		"go", "run", ".",
		"--validate",
		"-c", path,
	)

	cmd.Env = append(os.Environ(), "GREYBEARD_DATABASE_URL=")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected validation to fail, got: %s", output)
	}

	if !strings.Contains(string(output), "check timeout must be smaller than interval") {
		t.Errorf("unexpected output: %s", output)
	}

	if !strings.Contains(string(output), "UNKNOWN") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestValidateFlagInvalidImportedConfig(t *testing.T) {

    dir := t.TempDir()
    checksDir := filepath.Join(dir, "checks")
    
    err := os.Mkdir(checksDir, 0o755)
    if err != nil {
        t.Fatalf("unable to create checks directory: %v", err)
    }
    
    mainconfig := `startup_spread: 0

defaults:
    interval: 1m
    timeout: 45s
    attempts: 3
    critical_reminder_interval: 15m
imports:
    - checks

actions:
    - name: capture-json
      command: ./test_actions/capture-json.sh
      timeout: 5s`
      
    brokenconfig := "checks: [\n"

	configpath := filepath.Join(dir, "greybeard.yml")
	brokenpath := filepath.Join(checksDir, "broken.yml")
	
	writeFile(t, configpath, mainconfig)
	writeFile(t, brokenpath, brokenconfig)

	cmd := exec.Command(
		"go", "run", ".",
		"--validate",
		"-c", configpath,
	)

	cmd.Env = append(os.Environ(), "GREYBEARD_DATABASE_URL=")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected validation to fail, got: %s", output)
	}

	if !strings.Contains(string(output), "unable to parse config") {
		t.Errorf("unexpected output: %s", output)
	}

	if !strings.Contains(string(output), "UNKNOWN") {
		t.Errorf("unexpected output: %s", output)
	}
}
