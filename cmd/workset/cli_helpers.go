package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strantalis/workset/internal/config"
	"github.com/strantalis/workset/internal/output"
	"github.com/urfave/cli/v3"
)

func loadGlobal(cmd *cli.Command) (config.GlobalConfig, string, error) {
	path := ""
	if cmd != nil {
		path = cmd.String("config")
	}
	cfg, info, err := config.LoadGlobalWithInfo(path)
	if err != nil {
		return config.GlobalConfig{}, "", err
	}
	if verboseEnabled(cmd) {
		printConfigLoadInfo(cmd, path, info)
	}
	return cfg, info.Path, nil
}

func printWorkspaceCreated(w io.Writer, info output.WorkspaceCreated, asJSON bool, plain bool) error {
	if asJSON {
		return output.WriteJSON(w, info)
	}
	styles := output.NewStyles(w, plain)
	return output.PrintWorkspaceCreated(w, info, styles)
}

func commandWriter(cmd *cli.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	root := cmd.Root()
	if root != nil && root.Writer != nil {
		return root.Writer
	}
	if cmd.Writer != nil {
		return cmd.Writer
	}
	return os.Stdout
}

func commandErrWriter(cmd *cli.Command) io.Writer {
	if cmd == nil {
		return os.Stderr
	}
	root := cmd.Root()
	if root != nil && root.ErrWriter != nil {
		return root.ErrWriter
	}
	if cmd.ErrWriter != nil {
		return cmd.ErrWriter
	}
	return os.Stderr
}

func enableSuggestions(cmd *cli.Command) {
	if cmd == nil {
		return
	}
	cmd.Suggest = true
	for _, sub := range cmd.Commands {
		enableSuggestions(sub)
	}
}

func workspaceFlag(required bool) cli.Flag {
	usage := "Workspace name or path"
	if required {
		usage = "Workspace name or path (required unless defaults.workspace set)"
	}
	return &cli.StringFlag{
		Name:    "workspace",
		Aliases: []string{"w"},
		Usage:   usage,
	}
}

func outputFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "Output JSON",
		},
		&cli.BoolFlag{
			Name:  "plain",
			Usage: "Disable styling",
		},
	}
}

func appendOutputFlags(flags []cli.Flag) []cli.Flag {
	return append(flags, outputFlags()...)
}

type outputMode struct {
	JSON  bool
	Plain bool
}

func outputModeFromContext(cmd *cli.Command) outputMode {
	jsonFlag := boolFlagWithArgs(cmd, "json")
	plainFlag := boolFlagWithArgs(cmd, "plain")
	if jsonFlag {
		plainFlag = true
	}
	return outputMode{JSON: jsonFlag, Plain: plainFlag}
}

func verboseEnabled(cmd *cli.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.Bool("verbose") {
		return true
	}
	if root := cmd.Root(); root != nil && root != cmd {
		if root.Bool("verbose") {
			return true
		}
	}
	value, ok := boolFromArgs(cmd.Args().Slice(), "verbose")
	return ok && value
}

func printConfigLoadInfo(cmd *cli.Command, override string, info config.GlobalConfigLoadInfo) {
	w := commandErrWriter(cmd)
	if override != "" {
		_, _ = fmt.Fprintf(w, "config: using override %s\n", info.Path)
	} else if info.Path != "" {
		_, _ = fmt.Fprintf(w, "config: using %s\n", info.Path)
	}
	if info.Migrated && info.LegacyPath != "" {
		_, _ = fmt.Fprintf(w, "config: migrated %s -> %s\n", info.LegacyPath, info.Path)
	} else if info.UsedLegacy && info.LegacyPath != "" {
		_, _ = fmt.Fprintf(w, "config: using legacy %s\n", info.LegacyPath)
	}
	if !info.Exists {
		_, _ = fmt.Fprintln(w, "config: no config file found; using defaults")
	}
}

func usageError(ctx context.Context, cmd *cli.Command, message string) error {
	mode := outputModeFromContext(cmd)
	if mode.JSON {
		return cli.Exit(message, 1)
	}
	if message != "" {
		if _, err := fmt.Fprintln(commandErrWriter(cmd), message); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(commandErrWriter(cmd)); err != nil {
			return err
		}
	}
	if err := printCommandHelp(ctx, cmd); err != nil {
		return err
	}
	return cli.Exit("", 1)
}

func printCommandHelp(_ context.Context, cmd *cli.Command) error {
	if cmd == nil {
		return nil
	}
	root := cmd.Root()
	if root == nil || root == cmd {
		return cli.ShowRootCommandHelp(cmd)
	}
	tmpl := cmd.CustomHelpTemplate
	if tmpl == "" {
		if len(cmd.Commands) == 0 {
			tmpl = cli.CommandHelpTemplate
		} else {
			tmpl = cli.SubcommandHelpTemplate
		}
	}
	cli.HelpPrinter(commandWriter(cmd), tmpl, cmd)
	return nil
}

