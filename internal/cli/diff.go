package cli

import (
	"fmt"
	"iter"
	"strconv"
	"strings"
	"unicode/utf8"
)

const defaultMaxDiffBytes = 200_000

func excludedGlobs() []string {
	return []string{
		"**/go.sum",
		"**/go.work.sum",
		"**/package-lock.json",
		"**/npm-shrinkwrap.json",
		"**/yarn.lock",
		"**/pnpm-lock.yaml",
		"**/bun.lock",
		"**/bun.lockb",
		"**/Cargo.lock",
		"**/Gemfile.lock",
		"**/composer.lock",
		"**/poetry.lock",
		"**/uv.lock",
		"**/Pipfile.lock",
		"**/pubspec.lock",
		"**/mix.lock",
		"**/Podfile.lock",
		"**/flake.lock",
		"**/*.pb.go",
		"**/*.min.js",
		"**/*.min.css",
	}
}

func parseMaxDiffBytes(value string) (int, error) {
	if value == "" {
		return defaultMaxDiffBytes, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("CMT_MAX_DIFF_BYTES must be a positive integer, got %q", value)
	}
	return n, nil
}

func truncateDiff(diff string, limit int) (string, bool) {
	if len(diff) <= limit {
		return diff, false
	}

	var b strings.Builder
	for file := range diffFiles(diff) {
		if b.Len()+len(file) > limit {
			continue
		}
		b.WriteString(file)
	}
	if b.Len() > 0 {
		return b.String(), true
	}

	end := limit
	for end > 0 && !utf8.RuneStart(diff[end]) {
		end--
	}
	cut := diff[:end]
	if i := strings.LastIndexByte(cut, '\n'); i >= 0 {
		cut = cut[:i+1]
	}
	return cut, true
}

func diffFiles(diff string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := 0
		offset := 0
		for line := range strings.Lines(diff) {
			if offset > start && strings.HasPrefix(line, "diff --git ") {
				if !yield(diff[start:offset]) {
					return
				}
				start = offset
			}
			offset += len(line)
		}
		if start < len(diff) {
			yield(diff[start:])
		}
	}
}
