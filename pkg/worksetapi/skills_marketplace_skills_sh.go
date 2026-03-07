package worksetapi

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type skillsSHMarketplaceProvider struct {
	client  *http.Client
	baseURL string
}

type skillsSHSearchResponse struct {
	Skills []struct {
		ID          string `json:"id"`
		SkillID     string `json:"skillId"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Installs    int    `json:"installs"`
		Source      string `json:"source"`
	} `json:"skills"`
}

var (
	weeklyInstallsRe  = regexp.MustCompile(`(?s)Weekly Installs.*?text-3xl[^>]*>([^<]+)<`)
	githubStarsRe     = regexp.MustCompile(`(?s)GitHub Stars.*?<span>([^<]+)</span>`)
	firstSeenRe       = regexp.MustCompile(`(?s)First Seen.*?text-sm font-mono text-foreground">([^<]+)<`)
	securitySectionRe = regexp.MustCompile(`(?s)Security Audits.*?<div class="divide-y divide-border">(.*?)</div></div>`)
	securityRowRe     = regexp.MustCompile(`href="([^"]+/security/([^"]+))".*?<span class="text-sm font-medium text-foreground truncate">([^<]+)</span>.*?<span class="text-xs font-mono uppercase px-2 py-1 rounded [^"]*">([^<]+)</span>`)
)

func newSkillsSHMarketplaceProvider(client *http.Client) *skillsSHMarketplaceProvider {
	return &skillsSHMarketplaceProvider{
		client:  client,
		baseURL: "https://skills.sh",
	}
}

func (p *skillsSHMarketplaceProvider) Search(ctx context.Context, query string, limit int) ([]MarketplaceSkill, error) {
	endpoint := fmt.Sprintf("%s/api/search?q=%s&limit=%d", p.baseURL, url.QueryEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build skills.sh request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request skills.sh: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request skills.sh: unexpected status %d", resp.StatusCode)
	}

	var payload skillsSHSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode skills.sh response: %w", err)
	}

	results := make([]MarketplaceSkill, 0, len(payload.Skills))
	for _, entry := range payload.Skills {
		installs := entry.Installs
		sourceRepo := strings.TrimSpace(entry.Source)
		skillID := strings.TrimSpace(entry.SkillID)
		externalID := strings.TrimSpace(entry.ID)
		if externalID == "" && sourceRepo != "" && skillID != "" {
			externalID = sourceRepo + "/" + skillID
		}
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/skills/%s/SKILL.md", sourceRepo, skillID)
		results = append(results, MarketplaceSkill{
			Provider:     MarketplaceProviderSkillsSh,
			ExternalID:   externalID,
			Name:         strings.TrimSpace(entry.Name),
			Description:  strings.TrimSpace(entry.Description),
			SourceRepo:   sourceRepo,
			SourceURL:    "https://github.com/" + sourceRepo,
			ListingURL:   p.skillDetailURL(externalID),
			RawSkillURL:  rawURL,
			InstallCount: &installs,
		})
	}
	p.enrichSearchResults(ctx, results)
	return results, nil
}

func (p *skillsSHMarketplaceProvider) GetMetadata(ctx context.Context, skill MarketplaceSkill) (MarketplaceSkill, error) {
	detailURL := strings.TrimSpace(skill.ListingURL)
	if detailURL == "" {
		detailURL = p.skillDetailURL(skill.ExternalID)
	}
	if detailURL == "" {
		return skill, nil
	}
	detailHTML, err := fetchMarketplaceContent(ctx, p.client, detailURL)
	if err != nil {
		return MarketplaceSkill{}, err
	}
	return enrichSkillFromDetailPage(skill, detailURL, detailHTML), nil
}

