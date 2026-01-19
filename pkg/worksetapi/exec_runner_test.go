package worksetapi

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRunExecCommandCreatesFileInRoot(t *testing.T) {
	root := t.TempDir()
	env := append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"WORKSET_TEST_TOUCH=touch.txt",
	)

	err := runExecCommand(context.Background(), root, []string{
		os.Args[0],
		"-test.run=TestExecHelperProcess",
		"--",
		"touch",
	}, env)
	if err != nil {
		t.Fatalf("run exec: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "touch.txt")); err != nil {
		t.Fatalf("expected touched file: %v", err)
	}
}

func TestRunExecCommandNonZeroExit(t *testing.T) {
	root := t.TempDir()
	env := append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	err := runExecCommand(context.Background(), root, []string{
		os.Args[0],
		"-test.run=TestExecHelperProcess",
		"--",
		"exit",
		"2",
	}, env)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunExecCommandMissingBinary(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing-bin")
	if err := runExecCommand(context.Background(), root, []string{missing}, os.Environ()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(0)
	}

	switch args[0] {
	case "touch":
		name := os.Getenv("WORKSET_TEST_TOUCH")
		if name != "" {
			_ = os.WriteFile(name, []byte("ok"), 0o600)
		}
		os.Exit(0)
	case "exit":
		if len(args) > 1 {
			if code, err := strconv.Atoi(args[1]); err == nil {
				os.Exit(code)
			}
		}
		os.Exit(0)
	default:
		os.Exit(0)
	}
}
