package worksetapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	MarketplaceProviderSkillsSh = "skills.sh"
)

var marketplaceHTTPClient = &http.Client{Timeout: 8 * time.Second}

// MarketplaceSkill is a provider-normalized remote skill listing.
type MarketplaceSkill struct {
	Provider       string                    `json:"provider"`
	ExternalID     string                    `json:"externalId"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description"`
	SourceRepo     string                    `json:"sourceRepo"`
	SourceURL      string                    `json:"sourceUrl"`
	ListingURL     string                    `json:"listingUrl,omitempty"`
	RawSkillURL    string                    `json:"rawSkillUrl"`
	InstallCount   *int                      `json:"installCount,omitempty"`
	WeeklyInstalls *int                      `json:"weeklyInstalls,omitempty"`
	GitHubStars    *int                      `json:"githubStars,omitempty"`
	FirstSeen      string                    `json:"firstSeen,omitempty"`
	RepoVerified   *bool                     `json:"repoVerified,omitempty"`
	AuditSummaries []MarketplaceAuditSummary `json:"auditSummaries,omitempty"`
	Verified       *bool                     `json:"verified,omitempty"`
	TrustScore     *float64                  `json:"trustScore,omitempty"`
	BenchmarkScore *int                      `json:"benchmarkScore,omitempty"`
	Relevance      *float64                  `json:"relevance,omitempty"`
}

type MarketplaceAuditSummary struct {
	Provider  string `json:"provider"`
	Status    string `json:"status"`
	DetailURL string `json:"detailUrl,omitempty"`
}

// MarketplaceSkillContent includes normalized metadata plus raw SKILL.md content.
type MarketplaceSkillContent struct {
	Skill   MarketplaceSkill `json:"skill"`
	Content string           `json:"content"`
}

// MarketplaceSkillRequest identifies a marketplace skill without forcing the
// caller to know the provider-specific source shape.
type MarketplaceSkillRequest struct {
	Provider     string `json:"provider"`
	ExternalID   string `json:"externalId,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	SourceRepo   string `json:"sourceRepo,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	ListingURL   string `json:"listingUrl,omitempty"`
	RawSkillURL  string `json:"rawSkillUrl,omitempty"`
	InstallCount *int   `json:"installCount,omitempty"`
}

// InstallMarketplaceSkillInput installs a remote skill into the local skill registry.
type InstallMarketplaceSkillInput struct {
	MarketplaceSkillRequest

	Scope   string   `json:"scope"`
	DirName string   `json:"dirName"`
	Tools   []string `json:"tools"`
}

type marketplaceProvider interface {
	Search(ctx context.Context, query string, limit int) ([]MarketplaceSkill, error)
	GetMetadata(ctx context.Context, skill MarketplaceSkill) (MarketplaceSkill, error)
	GetContent(ctx context.Context, skill MarketplaceSkill) (MarketplaceSkillContent, error)
}

func newMarketplaceProviders(client *http.Client) map[string]marketplaceProvider {
	return map[string]marketplaceProvider{
		MarketplaceProviderSkillsSh: newSkillsSHMarketplaceProvider(client),
	}
}

func normalizeMarketplaceLimit(limit int) int {
	switch {
	case limit <= 0:
		return 24
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func marketplaceSkillLess(left, right MarketplaceSkill) bool {
	leftInstall := -1
	if left.InstallCount != nil {
		leftInstall = *left.InstallCount
	}
	rightInstall := -1
	if right.InstallCount != nil {
		rightInstall = *right.InstallCount
	}
	if leftInstall != rightInstall {
		return leftInstall > rightInstall
	}

	leftVerified := left.Verified != nil && *left.Verified
	rightVerified := right.Verified != nil && *right.Verified
	if leftVerified != rightVerified {
		return leftVerified
	}

	if left.Name != right.Name {
		return left.Name < right.Name
	}
	return left.Provider < right.Provider
}

func normalizeMarketplaceSkill(input MarketplaceSkillRequest) MarketplaceSkill {
	sourceRepo := strings.TrimPrefix(strings.TrimSpace(input.SourceRepo), "/")
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" && sourceRepo != "" {
		sourceURL = "https://github.com/" + sourceRepo
	}
	return MarketplaceSkill{
		Provider:     strings.TrimSpace(input.Provider),
		ExternalID:   strings.TrimSpace(input.ExternalID),
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		SourceRepo:   sourceRepo,
		SourceURL:    sourceURL,
		ListingURL:   strings.TrimSpace(input.ListingURL),
		RawSkillURL:  strings.TrimSpace(input.RawSkillURL),
		InstallCount: input.InstallCount,
	}
}

func normalizeMarketplaceProvider(provider string) string {
	value := strings.TrimSpace(provider)
	if value == "" || value == "all" {
		return MarketplaceProviderSkillsSh
	}
	return value
}

func fetchMarketplaceContent(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request %s: %w", rawURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request %s: unexpected status %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rawURL, err)
	}
	return string(body), nil
}

