package worksetapi

import "testing"

func TestParseGitHubRemoteURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		host  string
		owner string
		repo  string
	}{
		{"ssh", "git@github.com:acme/widgets.git", "github.com", "acme", "widgets"},
		{"https", "https://github.com/acme/widgets", "github.com", "acme", "widgets"},
		{"ssh-url", "ssh://git@github.com/acme/widgets.git", "github.com", "acme", "widgets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseGitHubRemoteURL(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if info.Host != tt.host || info.Owner != tt.owner || info.Repo != tt.repo {
				t.Fatalf("unexpected parse result: %+v", info)
			}
		})
	}
}

func TestParseAgentJSON(t *testing.T) {
	payload := `Some preface text
{"title":"feat: add api","body":"Adds the API."}
Extra text`
	result, err := parseAgentJSON(payload)
	if err != nil {
		t.Fatalf("parse agent json: %v", err)
	}
	if result.Title != "feat: add api" {
		t.Fatalf("unexpected title: %q", result.Title)
	}
}
