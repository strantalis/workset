package main

import "testing"

func TestMergePathEntries(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		fromLogin string
		want      string
	}{
		{
			name:      "returns login when current empty",
			current:   "",
			fromLogin: "/usr/local/bin:/usr/bin",
			want:      "/usr/local/bin:/usr/bin",
		},
		{
			name:      "preserves current when login empty",
			current:   "/opt/bin:/usr/bin",
			fromLogin: "",
			want:      "/opt/bin:/usr/bin",
		},
		{
			name:      "dedupes and appends missing entries",
			current:   "/opt/bin:/usr/bin",
			fromLogin: "/usr/bin:/bin",
			want:      "/opt/bin:/usr/bin:/bin",
		},
		{
			name:      "dedupes duplicates in current",
			current:   "/opt/bin:/opt/bin:/usr/bin",
			fromLogin: "/usr/bin:/bin",
			want:      "/opt/bin:/usr/bin:/bin",
		},
		{
			name:      "dedupes duplicates in login",
			current:   "/opt/bin:/usr/bin",
			fromLogin: "/bin:/bin:/usr/bin",
			want:      "/opt/bin:/usr/bin:/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergePathEntries(tt.current, tt.fromLogin)
			if got != tt.want {
				t.Fatalf("mergePathEntries(%q, %q) = %q, want %q", tt.current, tt.fromLogin, got, tt.want)
			}
		})
	}
}

func TestExtractPathFromShellOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "extracts from prefixed line",
			output: "noise\n__WORKSET_PATH__/opt/bin:/usr/bin\n",
			want:   "/opt/bin:/usr/bin",
		},
		{
			name:   "uses last prefixed line",
			output: "__WORKSET_PATH__/bin\nstuff\n__WORKSET_PATH__/usr/local/bin:/bin\n",
			want:   "/usr/local/bin:/bin",
		},
		{
			name:   "returns empty when missing",
			output: "noise only\n",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPathFromShellOutput(tt.output)
			if got != tt.want {
				t.Fatalf("extractPathFromShellOutput(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}
