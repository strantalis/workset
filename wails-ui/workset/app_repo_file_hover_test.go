package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubRepoHoverBackend struct {
	hoverCalls      int
	definitionCalls int
	response        RepoFileHoverResponse
	definitions     []repoFileDefinitionLocation
}

func (s *stubRepoHoverBackend) Hover(_ context.Context, _ repoHoverLSPRequest) (RepoFileHoverResponse, error) {
	s.hoverCalls++
	return s.response, nil
}

func (s *stubRepoHoverBackend) Definition(_ context.Context, _ repoHoverLSPRequest) ([]repoFileDefinitionLocation, error) {
	s.definitionCalls++
	return s.definitions, nil
}

func (s *stubRepoHoverBackend) Close() error { return nil }
func (s *stubRepoHoverBackend) Alive() bool  { return true }

func TestResolveRepoHoverRuntimeUsesRepoLocalTypescriptServer(t *testing.T) {
	repoPath := t.TempDir()
	srcPath := filepath.Join(repoPath, "src")
	binPath := filepath.Join(repoPath, "node_modules", ".bin")
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binPath, "vtsls"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write vtsls stub: %v", err)
	}

	runtime, supported, err := resolveRepoHoverRuntime(
		repoPath,
		filepath.Join(repoPath, "src", "example.ts"),
	)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected TypeScript file to be supported")
	}
	if runtime.provider != "vtsls" {
		t.Fatalf("expected vtsls provider, got %q", runtime.provider)
	}
	if runtime.command == "" || filepath.Base(runtime.command) != "vtsls" {
		t.Fatalf("expected local vtsls command, got %q", runtime.command)
	}
	if runtime.rootPath != repoPath {
		t.Fatalf("expected repo root %q, got %q", repoPath, runtime.rootPath)
	}
}

func TestResolveRepoHoverRuntimeFallsBackToRepoLocalTSServer(t *testing.T) {
	repoPath := t.TempDir()
	srcPath := filepath.Join(repoPath, "src")
	typeScriptBinPath := filepath.Join(repoPath, "node_modules", "typescript", "bin")
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(typeScriptBinPath, 0o755); err != nil {
		t.Fatalf("mkdir typescript/bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(typeScriptBinPath, "tsserver"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write tsserver stub: %v", err)
	}

	runtime, supported, err := resolveRepoHoverRuntime(
		repoPath,
		filepath.Join(repoPath, "src", "example.ts"),
	)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected TypeScript file to be supported")
	}
	if runtime.provider != "tsserver" {
		t.Fatalf("expected tsserver provider, got %q", runtime.provider)
	}
	if runtime.backendType != "tsserver" {
		t.Fatalf("expected tsserver backend type, got %q", runtime.backendType)
	}
	if runtime.command == "" || filepath.Base(runtime.command) != "tsserver" {
		t.Fatalf("expected local tsserver command, got %q", runtime.command)
	}
}

func TestResolveRepoHoverRuntimeUsesRepoLocalSvelteServer(t *testing.T) {
	repoPath := t.TempDir()
	srcPath := filepath.Join(repoPath, "src")
	binPath := filepath.Join(repoPath, "node_modules", ".bin")
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "svelte.config.js"), []byte("export default {}\n"), 0o644); err != nil {
		t.Fatalf("write svelte config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binPath, "svelteserver"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write svelteserver stub: %v", err)
	}

	runtime, supported, err := resolveRepoHoverRuntime(
		repoPath,
		filepath.Join(repoPath, "src", "Example.svelte"),
	)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected Svelte file to be supported")
	}
	if runtime.provider != "svelteserver" {
		t.Fatalf("expected svelteserver provider, got %q", runtime.provider)
	}
	if runtime.languageID != "svelte" {
		t.Fatalf("expected svelte language id, got %q", runtime.languageID)
	}
}

func TestResolveRepoHoverRuntimeUsesTerraformLanguageServer(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("write main.tf: %v", err)
	}

	originalLookPath := hoverLookPath
	defer func() {
		hoverLookPath = originalLookPath
	}()
	hoverLookPath = func(name string) (string, error) {
		if name != "terraform-ls" {
			return "", errors.New("not found")
		}
		return filepath.Join(repoPath, "terraform-ls"), nil
	}

	runtime, supported, err := resolveRepoHoverRuntime(repoPath, filepath.Join(repoPath, "main.tf"))
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected Terraform file to be supported")
	}
	if runtime.provider != "terraform-ls" {
		t.Fatalf("expected terraform-ls provider, got %q", runtime.provider)
	}
	if runtime.languageID != "terraform" {
		t.Fatalf("expected terraform language id, got %q", runtime.languageID)
	}
	if runtime.rootPath != repoPath {
		t.Fatalf("expected repo root %q, got %q", repoPath, runtime.rootPath)
	}
}

