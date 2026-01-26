package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/strantalis/workset/pkg/sessiond"
	"github.com/strantalis/workset/pkg/worksetapi"
)

var buildInfo = "dev"

func main() {
	if exe, err := os.Executable(); err == nil {
		hash := sha256.Sum256([]byte(buildInfo))
		_, _ = fmt.Fprintf(
			os.Stderr,
			"sessiond: exe=%s WORKSET_SESSIOND_PATH=%s build=%s\n",
			exe,
			os.Getenv("WORKSET_SESSIOND_PATH"),
			hex.EncodeToString(hash[:6]),
		)
	}
	var socketPath string
	var idleTimeout string
	var recordDir string
	var recordPty bool
	flag.StringVar(&socketPath, "socket", "", "path to sessiond socket")
	flag.StringVar(&idleTimeout, "idle-timeout", "", "idle timeout (e.g. 30m, 0 to disable)")
	flag.StringVar(&recordDir, "record-dir", "", "directory to record raw PTY output")
	flag.BoolVar(&recordPty, "record-pty", false, "record raw PTY output to disk")
	flag.Parse()

	opts := sessiond.DefaultOptions()
	if socketPath != "" {
		opts.SocketPath = socketPath
	}
	if recordDir == "" {
		recordDir = os.Getenv("WORKSET_SESSIOND_RECORD_DIR")
	}
	if recordDir != "" {
		opts.RecordDir = recordDir
	}
	if !recordPty {
		recordPty = envTruthy(os.Getenv("WORKSET_SESSIOND_RECORD_PTY"))
	}
	opts.RecordPty = recordPty

	cfgIdle := loadIdleTimeout()
	if idleTimeout != "" {
		cfgIdle = idleTimeout
	}
	if cfgIdle != "" {
		if cfgIdle == "0" || cfgIdle == "off" || cfgIdle == "disabled" || cfgIdle == "false" {
			opts.IdleTimeout = 0
		} else if parsed, err := time.ParseDuration(cfgIdle); err == nil {
			opts.IdleTimeout = parsed
		}
	}

	server := sessiond.NewServer(opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.SetShutdown(cancel)

	if err := server.Listen(ctx); err != nil {
		logFatal(err)
	}
}

func loadIdleTimeout() string {
	svc := worksetapi.NewService(worksetapi.Options{})
	cfg, _, err := svc.GetConfig(context.Background())
	if err != nil {
		return ""
	}
	return cfg.Defaults.TerminalIdleTimeout
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func logFatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
