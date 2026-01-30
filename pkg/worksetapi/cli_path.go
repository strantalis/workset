package worksetapi

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var defaultCLICandidateDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/opt/local/bin",
	"/run/current-system/sw/bin",
	"/nix/var/nix/profiles/default/bin",
	"/snap/bin",
	"/var/lib/snapd/snap/bin",
}

func resolveCLIPath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if hasPathSeparator(command) {
		if isExecutableCandidate(command) {
			return filepath.Clean(command)
		}
		return ""
	}
	if path, err := exec.LookPath(command); err == nil {
		return path
	}
	for _, candidate := range cliCandidates(command) {
		if isExecutableCandidate(candidate) {
			return candidate
		}
	}
	return ""
}

func cliCandidates(command string) []string {
	dirs := cliCandidateDirs()
	candidates := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		for _, name := range cliExecutableNames(command) {
			candidates = append(candidates, filepath.Join(dir, name))
		}
	}
	return candidates
}

func cliCandidateDirs() []string {
	seen := map[string]struct{}{}
	dirs := make([]string, 0, len(defaultCLICandidateDirs)+8)
	addDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			return
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}
	}

	for _, dir := range defaultCLICandidateDirs {
		addDir(dir)
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		addDir(filepath.Join(home, ".nix-profile", "bin"))
		addDir(filepath.Join(home, ".local", "bin"))
		addDir(filepath.Join(home, ".asdf", "shims"))
		addDir(filepath.Join(home, ".volta", "bin"))
		addDir(filepath.Join(home, ".npm-global", "bin"))
		addDir(filepath.Join(home, ".local", "share", "pnpm"))
		addDir(filepath.Join(home, "Library", "pnpm"))
		addDir(filepath.Join(home, ".bun", "bin"))
		if nvmBin := nvmDefaultBin(home); nvmBin != "" {
			addDir(nvmBin)
		}
		xdgState := os.Getenv("XDG_STATE_HOME")
		if xdgState == "" {
			xdgState = filepath.Join(home, ".local", "state")
		}
		addDir(filepath.Join(xdgState, "nix", "profiles", "profile", "bin"))
	}

	if path := os.Getenv("NVM_BIN"); path != "" {
		addDir(path)
	}
	if path := os.Getenv("VOLTA_HOME"); path != "" {
		addDir(filepath.Join(path, "bin"))
	}
	if path := os.Getenv("PNPM_HOME"); path != "" {
		addDir(path)
	}
	if path := os.Getenv("BUN_INSTALL"); path != "" {
		addDir(filepath.Join(path, "bin"))
	}
	for _, key := range []string{"NPM_CONFIG_PREFIX", "npm_config_prefix"} {
		if prefix := os.Getenv(key); prefix != "" {
			addDir(npmPrefixBin(prefix))
		}
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			addDir(filepath.Join(appData, "npm"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			addDir(filepath.Join(localAppData, "npm"))
		}
	}

	return dirs
}

func cliExecutableNames(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{command + ".cmd", command + ".exe", command}
	}
	return []string{command}
}

func hasPathSeparator(value string) bool {
	if strings.ContainsRune(value, os.PathSeparator) {
		return true
	}
	if os.PathSeparator != '/' && strings.Contains(value, "/") {
		return true
	}
	return false
}

func isExecutableCandidate(path string) bool {
	if runtime.GOOS == "windows" {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	return isExecutableFile(path)
}

func npmPrefixBin(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	prefix = filepath.Clean(prefix)
	if runtime.GOOS == "windows" {
		return prefix
	}
	return filepath.Join(prefix, "bin")
}

func nvmDefaultBin(home string) string {
	nvmDir := os.Getenv("NVM_DIR")
	if nvmDir == "" && home != "" {
		nvmDir = filepath.Join(home, ".nvm")
	}
	if nvmDir == "" {
		return ""
	}
	aliasPath := filepath.Join(nvmDir, "alias", "default")
	if data, err := os.ReadFile(aliasPath); err == nil {
		version := strings.TrimSpace(string(data))
		if version != "" {
			if bin := filepath.Join(nvmDir, "versions", "node", version, "bin"); dirExists(bin) {
				return bin
			}
		}
	}
	matches, err := filepath.Glob(filepath.Join(nvmDir, "versions", "node", "*", "bin"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	for i := len(matches) - 1; i >= 0; i-- {
		if dirExists(matches[i]) {
			return matches[i]
		}
	}
	return ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
