package commit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeProvider struct {
	msg    Message
	err    error
	prompt Prompt
}

func (f *fakeProvider) Generate(_ context.Context, p Prompt) (Message, error) {
	f.prompt = p
	return f.msg, f.err
}

func TestMessageString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{"type only", Message{Type: "fix", Subject: "handle empty diff"}, "fix: handle empty diff"},
		{"scope and body", Message{Type: "feat", Scope: "git", Subject: "add reader", Body: "Details."}, "feat(git): add reader\n\nDetails."},
		{"breaking", Message{Type: "refactor", Scope: "cli", Subject: "rename flag", Breaking: true}, "refactor(cli)!: rename flag"},
		{"plain", Message{Subject: "Add reader", Body: "Details."}, "Add reader\n\nDetails."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.msg.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	p := &fakeProvider{msg: Message{Type: "feat", Scope: " git ", Subject: " add reader \n", Body: "\nDetails.\n"}}

	got, err := Generate(t.Context(), p, Input{Diff: "diff", Style: Conventional})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := (Message{Type: "feat", Scope: "git", Subject: "add reader", Body: "Details."}); got != want {
		t.Errorf("Generate = %+v, want %+v", got, want)
	}
	if !strings.Contains(p.prompt.User, "<diff>\ndiff\n</diff>") || p.prompt.Style != Conventional {
		t.Errorf("prompt = %+v", p.prompt)
	}
}

func TestGenerateRejectsInvalidMessages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		msg   Message
		style Style
	}{
		{"empty subject", Message{Type: "feat", Subject: "  "}, Conventional},
		{"multiline subject", Message{Type: "feat", Subject: "a\nb"}, Conventional},
		{"unknown type", Message{Type: "feature", Subject: "add"}, Conventional},
		{"plain empty subject", Message{}, Plain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Generate(t.Context(), &fakeProvider{msg: tt.msg}, Input{Style: tt.style}); err == nil {
				t.Error("Generate: want error")
			}
		})
	}
}

func TestGenerateProviderError(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	if _, err := Generate(t.Context(), &fakeProvider{err: want}, Input{}); !errors.Is(err, want) {
		t.Errorf("Generate error = %v, want %v", err, want)
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Parallel()
	p := BuildPrompt(Input{
		Diff:    "+line",
		Stat:    "a.go | 1 +",
		Commits: []string{"feat: one", "fix: two"},
		Branch:  "main",
		Style:   Plain,
	})

	for _, want := range []string{"<branch>\nmain\n</branch>", "<recent_commits>\nfeat: one\nfix: two\n</recent_commits>", "<stat>\na.go | 1 +\n</stat>", "<diff>\n+line\n</diff>"} {
		if !strings.Contains(p.User, want) {
			t.Errorf("user prompt missing %q:\n%s", want, p.User)
		}
	}
	if strings.Contains(p.User, "<hint>") {
		t.Errorf("user prompt has empty hint section:\n%s", p.User)
	}
	if strings.Contains(p.System, "Conventional Commits") {
		t.Error("plain system prompt mentions Conventional Commits")
	}
}

func TestStyleSchema(t *testing.T) {
	t.Parallel()
	conventional := Conventional.Schema()["properties"].(map[string]any)
	if _, ok := conventional["type"]; !ok {
		t.Error("conventional schema has no type property")
	}
	plain := Plain.Schema()["properties"].(map[string]any)
	if _, ok := plain["type"]; ok {
		t.Error("plain schema has a type property")
	}
}
