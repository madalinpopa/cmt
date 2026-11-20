package command

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func newTestCommand(got *[]string) (*Command, *bool, *string) {
	c := &Command{
		Use:     "cmt [flags]",
		Short:   "Generate commit messages",
		Example: "cmt -a\ncmt --dry-run",
		Run: func(_ context.Context, _ *Command, args []string) error {
			*got = args
			return nil
		},
	}
	all := c.Flags().Bool("all", false, "stage all modified files")
	msg := c.Flags().String("context", "", "hint for the model")
	return c, all, msg
}

func TestExecuteFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantPos []string
		wantAll bool
		wantCtx string
	}{
		{"no args", nil, nil, false, ""},
		{"flags only", []string{"--all", "-context", "x"}, nil, true, "x"},
		{"flags then positional", []string{"--all", "--context=y", "a", "b"}, []string{"a", "b"}, true, "y"},
		{"flags after positional", []string{"a", "--all", "b", "--context", "x"}, []string{"a", "b"}, true, "x"},
		{"terminator", []string{"--all", "--", "--context", "z"}, []string{"--context", "z"}, true, ""},
		{"terminator after positional", []string{"a", "--", "--all"}, []string{"a", "--all"}, false, ""},
		{"single dash is positional", []string{"-", "--all"}, []string{"-"}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			c, all, msg := newTestCommand(&got)
			if err := c.Execute(t.Context(), tt.args); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !slices.Equal(got, tt.wantPos) {
				t.Errorf("positional = %q, want %q", got, tt.wantPos)
			}
			if *all != tt.wantAll || *msg != tt.wantCtx {
				t.Errorf("all=%v context=%q, want %v %q", *all, *msg, tt.wantAll, tt.wantCtx)
			}
		})
	}
}

func TestExecuteHelp(t *testing.T) {
	t.Parallel()
	var got []string
	c, _, _ := newTestCommand(&got)
	var out bytes.Buffer
	c.SetOut(&out)

	if err := c.Execute(t.Context(), []string{"--help"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != nil {
		t.Error("Run called on --help")
	}
	for _, want := range []string{"Generate commit messages", "Usage:\n  cmt [flags]", "-all", "-context", "Examples:\n  cmt -a\n  cmt --dry-run"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("usage missing %q:\n%s", want, out.String())
		}
	}
}

func TestExecuteUsageErrors(t *testing.T) {
	t.Parallel()
	var got []string
	c, _, _ := newTestCommand(&got)
	if _, ok := errors.AsType[*UsageError](c.Execute(t.Context(), []string{"--nope"})); !ok {
		t.Error("unknown flag: want *UsageError")
	}
}

func TestExecuteNilRunPrintsUsage(t *testing.T) {
	t.Parallel()
	c := &Command{Use: "cmt"}
	var out bytes.Buffer
	c.SetOut(&out)
	if err := c.Execute(t.Context(), nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "Usage:\n  cmt") {
		t.Errorf("usage = %q", out.String())
	}
}
