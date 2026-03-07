package worksetapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsSHMarketplaceProviderSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			_, _ = w.Write([]byte(`{"skills":[{"id":"anthropics/skills/frontend-design","skillId":"frontend-design","name":"frontend-design","installs":129428,"source":"anthropics/skills"}]}`))
		case "/anthropics/skills/frontend-design":
			_, _ = w.Write([]byte(`
				<div>Weekly Installs</div><div class="text-3xl font-semibold font-mono tracking-tight text-foreground">129.5K</div>
				<div>Repository</div><span aria-label="Verified organization on GitHub"></span>
				<div>GitHub Stars</div><div><span>86.0K</span></div>
				<div>First Seen</div><div class="text-sm font-mono text-foreground">Jan 19, 2026</div>
				<div>Security Audits</div><div class="divide-y divide-border">
					<a href="/anthropics/skills/frontend-design/security/agent-trust-hub"><span class="text-sm font-medium text-foreground truncate">Gen Agent Trust Hub</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">Pass</span></a>
					<a href="/anthropics/skills/frontend-design/security/socket"><span class="text-sm font-medium text-foreground truncate">Socket</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">0 alerts</span></a>
					<a href="/anthropics/skills/frontend-design/security/snyk"><span class="text-sm font-medium text-foreground truncate">Snyk</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">Low Risk</span></a>
				</div></div>`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := &skillsSHMarketplaceProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	results, err := provider.Search(context.Background(), "frontend", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Provider != MarketplaceProviderSkillsSh {
		t.Fatalf("provider = %q, want %q", result.Provider, MarketplaceProviderSkillsSh)
	}
	if result.SourceRepo != "anthropics/skills" {
		t.Fatalf("sourceRepo = %q", result.SourceRepo)
	}
	if result.RawSkillURL != "https://raw.githubusercontent.com/anthropics/skills/main/skills/frontend-design/SKILL.md" {
		t.Fatalf("rawSkillUrl = %q", result.RawSkillURL)
	}
	if result.InstallCount == nil || *result.InstallCount != 129428 {
		t.Fatalf("installCount = %v", result.InstallCount)
	}
	if len(result.AuditSummaries) != 3 {
		t.Fatalf("expected 3 audit summaries, got %d", len(result.AuditSummaries))
	}
	if result.AuditSummaries[0].Provider != "Gen Agent Trust Hub" || result.AuditSummaries[0].Status != "Pass" {
		t.Fatalf("unexpected first audit: %+v", result.AuditSummaries[0])
	}
}

func TestSkillsSHMarketplaceProviderGetContentFallsBackToMaster(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/main/skills/frontend-design/SKILL.md":
			http.NotFound(w, r)
		case "/master/skills/frontend-design/SKILL.md":
			_, _ = w.Write([]byte("---\nname: frontend-design\n---\n# frontend"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := &skillsSHMarketplaceProvider{client: server.Client()}
	content, err := provider.GetContent(context.Background(), MarketplaceSkill{
		Provider:    MarketplaceProviderSkillsSh,
		Name:        "frontend-design",
		RawSkillURL: server.URL + "/main/skills/frontend-design/SKILL.md",
	})
	if err != nil {
		t.Fatalf("GetContent() error = %v", err)
	}
	if !strings.Contains(content.Content, "# frontend") {
		t.Fatalf("content = %q", content.Content)
	}
	if content.Skill.RawSkillURL != server.URL+"/master/skills/frontend-design/SKILL.md" {
		t.Fatalf("resolved rawSkillUrl = %q", content.Skill.RawSkillURL)
	}
}

func TestEnrichSkillFromDetailPage(t *testing.T) {
	t.Parallel()

	repoVerified := true
	skill := MarketplaceSkill{
		Provider:   MarketplaceProviderSkillsSh,
		ExternalID: "anthropics/skills/frontend-design",
		Name:       "frontend-design",
		SourceRepo: "anthropics/skills",
	}
	input := `
	<div>Weekly Installs</div><div class="text-3xl font-semibold font-mono tracking-tight text-foreground">129.5K</div>
	<div>Repository</div><span aria-label="Verified organization on GitHub"></span>
	<div>GitHub Stars</div><div><span>86.0K</span></div>
	<div>First Seen</div><div class="text-sm font-mono text-foreground">Jan 19, 2026</div>
	<div>Security Audits</div><div class="divide-y divide-border">
		<a href="/anthropics/skills/frontend-design/security/agent-trust-hub"><span class="text-sm font-medium text-foreground truncate">Gen Agent Trust Hub</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">Pass</span></a>
		<a href="/anthropics/skills/frontend-design/security/socket"><span class="text-sm font-medium text-foreground truncate">Socket</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">0 alerts</span></a>
		<a href="/anthropics/skills/frontend-design/security/snyk"><span class="text-sm font-medium text-foreground truncate">Snyk</span><span class="text-xs font-mono uppercase px-2 py-1 rounded bg-green-500/10 text-green-500">Low Risk</span></a>
	</div></div>`

	enriched := enrichSkillFromDetailPage(skill, "https://skills.sh/anthropics/skills/frontend-design", input)
	if enriched.WeeklyInstalls == nil || *enriched.WeeklyInstalls != 129500 {
		t.Fatalf("weekly installs = %v", enriched.WeeklyInstalls)
	}
	if enriched.GitHubStars == nil || *enriched.GitHubStars != 86000 {
		t.Fatalf("github stars = %v", enriched.GitHubStars)
	}
	if enriched.FirstSeen != "Jan 19, 2026" {
		t.Fatalf("first seen = %q", enriched.FirstSeen)
	}
	if enriched.RepoVerified == nil || *enriched.RepoVerified != repoVerified {
		t.Fatalf("repo verified = %v", enriched.RepoVerified)
	}
	if len(enriched.AuditSummaries) != 3 {
		t.Fatalf("expected 3 audit summaries, got %d", len(enriched.AuditSummaries))
	}
}

func TestInstallMarketplaceSkillInstallsAcrossTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/skill.md" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`---
name: frontend-design
description: Create polished UI
---
# frontend`))
	}))
	defer server.Close()

	svc := &Service{}
	got, err := svc.InstallMarketplaceSkill(context.Background(), InstallMarketplaceSkillInput{
		MarketplaceSkillRequest: MarketplaceSkillRequest{
			Provider:    MarketplaceProviderSkillsSh,
			ExternalID:  "anthropics/skills/frontend-design",
			Name:        "frontend-design",
			SourceRepo:  "anthropics/skills",
			RawSkillURL: server.URL + "/skill.md",
		},
		Scope:   "global",
		DirName: "frontend-design",
		Tools:   []string{"agents", "codex"},
	}, "")
	if err != nil {
		t.Fatalf("InstallMarketplaceSkill() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", "frontend-design", "SKILL.md"),
		filepath.Join(home, ".codex", "skills", "frontend-design", "SKILL.md"),
	} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("missing installed file %s: %v", path, readErr)
		}
		if !strings.Contains(string(data), "# frontend") {
			t.Fatalf("unexpected content in %s", path)
		}
		metadata, readErr := readSkillMarketplaceSource(filepath.Dir(path))
		if readErr != nil {
			t.Fatalf("missing metadata for %s: %v", path, readErr)
		}
		if metadata == nil || metadata.ExternalID == "" {
			t.Fatalf("expected marketplace metadata for %s, got %+v", path, metadata)
		}
	}

	if got.Name != "frontend-design" {
		t.Fatalf("Name = %q", got.Name)
	}
	if got.Description != "Create polished UI" {
		t.Fatalf("Description = %q", got.Description)
	}
	if got.Scope != "global" {
		t.Fatalf("Scope = %q", got.Scope)
	}
	if len(got.Tools) != 2 {
		t.Fatalf("Tools = %v", got.Tools)
	}
	if got.Marketplace == nil || got.Marketplace.SourceRepo != "anthropics/skills" {
		t.Fatalf("Marketplace = %+v", got.Marketplace)
	}
}