func boolFlagWithArgs(cmd *cli.Command, name string) bool {
	if cmd.Bool(name) {
		return true
	}
	if value, ok := boolFromArgs(cmd.Args().Slice(), name); ok {
		return value
	}
	return false
}

type flagSpec struct {
	TakesValue bool
}

func normalizeArgs(root *cli.Command, args []string) []string {
	if root == nil || len(args) == 0 {
		return args
	}

	cmd := root
	i := 1
	for i < len(args) {
		token := args[i]
		if token == "--" || strings.HasPrefix(token, "-") {
			break
		}
		next := findSubcommand(cmd, token)
		if next == nil {
			break
		}
		cmd = next
		i++
	}

	prefix := append([]string{}, args[:i]...)
	flags := make([]string, 0)
	rest := make([]string, 0)

	for j := i; j < len(args); j++ {
		token := args[j]
		if token == "--" {
			rest = append(rest, args[j:]...)
			break
		}
		if spec, ok := interspersedFlag(token); ok {
			flags = append(flags, token)
			if spec.TakesValue && !strings.Contains(token, "=") && j+1 < len(args) {
				flags = append(flags, args[j+1])
				j++
			}
			continue
		}
		rest = append(rest, token)
	}

	normalized := append(prefix, flags...)
	normalized = append(normalized, rest...)
	return normalized
}

func findSubcommand(cmd *cli.Command, name string) *cli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Commands {
		if sub.Name == name {
			return sub
		}
		for _, alias := range sub.Aliases {
			if alias == name {
				return sub
			}
		}
	}
	return nil
}

func interspersedFlag(token string) (flagSpec, bool) {
	switch token {
	case "-w", "--workspace":
		return flagSpec{TakesValue: true}, true
	case "--json", "--plain":
		return flagSpec{TakesValue: false}, true
	case "--config":
		return flagSpec{TakesValue: true}, true
	case "--verbose":
		return flagSpec{TakesValue: false}, true
	case "--path", "--group", "--repo":
		return flagSpec{TakesValue: true}, true
	case "--backend", "--name":
		return flagSpec{TakesValue: true}, true
	case "--interactive", "--pty":
		return flagSpec{TakesValue: false}, true
	case "--yes":
		return flagSpec{TakesValue: false}, true
	}
	switch {
	case strings.HasPrefix(token, "--workspace="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--config="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--verbose="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--json="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--plain="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--path="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--group="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--repo="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--backend="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--name="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--interactive="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--pty="):
		return flagSpec{TakesValue: false}, true
	case strings.HasPrefix(token, "--yes="):
		return flagSpec{TakesValue: false}, true
	}
	return flagSpec{}, false
}

func boolFromArgs(args []string, name string) (bool, bool) {
	long := "--" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == long {
			return true, true
		}
		if strings.HasPrefix(arg, long+"=") {
			value := strings.TrimPrefix(arg, long+"=")
			if value == "" {
				return true, true
			}
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return true, true
			}
			return parsed, true
		}
	}
	return false, false
}

func workspaceFromArgs(cmd *cli.Command) string {
	args := cmd.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-w" || arg == "--workspace" {
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
			return ""
		}
		if strings.HasPrefix(arg, "--workspace=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--workspace="))
		}
	}
	return ""
}

func confirmPrompt(r io.Reader, w io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(w, prompt); err != nil {
		return false, err
	}
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return false, nil
	}
	line = strings.ToLower(line)
	return line == "y" || line == "yes", nil
}

func resolveWorkspaceTarget(arg string, cfg *config.GlobalConfig) (string, string, error) {
	target := strings.TrimSpace(arg)
	if target == "" {
		target = strings.TrimSpace(cfg.Defaults.Workspace)
	}
	if target == "" {
		return "", "", fmt.Errorf("workspace required: pass -w <name|path> or set defaults.workspace (example: workset rm -w <name> --delete)")
	}
	if ref, ok := cfg.Workspaces[target]; ok {
		return target, ref.Path, nil
	}
	if !filepath.IsAbs(target) && cfg.Defaults.WorkspaceRoot != "" {
		candidate := filepath.Join(cfg.Defaults.WorkspaceRoot, target)
		if _, err := os.Stat(candidate); err == nil {
			return target, candidate, nil
		}
	}
	if filepath.IsAbs(target) {
		name := workspaceNameByPath(cfg, target)
		return name, target, nil
	}
	return "", "", fmt.Errorf("workspace not found: %q (use a registered name, an absolute path, or a path under defaults.workspace_root)", target)
}

func workspaceNameByPath(cfg *config.GlobalConfig, path string) string {
	clean := filepath.Clean(path)
	for name, ref := range cfg.Workspaces {
		if filepath.Clean(ref.Path) == clean {
			return name
		}
	}
	return ""
}
