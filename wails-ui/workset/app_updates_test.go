package main

import "testing"

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    string
		right   string
		want    int
		wantErr bool
	}{
		{name: "newer stable", left: "v1.2.0", right: "v1.1.9", want: 1},
		{name: "equal stable", left: "v1.2.0", right: "v1.2.0", want: 0},
		{name: "alpha lower than stable", left: "v1.2.0-alpha.2", right: "v1.2.0", want: -1},
		{name: "alpha ordinal compare", left: "v1.2.0-alpha.10", right: "v1.2.0-alpha.2", want: 1},
		{name: "invalid", left: "1.2", right: "v1.2.0", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compareVersions(tc.left, tc.right)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q vs %q", tc.left, tc.right)
				}
				return
			}
			if err != nil {
				t.Fatalf("compareVersions returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

func TestNormalizeUpdateChannel(t *testing.T) {
	t.Parallel()
	if got := normalizeUpdateChannel("stable"); got != UpdateChannelStable {
		t.Fatalf("stable channel mismatch: %q", got)
	}
	if got := normalizeUpdateChannel("ALPHA"); got != UpdateChannelAlpha {
		t.Fatalf("alpha channel mismatch: %q", got)
	}
	if got := normalizeUpdateChannel("unknown"); got != "" {
		t.Fatalf("expected empty channel for unknown, got %q", got)
	}
}