func (s *Service) SearchMarketplaceSkills(ctx context.Context, provider, query string, limit int) ([]MarketplaceSkill, error) {
	provider = normalizeMarketplaceProvider(provider)
	query = strings.TrimSpace(query)
	if query == "" {
		return []MarketplaceSkill{}, nil
	}
	limit = normalizeMarketplaceLimit(limit)

	providers := newMarketplaceProviders(marketplaceHTTPClient)
	selected, ok := providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported marketplace provider %q", provider)
	}
	results, err := selected.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return marketplaceSkillLess(results[i], results[j])
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *Service) GetMarketplaceSkillContent(ctx context.Context, input MarketplaceSkillRequest) (MarketplaceSkillContent, error) {
	skill := normalizeMarketplaceSkill(input)
	if skill.Provider == "" {
		return MarketplaceSkillContent{}, errors.New("marketplace provider is required")
	}
	provider, ok := newMarketplaceProviders(marketplaceHTTPClient)[skill.Provider]
	if !ok {
		return MarketplaceSkillContent{}, fmt.Errorf("unsupported marketplace provider %q", skill.Provider)
	}
	return provider.GetContent(ctx, skill)
}

func (s *Service) GetMarketplaceSkillMetadata(ctx context.Context, input MarketplaceSkillRequest) (MarketplaceSkill, error) {
	skill := normalizeMarketplaceSkill(input)
	if skill.Provider == "" {
		return MarketplaceSkill{}, errors.New("marketplace provider is required")
	}
	provider, ok := newMarketplaceProviders(marketplaceHTTPClient)[skill.Provider]
	if !ok {
		return MarketplaceSkill{}, fmt.Errorf("unsupported marketplace provider %q", skill.Provider)
	}
	return provider.GetMetadata(ctx, skill)
}

func (s *Service) InstallMarketplaceSkill(ctx context.Context, input InstallMarketplaceSkillInput, projectRoot string) (SkillInfo, error) {
	if strings.TrimSpace(input.Scope) == "" {
		return SkillInfo{}, errors.New("scope is required")
	}
	if strings.TrimSpace(input.DirName) == "" {
		return SkillInfo{}, errors.New("dirName is required")
	}
	if len(input.Tools) == 0 {
		return SkillInfo{}, errors.New("at least one target tool is required")
	}

	content, err := s.GetMarketplaceSkillContent(ctx, input.MarketplaceSkillRequest)
	if err != nil {
		return SkillInfo{}, err
	}

	for _, tool := range input.Tools {
		if err := saveSkillToPath(input.Scope, input.DirName, tool, content.Content, projectRoot); err != nil {
			return SkillInfo{}, err
		}
		path, err := resolveSkillPathWithRoot(input.Scope, input.DirName, tool, projectRoot)
		if err != nil {
			return SkillInfo{}, err
		}
		if err := writeSkillMarketplaceSource(filepath.Dir(path), &SkillMarketplaceSource{
			Provider:    content.Skill.Provider,
			ExternalID:  content.Skill.ExternalID,
			SourceRepo:  content.Skill.SourceRepo,
			SourceURL:   content.Skill.SourceURL,
			ListingURL:  content.Skill.ListingURL,
			RawSkillURL: content.Skill.RawSkillURL,
		}); err != nil {
			return SkillInfo{}, err
		}
	}

	name, description := parseSkillFrontmatter(content.Content)
	if name == "" {
		name = input.DirName
	}
	path, err := resolveSkillPathWithRoot(input.Scope, input.DirName, input.Tools[0], projectRoot)
	if err != nil {
		return SkillInfo{}, err
	}

	return SkillInfo{
		Name:        name,
		Description: description,
		DirName:     input.DirName,
		Scope:       input.Scope,
		Tools:       append([]string(nil), input.Tools...),
		Path:        path,
		Marketplace: &SkillMarketplaceSource{
			Provider:    content.Skill.Provider,
			ExternalID:  content.Skill.ExternalID,
			SourceRepo:  content.Skill.SourceRepo,
			SourceURL:   content.Skill.SourceURL,
			ListingURL:  content.Skill.ListingURL,
			RawSkillURL: content.Skill.RawSkillURL,
		},
	}, nil
}
