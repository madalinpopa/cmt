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
		newProvider: func(string) commit.Provider { return p },
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
