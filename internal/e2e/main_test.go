package e2e

import (
	"fmt"
	"os"
	"testing"
)

var worksetBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "workset-e2e-*")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmp)
	}

	worksetBin, err = buildWorksetBinary(tmp)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		cleanup()
		os.Exit(1)
	}

	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}
