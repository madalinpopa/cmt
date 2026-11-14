package commit

import (
	"context"
	"errors"
	"slices"
	"strings"
)

type Style int

const (
	Conventional Style = iota
	Plain
)

type Input struct {
	Diff    string
	Stat    string
	Commits []string
	Branch  string
	Hint    string
	Style   Style

	DiffTruncated bool
}

type Message struct {
	Type     string `json:"type"`
	Scope    string `json:"scope"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	Breaking bool   `json:"breaking"`
}

type Provider interface {
	Generate(ctx context.Context, p Prompt) (Message, error)
}

func Generate(ctx context.Context, p Provider, in Input) (Message, error) {
	msg, err := p.Generate(ctx, BuildPrompt(in))
	if err != nil {
		return Message{}, err
	}

	msg = msg.normalize()
	if err := msg.validate(in.Style); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (m Message) String() string {
	header := m.Subject
	if m.Type != "" {
		header = m.Type
		if m.Scope != "" {
			header += "(" + m.Scope + ")"
		}
		if m.Breaking {
			header += "!"
		}
		header += ": " + m.Subject
	}

	if m.Body == "" {
		return header
	}
	return header + "\n\n" + m.Body
}

func (m Message) normalize() Message {
	m.Type = strings.TrimSpace(m.Type)
	m.Scope = strings.TrimSpace(m.Scope)
	m.Subject = strings.TrimSpace(m.Subject)
	m.Body = wrapBody(strings.TrimSpace(m.Body))
	return m
}

func (m Message) validate(style Style) error {
	if m.Subject == "" {
		return errors.New("generated message has an empty subject")
	}
	if strings.ContainsAny(m.Subject, "\r\n") {
		return errors.New("generated subject spans multiple lines")
	}
	if style == Conventional && !slices.Contains(commitTypes(), m.Type) {
		return errors.New("generated message has an invalid type: " + m.Type)
	}
	return nil
}

func commitTypes() []string {
	return []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"}
}
