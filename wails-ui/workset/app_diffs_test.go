package main

import "testing"

func TestParseNameStatusZ(t *testing.T) {
	input := []byte("M\x00readme.md\x00R100\x00old.txt\x00new.txt\x00")
	entries := parseNameStatusZ(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].status != "M" || entries[0].path != "readme.md" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].status != "R100" || entries[1].prevPath != "old.txt" || entries[1].path != "new.txt" {
		t.Fatalf("unexpected rename entry: %+v", entries[1])
	}
}

func TestParseNumstatZ(t *testing.T) {
	input := []byte("2\t1\treadme.md\x00-\t-\tbin.dat\x003\t2\told.txt\x00new.txt\x00")
	entries := parseNumstatZ(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].path != "readme.md" || entries[0].added != 2 || entries[0].removed != 1 {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if !entries[1].binary || entries[1].path != "bin.dat" {
		t.Fatalf("unexpected binary entry: %+v", entries[1])
	}
	if entries[2].prevPath != "old.txt" || entries[2].path != "new.txt" {
		t.Fatalf("unexpected rename entry: %+v", entries[2])
	}
}

func TestFinalizePatch(t *testing.T) {
	patch := "line\n"
	result := finalizePatch(patch)
	if result.Truncated {
		t.Fatalf("did not expect truncation")
	}
	if result.Patch != patch {
		t.Fatalf("expected patch to be preserved")
	}

	largePatch := make([]byte, maxDiffBytes+1)
	for i := range largePatch {
		largePatch[i] = 'a'
	}
	largeResult := finalizePatch(string(largePatch))
	if !largeResult.Truncated {
		t.Fatalf("expected truncation for large patch")
	}
	if largeResult.Patch != "" {
		t.Fatalf("expected truncated patch to be empty")
	}
}