func (p *skillsSHMarketplaceProvider) GetContent(ctx context.Context, skill MarketplaceSkill) (MarketplaceSkillContent, error) {
	candidates := []string{strings.TrimSpace(skill.RawSkillURL)}
	if strings.Contains(skill.RawSkillURL, "/main/") {
		candidates = append(candidates, strings.Replace(skill.RawSkillURL, "/main/", "/master/", 1))
	}

	var (
		content string
		lastErr error
	)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		fetchedContent, err := fetchMarketplaceContent(ctx, p.client, candidate)
		if err == nil {
			skill.RawSkillURL = candidate
			content = fetchedContent
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		return MarketplaceSkillContent{}, lastErr
	}

	detailURL := strings.TrimSpace(skill.ListingURL)
	if detailURL == "" {
		detailURL = p.skillDetailURL(skill.ExternalID)
	}
	if detailURL != "" {
		if metadata, err := p.GetMetadata(ctx, skill); err == nil {
			skill = metadata
		}
	}

	return MarketplaceSkillContent{Skill: skill, Content: content}, nil
}

func (p *skillsSHMarketplaceProvider) skillDetailURL(externalID string) string {
	externalID = strings.Trim(strings.TrimSpace(externalID), "/")
	if externalID == "" {
		return ""
	}
	return p.baseURL + "/" + externalID
}

func (p *skillsSHMarketplaceProvider) enrichSearchResults(ctx context.Context, skills []MarketplaceSkill) {
	maxEnriched := min(len(skills), 8)
	var wg sync.WaitGroup
	for index := 0; index < maxEnriched; index++ {
		wg.Add(1)
		go func(targetIndex int) {
			defer wg.Done()
			detailURL := skills[targetIndex].ListingURL
			if detailURL == "" {
				return
			}
			detailHTML, err := fetchMarketplaceContent(ctx, p.client, detailURL)
			if err != nil {
				return
			}
			skills[targetIndex] = enrichSkillFromDetailPage(skills[targetIndex], detailURL, detailHTML)
		}(index)
	}
	wg.Wait()
}

func enrichSkillFromDetailPage(skill MarketplaceSkill, detailURL, input string) MarketplaceSkill {
	skill.ListingURL = detailURL

	if weekly := parseCompactCount(firstMatch(weeklyInstallsRe, input)); weekly != nil {
		skill.WeeklyInstalls = weekly
	}
	if stars := parseCompactCount(firstMatch(githubStarsRe, input)); stars != nil {
		skill.GitHubStars = stars
	}
	if firstSeen := strings.TrimSpace(firstMatch(firstSeenRe, input)); firstSeen != "" {
		firstSeen = html.UnescapeString(firstSeen)
		if firstSeen != "Jan 1, 1970" {
			skill.FirstSeen = firstSeen
		}
	}

	repoVerified := strings.Contains(input, `aria-label="Verified organization on GitHub"`)
	skill.RepoVerified = &repoVerified

	if len(skill.AuditSummaries) == 0 {
		skill.AuditSummaries = parseDetailAuditSummaries(detailURL, input)
	}
	return skill
}

func parseDetailAuditSummaries(detailURL, input string) []MarketplaceAuditSummary {
	section := firstMatch(securitySectionRe, input)
	if section == "" {
		return nil
	}
	matches := securityRowRe.FindAllStringSubmatch(section, -1)
	if len(matches) == 0 {
		return nil
	}
	results := make([]MarketplaceAuditSummary, 0, len(matches))
	for _, match := range matches {
		if len(match) < 5 {
			continue
		}
		relativeURL := strings.TrimSpace(match[1])
		if strings.HasPrefix(relativeURL, "/") {
			relativeURL = "https://skills.sh" + relativeURL
		} else if !strings.HasPrefix(relativeURL, "http") && detailURL != "" {
			relativeURL = strings.TrimRight(detailURL, "/") + "/" + relativeURL
		}
		results = append(results, MarketplaceAuditSummary{
			Provider:  html.UnescapeString(strings.TrimSpace(match[3])),
			Status:    html.UnescapeString(strings.TrimSpace(match[4])),
			DetailURL: relativeURL,
		})
	}
	return results
}

func firstMatch(re *regexp.Regexp, input string) string {
	match := re.FindStringSubmatch(input)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func parseCompactCount(input string) *int {
	value := strings.TrimSpace(strings.ReplaceAll(input, ",", ""))
	if value == "" {
		return nil
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(value, "K"):
		multiplier = 1_000
		value = strings.TrimSuffix(value, "K")
	case strings.HasSuffix(value, "M"):
		multiplier = 1_000_000
		value = strings.TrimSuffix(value, "M")
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	parsed := int(number * multiplier)
	return &parsed
}