func TestResolveRepoHoverRuntimeSupportsTerraformStackSuffixes(t *testing.T) {
	repoPath := t.TempDir()
	stackDir := filepath.Join(repoPath, "stacks", "prod")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack dir: %v", err)
	}
	stackFile := filepath.Join(stackDir, "app.tfcomponent.hcl")
	if err := os.WriteFile(stackFile, []byte("component \"app\" {}\n"), 0o644); err != nil {
		t.Fatalf("write stack file: %v", err)
	}

	runtime, supported, err := resolveRepoHoverRuntime(repoPath, stackFile)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected terraform stack file to be supported")
	}
	if runtime.languageID != "terraform-stack" {
		t.Fatalf("expected terraform-stack language id, got %q", runtime.languageID)
	}
	if runtime.rootPath != stackDir {
		t.Fatalf("expected nearest terraform root %q, got %q", stackDir, runtime.rootPath)
	}
}

func TestResolveRepoHoverRuntimeDoesNotSupportGenericHCL(t *testing.T) {
	repoPath := t.TempDir()
	filePath := filepath.Join(repoPath, "settings.hcl")
	if err := os.WriteFile(filePath, []byte("value = 1\n"), 0o644); err != nil {
		t.Fatalf("write settings.hcl: %v", err)
	}

	_, supported, err := resolveRepoHoverRuntime(repoPath, filePath)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if supported {
		t.Fatalf("expected generic hcl file to be unsupported")
	}
}

func TestResolveRepoHoverRuntimeUsesNearestTerraformRoot(t *testing.T) {
	repoPath := t.TempDir()
	nestedRoot := filepath.Join(repoPath, "infra", "prod")
	moduleDir := filepath.Join(nestedRoot, "modules", "app")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("write repo main.tf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedRoot, "main.tf"), []byte("module \"app\" {}\n"), 0o644); err != nil {
		t.Fatalf("write nested main.tf: %v", err)
	}
	targetFile := filepath.Join(moduleDir, "variables.tf")
	if err := os.WriteFile(targetFile, []byte("variable \"name\" {}\n"), 0o644); err != nil {
		t.Fatalf("write module variables.tf: %v", err)
	}

	runtime, supported, err := resolveRepoHoverRuntime(repoPath, targetFile)
	if err != nil {
		t.Fatalf("resolveRepoHoverRuntime: %v", err)
	}
	if !supported {
		t.Fatalf("expected Terraform file to be supported")
	}
	if runtime.rootPath != moduleDir {
		t.Fatalf("expected nearest terraform root %q, got %q", moduleDir, runtime.rootPath)
	}
}

func TestGetRepoFileHoverReturnsUnavailableWhenProviderMissing(t *testing.T) {
	originalLookPath := hoverLookPath
	defer func() {
		hoverLookPath = originalLookPath
	}()
	hoverLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: app\n    repo_dir: repo\n")
	repoPath := filepath.Join(workspaceRoot, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, "src"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "src", "example.ts"), []byte("map(foo)\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	response, err := app.GetRepoFileHover(RepoFileHoverRequest{
		WorkspaceID: "ws-1",
		RepoID:      "ws-1::app",
		Path:        "src/example.ts",
		Content:     "map(foo)\n",
		Line:        0,
		Character:   1,
	})
	if err != nil {
		t.Fatalf("GetRepoFileHover: %v", err)
	}
	if !response.Supported {
		t.Fatalf("expected TypeScript hover to be supported")
	}
	if response.Available {
		t.Fatalf("expected hover provider to be unavailable")
	}
	if response.InstallHint == "" {
		t.Fatalf("expected install hint when provider is unavailable")
	}
}

