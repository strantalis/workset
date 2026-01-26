package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/strantalis/workset/pkg/termemu"
)

func main() {
	var inputPath string
	var outputPath string
	var cols int
	var rows int
	var limit int
	flag.StringVar(&inputPath, "input", "", "path to raw PTY capture")
	flag.StringVar(&outputPath, "output", "", "path to write ANSI snapshot (stdout if empty)")
	flag.IntVar(&cols, "cols", 120, "terminal columns")
	flag.IntVar(&rows, "rows", 40, "terminal rows")
	flag.IntVar(&limit, "limit", 0, "limit bytes from input (0 = all)")
	flag.Parse()

	if inputPath == "" {
		exitErr(fmt.Errorf("input path required"))
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		exitErr(err)
	}

	emu := termemu.New(cols, rows)
	if limit > 0 && limit < len(data) {
		data = data[:limit]
	}
	emu.Write(data)
	snapshot := emu.SnapshotANSI()

	var out io.Writer = os.Stdout
	if outputPath != "" {
		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			exitErr(err)
		}
		defer func() {
			_ = file.Close()
		}()
		out = file
	}
	if _, err := io.WriteString(out, snapshot); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
