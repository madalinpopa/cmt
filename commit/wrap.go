package commit

import (
	"strings"
	"unicode/utf8"
)

const bodyWidth = 72

func wrapBody(body string) string {
	var lines []string
	for line := range strings.SplitSeq(body, "\n") {
		lines = append(lines, wrapLine(strings.TrimRight(line, " \t\r")))
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string) string {
	if utf8.RuneCountInString(line) <= bodyWidth || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return line
	}

	indent := listIndent(line)
	var lines []string
	current := ""
	for word := range strings.FieldsSeq(line) {
		switch {
		case current == "":
			current = word
		case utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) > bodyWidth:
			lines = append(lines, current)
			current = indent + word
		default:
			current += " " + word
		}
	}
	return strings.Join(append(lines, current), "\n")
}

func listIndent(line string) string {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return "  "
	}

	digits := len(line) - len(strings.TrimLeft(line, "0123456789"))
	if digits > 0 && strings.HasPrefix(line[digits:], ". ") {
		return strings.Repeat(" ", digits+2)
	}
	return ""
}
