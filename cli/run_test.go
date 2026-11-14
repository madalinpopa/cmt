package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madalinpopa/cmt/command"
	"github.com/madalinpopa/cmt/commit"
	"github.com/madalinpopa/cmt/git"
)

func TestMain(m *testing.M) {
	env := map[string]string{
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CEILING_DIRECTORIES": os.TempDir(),
	}
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	m.Run()
}

type fakeProvider struct {
	msg    commit.Message
	prompt commit.Prompt
}

func (f *fakeProvider) Generate(_ context.Context, p commit.Prompt) (commit.Message, error) {
	f.prompt = p
	return f.msg, nil
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "--initial-branch=main")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	writeFile(t, dir, "a.txt", "a\n")
	writeFile(t, dir, "b.txt", "b\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "feat: add files")
	return dir
}

func newTestRunner(dir string, p *fakeProvider, env map[string]string) runner {
	return runner{
		repo:        git.Repo{Dir: dir},
		newProvider: func(string, string) (commit.Provider, error) { return p, nil },
		getenv:      func(key string) string { return env[key] },
	}
}

func TestDryRunWritesOnlyMessageToStdout(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	writeFile(t, dir, "a.txt", "a\n"+strings.Repeat("more a\n", 20))
	writeFile(t, dir, "b.txt", "b\nmore b\n")
	gitCmd(t, dir, "add", "-A")

	p := &fakeProvider{msg: commit.Message{Type: "feat", Scope: "a", Subject: "extend files", Body: "Add lines."}}
	cmd := &command.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	r := newTestRunner(dir, p, map[string]string{"CMT_MAX_DIFF_BYTES": "300"})
	if err := r.run(t.Context(), cmd, rootOptions{dryRun: true}, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if want := "feat(a): extend files\n\nAdd lines.\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if !strings.Contains(stderr.String(), "warning: diff is larger than 300 bytes") {
		t.Errorf("stderr = %q; want truncation warning", stderr.String())
	}
	if !strings.Contains(p.prompt.User, "<recent_commits>\nfeat: add files") || !strings.Contains(p.prompt.User, "<note>") {
		t.Errorf("prompt is missing context:\n%s", p.prompt.User)
	}
	if log := gitCmd(t, dir, "log", "--format=%s"); strings.TrimSpace(log) != "feat: add files" {
		t.Errorf("dry run created a commit: %q", log)
	}
}

func TestYesCommitsOnlyGivenPaths(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	writeFile(t, dir, "a.txt", "a\nstaged a\n")
	gitCmd(t, dir, "add", "a.txt")
	writeFile(t, dir, "b.txt", "b\nchanged b\n")

	p := &fakeProvider{msg: commit.Message{Type: "fix", Subject: "change b"}}
	cmd := &command.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	r := newTestRunner(dir, p, nil)
	if err := r.run(t.Context(), cmd, rootOptions{yes: true}, []string{"b.txt"}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(p.prompt.User, "+changed b") || strings.Contains(p.prompt.User, "staged a") {
		t.Errorf("prompt diff should only cover b.txt:\n%s", p.prompt.User)
	}
	if !strings.Contains(stdout.String(), "fix: change b") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if files := gitCmd(t, dir, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(files) != "b.txt" {
		t.Errorf("committed files = %q, want b.txt", files)
	}
	if staged := gitCmd(t, dir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "a.txt" {
		t.Errorf("staged files = %q, want a.txt", staged)
	}
}

type modifyingProvider struct {
	fakeProvider
	calls  int
	modify func()
}

func (m *modifyingProvider) Generate(ctx context.Context, p commit.Prompt) (commit.Message, error) {
	m.calls++
	if m.calls == 1 {
		m.modify()
	}
	return m.fakeProvider.Generate(ctx, p)
}

func TestYesRejectsChangesModifiedDuringGeneration(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	writeFile(t, dir, "a.txt", "a\nfirst\n")

	p := &modifyingProvider{
		msg:    commit.Message{Type: "fix", Subject: "change a"},
		modify: func() { writeFile(t, dir, "a.txt", "a\nsecond\n") },
	}
	cmd := &command.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	r := newTestRunner(dir, nil, nil)
	r.newProvider = func(string, string) (commit.Provider, error) { return p, nil }
	if err := r.run(t.Context(), cmd, rootOptions{yes: true, all: true}, nil); !errors.Is(err, errChangesModified) {
		t.Fatalf("run error = %v, want %v", err, errChangesModified)
	}
	if log := gitCmd(t, dir, "log", "--format=%s"); strings.TrimSpace(log) != "feat: add files" {
		t.Errorf("commit was created: %q", log)
	}
}

func TestConfirmRegeneratesWhenChangesModified(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	writeFile(t, dir, "a.txt", "a\nfirst\n")
	repo := git.Repo{Dir: dir}
	changes := git.Changes{All: true}

	snapshot, err := repo.Snapshot(t.Context(), changes)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "a\nsecond\n")

	calls := 0
	regenerate := func() (draft, error) {
		calls++
		snapshot, err := repo.Snapshot(t.Context(), changes)
		return draft{msg: commit.Message{Type: "fix", Subject: "second"}, snapshot: snapshot}, err
	}

	cmd := &command.Command{}
	cmd.SetIn(strings.NewReader("y\ny\n"))
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	first := draft{msg: commit.Message{Type: "fix", Subject: "first"}, snapshot: snapshot}
	if err := confirmAndCommit(t.Context(), cmd, repo, changes, first, regenerate); err != nil {
		t.Fatalf("confirmAndCommit: %v", err)
	}
	if calls != 1 {
		t.Errorf("regenerate calls = %d, want 1", calls)
	}
	if !strings.Contains(errOut.String(), "regenerating") {
		t.Errorf("stderr = %q; want regeneration notice", errOut.String())
	}
	if log := gitCmd(t, dir, "log", "-1", "--format=%s"); strings.TrimSpace(log) != "fix: second" {
		t.Errorf("last commit = %q, want fix: second", log)
	}
}

func TestRunNothingToCommit(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	cmd := &command.Command{}
	cmd.SetOut(&bytes.Buffer{})

	err := newTestRunner(dir, &fakeProvider{}, nil).run(t.Context(), cmd, rootOptions{dryRun: true}, nil)
	if !errors.Is(err, errNothingToCommit) {
		t.Errorf("run error = %v, want %v", err, errNothingToCommit)
	}
}

type closingProvider struct {
	fakeProvider
	closed bool
}

func (c *closingProvider) Close() error {
	c.closed = true
	return nil
}

func TestRunClosesProvider(t *testing.T) {
	t.Parallel()
	dir := newTestRepo(t)
	writeFile(t, dir, "a.txt", "a\nchanged\n")
	gitCmd(t, dir, "add", "a.txt")

	p := &closingProvider{msg: commit.Message{Type: "fix", Subject: "change a"}}
	r := newTestRunner(dir, nil, nil)
	r.newProvider = func(string, string) (commit.Provider, error) { return p, nil }

	cmd := &command.Command{}
	cmd.SetOut(&bytes.Buffer{})
	if err := r.run(t.Context(), cmd, rootOptions{dryRun: true}, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !p.closed {
		t.Error("provider was not closed")
	}
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "anthropic"} {
		if _, err := newProvider(name, ""); err != nil {
			t.Errorf("newProvider(%q): %v", name, err)
		}
	}
	_, err := newProvider("openai", "")
	if _, ok := errors.AsType[*command.UsageError](err); !ok {
		t.Errorf("newProvider(openai) error = %v; want *command.UsageError", err)
	}
}
