package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type RepoFileHoverRequest struct {
	WorkspaceID string `json:"workspaceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Line        int    `json:"line"`
	Character   int    `json:"character"`
}

type RepoFileHoverRange struct {
	StartLine      int `json:"startLine"`
	StartCharacter int `json:"startCharacter"`
	EndLine        int `json:"endLine"`
	EndCharacter   int `json:"endCharacter"`
}

type RepoFileHoverResponse struct {
	Supported         bool                `json:"supported"`
	Available         bool                `json:"available"`
	Found             bool                `json:"found"`
	Language          string              `json:"language,omitempty"`
	Provider          string              `json:"provider,omitempty"`
	Header            string              `json:"header,omitempty"`
	Documentation     string              `json:"documentation,omitempty"`
	DocumentationKind string              `json:"documentationKind,omitempty"`
	Source            string              `json:"source,omitempty"`
	Range             *RepoFileHoverRange `json:"range,omitempty"`
	UnavailableReason string              `json:"unavailableReason,omitempty"`
	InstallHint       string              `json:"installHint,omitempty"`
}

type repoFileDefinitionLocation struct {
	filePath       string
	startLine      int
	startCharacter int
	endLine        int
	endCharacter   int
}

type repoHoverBackend interface {
	Hover(ctx context.Context, request repoHoverLSPRequest) (RepoFileHoverResponse, error)
	Definition(ctx context.Context, request repoHoverLSPRequest) ([]repoFileDefinitionLocation, error)
	Close() error
	Alive() bool
}

type repoHoverRuntime struct {
	backendType string
	language    string
	provider    string
	languageID  string
	rootPath    string
	command     string
	args        []string
	installHint string
}

type repoHoverCandidate struct {
	backendType   string
	provider      string
	execName      string
	args          []string
	repoLocalBins []string
}

type repoHoverSpec struct {
	language    string
	extensions  []string
	languageIDs map[string]string
	rootMarkers []string
	candidates  []repoHoverCandidate
	installHint string
}

var hoverLookPath = exec.LookPath

var newRepoHoverBackend = func(ctx context.Context, runtime repoHoverRuntime) (repoHoverBackend, error) {
	switch runtime.backendType {
	case "", "lsp":
		return newLSPHoverBackend(ctx, runtime)
	case "tsserver":
		return newTSServerHoverBackend(ctx, runtime)
	default:
		return nil, fmt.Errorf("unsupported hover backend type %q", runtime.backendType)
	}
}

var repoHoverSpecs = []repoHoverSpec{
	{
		language:    "typescript",
		extensions:  []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"},
		languageIDs: map[string]string{".ts": "typescript", ".tsx": "typescriptreact", ".js": "javascript", ".jsx": "javascriptreact", ".mjs": "javascript", ".cjs": "javascript", ".mts": "typescript", ".cts": "typescript"},
		rootMarkers: []string{"tsconfig.json", "jsconfig.json", "package.json"},
		candidates: []repoHoverCandidate{
			{
				backendType:   "lsp",
				provider:      "vtsls",
				execName:      "vtsls",
				args:          []string{"--stdio"},
				repoLocalBins: []string{"node_modules/.bin/vtsls"},
			},
			{
				backendType:   "lsp",
				provider:      "typescript-language-server",
				execName:      "typescript-language-server",
				args:          []string{"--stdio"},
				repoLocalBins: []string{"node_modules/.bin/typescript-language-server"},
			},
			{
				backendType:   "tsserver",
				provider:      "tsserver",
				execName:      "tsserver",
				repoLocalBins: []string{"node_modules/.bin/tsserver", "node_modules/typescript/bin/tsserver"},
			},
		},
		installHint: "Install vtsls or typescript-language-server, or add a local TypeScript install that provides tsserver.",
	},
	{
		language:    "svelte",
		extensions:  []string{".svelte"},
		languageIDs: map[string]string{".svelte": "svelte"},
		rootMarkers: []string{"svelte.config.js", "svelte.config.cjs", "svelte.config.mjs", "svelte.config.ts", "package.json", "tsconfig.json", "jsconfig.json"},
		candidates: []repoHoverCandidate{
			{
				backendType:   "lsp",
				provider:      "svelteserver",
				execName:      "svelteserver",
				args:          []string{"--stdio"},
				repoLocalBins: []string{"node_modules/.bin/svelteserver"},
			},
		},
		installHint: "Install svelte-language-server in the repo or on PATH to enable Svelte hover.",
	},
	{
		language:    "go",
		extensions:  []string{".go"},
		languageIDs: map[string]string{".go": "go"},
		rootMarkers: []string{"go.work", "go.mod"},
		candidates: []repoHoverCandidate{
			{backendType: "lsp", provider: "gopls", execName: "gopls"},
		},
		installHint: "Install gopls on PATH to enable Go hover.",
	},
	{
		language:    "python",
		extensions:  []string{".py"},
		languageIDs: map[string]string{".py": "python"},
		rootMarkers: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"},
		candidates: []repoHoverCandidate{
			{
				backendType:   "lsp",
				provider:      "basedpyright",
				execName:      "basedpyright-langserver",
				args:          []string{"--stdio"},
				repoLocalBins: []string{".venv/bin/basedpyright-langserver", "venv/bin/basedpyright-langserver"},
			},
			{
				backendType:   "lsp",
				provider:      "pyright",
				execName:      "pyright-langserver",
				args:          []string{"--stdio"},
				repoLocalBins: []string{".venv/bin/pyright-langserver", "venv/bin/pyright-langserver"},
			},
			{
				backendType:   "lsp",
				provider:      "pylsp",
				execName:      "pylsp",
				repoLocalBins: []string{".venv/bin/pylsp", "venv/bin/pylsp"},
			},
		},
		installHint: "Install basedpyright-langserver, pyright-langserver, or pylsp to enable Python hover.",
	},
	{
		language:    "rust",
		extensions:  []string{".rs"},
		languageIDs: map[string]string{".rs": "rust"},
		rootMarkers: []string{"Cargo.toml"},
		candidates: []repoHoverCandidate{
			{backendType: "lsp", provider: "rust-analyzer", execName: "rust-analyzer"},
		},
		installHint: "Install rust-analyzer on PATH to enable Rust hover.",
	},
}

func (a *App) GetRepoFileHover(input RepoFileHoverRequest) (RepoFileHoverResponse, error) {
	ctx := a.appContext()

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	repoID := strings.TrimSpace(input.RepoID)
	if workspaceID == "" || repoID == "" {
		return RepoFileHoverResponse{}, errors.New("workspace and repo are required")
	}
	if strings.TrimSpace(input.Path) == "" {
		return RepoFileHoverResponse{}, errors.New("path is required")
	}
	if input.Line < 0 || input.Character < 0 {
		return RepoFileHoverResponse{}, errors.New("line and character must be non-negative")
	}

	root, err := a.resolveWorkspaceContentRoot(ctx, workspaceID, repoID)
	if err != nil {
		return RepoFileHoverResponse{}, err
	}

	resolvedPath, normalizedPath, err := resolveRepoFilePath(root.path, input.Path)
	if err != nil {
		return RepoFileHoverResponse{}, err
	}

	hoverRuntime, supported, err := resolveRepoHoverRuntime(root.path, resolvedPath)
	if err != nil {
		return RepoFileHoverResponse{}, err
	}
	if !supported {
		return RepoFileHoverResponse{}, nil
	}

	response := RepoFileHoverResponse{
		Supported: true,
		Language:  hoverRuntime.language,
		Provider:  hoverRuntime.provider,
	}

	if hoverRuntime.command == "" {
		response.InstallHint = hoverRuntime.installHint
		response.UnavailableReason = fmt.Sprintf("%s is not installed or not available in this repo.", hoverRuntime.provider)
		return response, nil
	}

	client, err := a.repoHoverClient(ctx, hoverRuntime)
	if err != nil {
		response.InstallHint = hoverRuntime.installHint
		response.UnavailableReason = err.Error()
		return response, nil
	}

	hoverResponse, err := client.Hover(ctx, repoHoverLSPRequest{
		filePath:   resolvedPath,
		path:       normalizedPath,
		content:    input.Content,
		line:       input.Line,
		character:  input.Character,
		languageID: hoverRuntime.languageID,
		language:   hoverRuntime.language,
		provider:   hoverRuntime.provider,
	})
	if err != nil {
		a.invalidateRepoHoverClient(hoverRuntime)
		response.Available = true
		return response, nil
	}
	return hoverResponse, nil
}

func resolveRepoHoverRuntime(repoPath, resolvedPath string) (repoHoverRuntime, bool, error) {
	spec, found := repoHoverSpecForPath(resolvedPath)
	if !found {
		return repoHoverRuntime{}, false, nil
	}

	rootPath := findRepoHoverRoot(repoPath, filepath.Dir(resolvedPath), spec.rootMarkers)
	command, backendType, provider, args := resolveRepoHoverCommand(rootPath, spec.candidates)
	return repoHoverRuntime{
		backendType: backendType,
		language:    spec.language,
		languageID:  repoHoverLanguageID(spec, resolvedPath),
		provider:    provider,
		rootPath:    rootPath,
		command:     command,
		args:        args,
		installHint: spec.installHint,
	}, true, nil
}

func repoHoverSpecForPath(filePath string) (repoHoverSpec, bool) {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, spec := range repoHoverSpecs {
		for _, candidate := range spec.extensions {
			if ext == candidate {
				return spec, true
			}
		}
	}
	return repoHoverSpec{}, false
}

func repoHoverLanguageID(spec repoHoverSpec, filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if languageID, ok := spec.languageIDs[ext]; ok && languageID != "" {
		return languageID
	}
	return spec.language
}

func findRepoHoverRoot(repoPath, startDir string, markers []string) string {
	repoPath = filepath.Clean(repoPath)
	current := filepath.Clean(startDir)
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current
			}
		}
		if current == repoPath {
			return repoPath
		}
		parent := filepath.Dir(current)
		if parent == current || !strings.HasPrefix(parent, repoPath) {
			return repoPath
		}
		current = parent
	}
}

func resolveRepoHoverCommand(rootPath string, candidates []repoHoverCandidate) (command, backendType, provider string, args []string) {
	for _, candidate := range candidates {
		for _, relativePath := range candidate.repoLocalBins {
			for _, localPath := range expandRepoLocalExecutableCandidates(rootPath, relativePath) {
				if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
					return localPath, candidate.backendType, candidate.provider, candidate.args
				}
			}
		}
		if candidate.execName == "" {
			continue
		}
		if resolvedPath, err := hoverLookPath(candidate.execName); err == nil {
			return resolvedPath, candidate.backendType, candidate.provider, candidate.args
		}
	}

	if len(candidates) == 0 {
		return "", "", "", nil
	}
	return "", candidates[0].backendType, candidates[0].provider, nil
}

func expandRepoLocalExecutableCandidates(rootPath, relativePath string) []string {
	base := filepath.Join(rootPath, filepath.FromSlash(relativePath))
	if runtime.GOOS != "windows" {
		return []string{base}
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".exe" || ext == ".cmd" || ext == ".bat" {
		return []string{base}
	}
	return []string{base + ".cmd", base + ".exe", base + ".bat", base}
}

func repoHoverClientCacheKey(runtime repoHoverRuntime) string {
	return runtime.language + "\x00" + runtime.provider + "\x00" + filepath.Clean(runtime.rootPath)
}

func (a *App) repoHoverClient(ctx context.Context, runtime repoHoverRuntime) (repoHoverBackend, error) {
	key := repoHoverClientCacheKey(runtime)

	a.repoHoverMu.Lock()
	cached := a.repoHoverClients[key]
	if cached != nil && cached.Alive() {
		a.repoHoverMu.Unlock()
		return cached, nil
	}
	if cached != nil {
		_ = cached.Close()
		delete(a.repoHoverClients, key)
	}
	a.repoHoverMu.Unlock()

	client, err := newRepoHoverBackend(ctx, runtime)
	if err != nil {
		return nil, err
	}

	a.repoHoverMu.Lock()
	defer a.repoHoverMu.Unlock()
	if cached = a.repoHoverClients[key]; cached != nil && cached.Alive() {
		_ = client.Close()
		return cached, nil
	}
	a.repoHoverClients[key] = client
	return client, nil
}

func (a *App) invalidateRepoHoverClient(runtime repoHoverRuntime) {
	key := repoHoverClientCacheKey(runtime)
	a.repoHoverMu.Lock()
	defer a.repoHoverMu.Unlock()
	if client := a.repoHoverClients[key]; client != nil {
		_ = client.Close()
		delete(a.repoHoverClients, key)
	}
}