func TestGetRepoFileHoverCachesRepoHoverBackends(t *testing.T) {
	originalLookPath := hoverLookPath
	originalFactory := newRepoHoverBackend
	defer func() {
		hoverLookPath = originalLookPath
		newRepoHoverBackend = originalFactory
	}()

	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: app\n    repo_dir: repo\n")
	repoPath := filepath.Join(workspaceRoot, "repo")
	binPath := filepath.Join(repoPath, "node_modules", ".bin")
	if err := os.MkdirAll(filepath.Join(repoPath, "src"), 0o755); err != nil {
		t.Fatalf("mkdir repo/src: %v", err)
	}
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("mkdir repo bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "src", "example.ts"), []byte("map(foo)\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binPath, "vtsls"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write local vtsls stub: %v", err)
	}

	hoverLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	factoryCalls := 0
	backend := &stubRepoHoverBackend{
		response: RepoFileHoverResponse{
			Supported: true,
			Available: true,
			Found:     true,
			Header:    "map<T, U>(value: T): U",
		},
	}
	newRepoHoverBackend = func(context.Context, repoHoverRuntime) (repoHoverBackend, error) {
		factoryCalls++
		return backend, nil
	}

	for i := 0; i < 2; i++ {
		response, err := app.GetRepoFileHover(RepoFileHoverRequest{
			WorkspaceID: "ws-1",
			RepoID:      "ws-1::app",
			Path:        "src/example.ts",
			Content:     "map(foo)\n",
			Line:        0,
			Character:   1,
		})
		if err != nil {
			t.Fatalf("GetRepoFileHover call %d: %v", i+1, err)
		}
		if !response.Found {
			t.Fatalf("expected hover content to be found")
		}
	}

	if factoryCalls != 1 {
		t.Fatalf("expected backend factory to be called once, got %d", factoryCalls)
	}
	if backend.hoverCalls != 2 {
		t.Fatalf("expected cached backend to serve both hovers, got %d", backend.hoverCalls)
	}
}

func TestGetRepoFileDefinitionReturnsUnavailableWhenProviderMissing(t *testing.T) {
	originalLookPath := hoverLookPath
	defer func() {
		hoverLookPath = originalLookPath
	}()
	hoverLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: app\n    repo_dir: repo\n")
	repoPath := filepath.Join(workspaceRoot, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, "src"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "src", "example.ts"), []byte("map(foo)\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	response, err := app.GetRepoFileDefinition(RepoFileDefinitionRequest{
		WorkspaceID: "ws-1",
		RepoID:      "ws-1::app",
		Path:        "src/example.ts",
		Content:     "map(foo)\n",
		Line:        0,
		Character:   1,
	})
	if err != nil {
		t.Fatalf("GetRepoFileDefinition: %v", err)
	}
	if !response.Supported {
		t.Fatalf("expected TypeScript definition to be supported")
	}
	if response.Available {
		t.Fatalf("expected definition provider to be unavailable")
	}
	if response.InstallHint == "" {
		t.Fatalf("expected install hint when provider is unavailable")
	}
}

func TestGetRepoFileHoverReturnsUnavailableWhenTerraformProviderMissing(t *testing.T) {
	originalLookPath := hoverLookPath
	defer func() {
		hoverLookPath = originalLookPath
	}()
	hoverLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: infra\n    repo_dir: repo\n")
	repoPath := filepath.Join(workspaceRoot, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("write main.tf: %v", err)
	}

	response, err := app.GetRepoFileHover(RepoFileHoverRequest{
		WorkspaceID: "ws-1",
		RepoID:      "ws-1::infra",
		Path:        "main.tf",
		Content:     "terraform {}\n",
		Line:        0,
		Character:   1,
	})
	if err != nil {
		t.Fatalf("GetRepoFileHover: %v", err)
	}
	if !response.Supported {
		t.Fatalf("expected Terraform hover to be supported")
	}
	if response.Available {
		t.Fatalf("expected Terraform hover provider to be unavailable")
	}
	if response.InstallHint == "" {
		t.Fatalf("expected install hint when Terraform provider is unavailable")
	}
}

