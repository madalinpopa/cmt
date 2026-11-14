package commit

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapBody(t *testing.T) {
	t.Parallel()
	long := "Increase the number of recent commit subjects sent as context for message generation from 10 to 20, giving the model more of the project's commit style to follow."
	url := "https://example.com/" + strings.Repeat("a", 80)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"short lines unchanged",
			"First line.\n\nSecond paragraph.",
			"First line.\n\nSecond paragraph.",
		},
		{
			"long paragraph",
			long,
			"Increase the number of recent commit subjects sent as context for\nmessage generation from 10 to 20, giving the model more of the project's\ncommit style to follow.",
		},
		{
			"bullet continuation is indented",
			"- " + long,
			"- Increase the number of recent commit subjects sent as context for\n  message generation from 10 to 20, giving the model more of the\n  project's commit style to follow.",
		},
		{
			"numbered continuation is indented",
			"12. " + long,
			"12. Increase the number of recent commit subjects sent as context for\n    message generation from 10 to 20, giving the model more of the\n    project's commit style to follow.",
		},
		{
			"long word stays whole",
			"See " + url + " for details.",
			"See\n" + url + "\nfor details.",
		},
		{
			"indented line untouched",
			"    " + long,
			"    " + long,
		},
		{
			"trailing spaces trimmed",
			"Line.  \r\nNext.",
			"Line.\nNext.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wrapBody(tt.body); got != tt.want {
				t.Errorf("wrapBody =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestWrapBodyKeepsWidthAndWords(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("ăîșț word ", 40)
	got := wrapBody(body)

	for line := range strings.SplitSeq(got, "\n") {
		if n := utf8.RuneCountInString(line); n > bodyWidth {
			t.Errorf("line has %d runes, over %d: %q", n, bodyWidth, line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != strings.Join(strings.Fields(body), " ") {
		t.Error("wrapBody changed the words")
	}
}
