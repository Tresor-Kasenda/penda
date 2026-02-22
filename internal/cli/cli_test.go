package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRoutesCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"routes"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "GET /metrics") {
		t.Fatalf("expected routes output, got %q", stdout.String())
	}
}

func TestRunDoctorCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Go:") {
		t.Fatalf("expected doctor output, got %q", stdout.String())
	}
}

func TestCmdNewCreatesProject(t *testing.T) {
	dir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"new", "myapp"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", code, stderr.String())
	}

	projectPath := filepath.Join(dir, "myapp")
	for _, fileName := range []string{"main.go", "go.mod", "README.md"} {
		if _, err := os.Stat(filepath.Join(projectPath, fileName)); err != nil {
			t.Fatalf("expected file %s to exist: %v", fileName, err)
		}
	}
}
