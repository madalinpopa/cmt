package cli

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/madalinpopa/cmt/command"
	"github.com/madalinpopa/cmt/commit"
	"github.com/madalinpopa/cmt/vcs"
)

func TestAskChoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		want      choice
		wantErr   error
		wantRetry bool
	}{
		{"yes", "y\n", choiceCommit, nil, false},
		{"edit uppercase", "E\n", choiceEdit, nil, false},
		{"regenerate word", "regenerate\n", choiceRegenerate, nil, false},
		{"no", "n\n", choiceCancel, nil, false},
		{"answer without newline", "y", choiceCommit, nil, false},
		{"retries unknown answer", "x\nn\n", choiceCancel, nil, true},
		{"empty input", "", 0, errNoAnswer, false},
		{"unknown then eof", "x\n", 0, errNoAnswer, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			got, err := askChoice(bufio.NewReader(strings.NewReader(tt.input)), &out)
			if got != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("askChoice = %v, %v; want %v, %v", got, err, tt.want, tt.wantErr)
			}
			if retried := strings.Contains(out.String(), "Please answer"); retried != tt.wantRetry {
				t.Errorf("retry prompt shown = %v, want %v:\n%s", retried, tt.wantRetry, out.String())
			}
		})
	}
}

func TestConfirmRegenerateThenCancel(t *testing.T) {
	t.Parallel()
	cmd := &command.Command{}
	cmd.SetIn(strings.NewReader("r\nn\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	calls := 0
	regenerate := func() (draft, error) {
		calls++
		return draft{msg: commit.Message{Type: "fix", Subject: "second try"}}, nil
	}

	first := draft{msg: commit.Message{Type: "feat", Subject: "first try"}}
	if err := confirmAndCommit(t.Context(), cmd, vcs.Repo{}, vcs.Changes{}, first, regenerate); err != nil {
		t.Fatalf("confirmAndCommit: %v", err)
	}
	if calls != 1 {
		t.Errorf("regenerate calls = %d, want 1", calls)
	}
	for _, want := range []string{"feat: first try", "fix: second try", "Canceled."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestConfirmRegenerateError(t *testing.T) {
	t.Parallel()
	cmd := &command.Command{}
	cmd.SetIn(strings.NewReader("r\n"))
	cmd.SetOut(&bytes.Buffer{})

	want := errors.New("boom")
	regenerate := func() (draft, error) { return draft{}, want }

	err := confirmAndCommit(t.Context(), cmd, vcs.Repo{}, vcs.Changes{}, draft{msg: commit.Message{Subject: "x"}}, regenerate)
	if !errors.Is(err, want) {
		t.Errorf("confirmAndCommit error = %v, want %v", err, want)
	}
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()
	if isTerminal(strings.NewReader("")) {
		t.Error("strings.Reader reported as terminal")
	}

	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if isTerminal(f) {
		t.Error("regular file reported as terminal")
	}
}

func TestRunRootRequiresTerminal(t *testing.T) {
	t.Parallel()
	cmd := &command.Command{}
	cmd.SetIn(strings.NewReader(""))

	err := defaultRunner().run(t.Context(), cmd, rootOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("runRoot error = %v", err)
	}
}

func TestRunRootRejectsAllWithPaths(t *testing.T) {
	t.Parallel()
	err := defaultRunner().run(t.Context(), &command.Command{}, rootOptions{all: true, dryRun: true}, []string{"a.txt"})
	if _, ok := errors.AsType[*command.UsageError](err); !ok {
		t.Errorf("runRoot error = %v; want *command.UsageError", err)
	}
}