func TestGetRepoFileDefinitionMapsTargetsIntoWorkspaceRoots(t *testing.T) {
	originalLookPath := hoverLookPath
	originalFactory := newRepoHoverBackend
	defer func() {
		hoverLookPath = originalLookPath
		newRepoHoverBackend = originalFactory
	}()

	app, workspaceRoot := setupRepoFilesAppWithWorkspace(t, "ws-1", "name: ws-1\nrepos:\n  - name: app\n    repo_dir: repo\n")
	repoPath := filepath.Join(workspaceRoot, "repo")
	binPath := filepath.Join(repoPath, "node_modules", ".bin")
	libPath := filepath.Join(repoPath, "src", "lib.ts")
	if err := os.MkdirAll(filepath.Join(repoPath, "src"), 0o755); err != nil {
		t.Fatalf("mkdir repo/src: %v", err)
	}
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("mkdir repo bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "src", "example.ts"), []byte("helper()\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(libPath, []byte("export function helper() {}\n"), 0o644); err != nil {
		t.Fatalf("write lib file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binPath, "vtsls"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write local vtsls stub: %v", err)
	}

	hoverLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	backend := &stubRepoHoverBackend{
		definitions: []repoFileDefinitionLocation{
			{
				filePath:       libPath,
				startLine:      0,
				startCharacter: 16,
				endLine:        0,
				endCharacter:   22,
			},
		},
	}
	newRepoHoverBackend = func(context.Context, repoHoverRuntime) (repoHoverBackend, error) {
		return backend, nil
	}

	response, err := app.GetRepoFileDefinition(RepoFileDefinitionRequest{
		WorkspaceID: "ws-1",
		RepoID:      "ws-1::app",
		Path:        "src/example.ts",
		Content:     "helper()\n",
		Line:        0,
		Character:   1,
	})
	if err != nil {
		t.Fatalf("GetRepoFileDefinition: %v", err)
	}
	if backend.definitionCalls != 1 {
		t.Fatalf("expected definition backend to be called once, got %d", backend.definitionCalls)
	}
	if !response.Found {
		t.Fatalf("expected definition target to be found, got %+v", response)
	}
	if len(response.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(response.Targets))
	}
	target := response.Targets[0]
	if target.RepoID != "ws-1::app" {
		t.Fatalf("expected repo target ws-1::app, got %q", target.RepoID)
	}
	if target.Path != "src/lib.ts" {
		t.Fatalf("expected target path src/lib.ts, got %q", target.Path)
	}
}

func TestTSServerHoverBackendReturnsQuickInfo(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	tsserverPath := filepath.Join(workingDir, "frontend", "node_modules", ".bin", "tsserver")
	if _, err := os.Stat(tsserverPath); err != nil {
		t.Skipf("tsserver not available at %s: %v", tsserverPath, err)
	}

	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}

	backend, err := newTSServerHoverBackend(context.Background(), repoHoverRuntime{
		backendType: "tsserver",
		language:    "typescript",
		languageID:  "typescript",
		provider:    "tsserver",
		rootPath:    repoPath,
		command:     tsserverPath,
	})
	if err != nil {
		t.Fatalf("newTSServerHoverBackend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	content := "const items = [1, 2, 3];\nitems.map((value) => value.toFixed());\n"
	response, err := backend.Hover(context.Background(), repoHoverLSPRequest{
		filePath:   filepath.Join(repoPath, "example.ts"),
		path:       "example.ts",
		content:    content,
		line:       1,
		character:  strings.Index("items.map((value) => value.toFixed());", "map"),
		languageID: "typescript",
		language:   "typescript",
		provider:   "tsserver",
	})
	if err != nil {
		t.Fatalf("Hover: %v", err)
	}
	if !response.Found {
		t.Fatalf("expected tsserver hover to find symbol info")
	}
	if !strings.Contains(response.Header, "map") {
		t.Fatalf("expected hover header to mention map, got %q", response.Header)
	}
}

func TestTSServerDefinitionReturnsFileTarget(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	tsserverPath := filepath.Join(workingDir, "frontend", "node_modules", ".bin", "tsserver")
	if _, err := os.Stat(tsserverPath); err != nil {
		t.Skipf("tsserver not available at %s: %v", tsserverPath, err)
	}

	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}

	backend, err := newTSServerHoverBackend(context.Background(), repoHoverRuntime{
		backendType: "tsserver",
		language:    "typescript",
		languageID:  "typescript",
		provider:    "tsserver",
		rootPath:    repoPath,
		command:     tsserverPath,
	})
	if err != nil {
		t.Fatalf("newTSServerHoverBackend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	libPath := filepath.Join(repoPath, "lib.ts")
	content := "import { helper } from './lib';\nhelper();\n"
	if err := os.WriteFile(libPath, []byte("export function helper(): string { return 'ok'; }\n"), 0o644); err != nil {
		t.Fatalf("write lib.ts: %v", err)
	}
	targets, err := backend.Definition(context.Background(), repoHoverLSPRequest{
		filePath:   filepath.Join(repoPath, "example.ts"),
		path:       "example.ts",
		content:    content,
		line:       1,
		character:  strings.Index("helper();", "helper"),
		languageID: "typescript",
		language:   "typescript",
		provider:   "tsserver",
	})
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(targets) == 0 {
		t.Fatalf("expected tsserver definition to return at least one target")
	}
	if filepath.Clean(targets[0].filePath) != libPath {
		t.Fatalf("expected definition target %q, got %q", libPath, targets[0].filePath)
	}
}
