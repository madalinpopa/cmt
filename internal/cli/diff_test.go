package cli

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	fileA = "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -0,0 +1 @@\n+aaaa\n"
	fileB = "diff --git a/b.txt b/b.txt\n--- a/b.txt\n+++ b/b.txt\n@@ -0,0 +1 @@\n+bbbb\n"
)

func TestDiffFiles(t *testing.T) {
	t.Parallel()
	got := slices.Collect(diffFiles(fileA + fileB))
	if want := []string{fileA, fileB}; !slices.Equal(got, want) {
		t.Errorf("diffFiles = %q, want %q", got, want)
	}
	if got := slices.Collect(diffFiles("")); got != nil {
		t.Errorf("diffFiles(\"\") = %q, want nil", got)
	}
}

func TestTruncateDiff(t *testing.T) {
	t.Parallel()
	diff := fileA + fileB
	tests := []struct {
		name          string
		limit         int
		want          string
		wantTruncated bool
	}{
		{"fits", len(diff), diff, false},
		{"keeps whole files", len(diff) - 1, fileA, true},
		{"cuts first file at a line", len(fileA) - 1, strings.TrimSuffix(fileA, "+aaaa\n"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, truncated := truncateDiff(diff, tt.limit)
			if got != tt.want || truncated != tt.wantTruncated {
				t.Errorf("truncateDiff = %q, %v; want %q, %v", got, truncated, tt.want, tt.wantTruncated)
			}
			if len(got) > tt.limit {
				t.Errorf("len = %d, over limit %d", len(got), tt.limit)
			}
		})
	}
}

func TestTruncateDiffSkipsOversizedFile(t *testing.T) {
	t.Parallel()
	large := "diff --git a/large.txt b/large.txt\n+" + strings.Repeat("x", 100) + "\n"
	got, truncated := truncateDiff(fileA+large+fileB, len(fileA)+len(fileB))
	if want := fileA + fileB; got != want || !truncated {
		t.Errorf("truncateDiff = %q, %v; want %q, true", got, truncated, want)
	}
}

func TestTruncateDiffKeepsValidUTF8(t *testing.T) {
	t.Parallel()
	diff := "+" + strings.Repeat("é", 10)
	for limit := 1; limit < len(diff); limit++ {
		got, _ := truncateDiff(diff, limit)
		if !utf8.ValidString(got) || len(got) > limit {
			t.Errorf("truncateDiff(limit=%d) = %q; want valid UTF-8 within limit", limit, got)
		}
	}
}

func TestParseMaxDiffBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value   string
		want    int
		wantErr bool
	}{
		{"", defaultMaxDiffBytes, false},
		{"5000", 5000, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"lots", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			got, err := parseMaxDiffBytes(tt.value)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Errorf("parseMaxDiffBytes(%q) = %d, %v", tt.value, got, err)
			}
		})
	}
}
