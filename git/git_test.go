package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

func newRepo(t *testing.T) Repo {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "--initial-branch=main")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	return Repo{Dir: dir}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitFile(t *testing.T, r Repo, name, content, subject string) {
	t.Helper()
	writeFile(t, r.Dir, name, content)
	gitCmd(t, r.Dir, "add", name)
	gitCmd(t, r.Dir, "commit", "-m", subject)
}

func TestEmptyRepo(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	ctx := t.Context()

	branch, err := r.CurrentBranch(ctx)
	if err != nil || branch != "main" {
		t.Fatalf("CurrentBranch = %q, %v; want main", branch, err)
	}

	commits, err := r.RecentCommits(ctx, 5)
	if err != nil || commits != nil {
		t.Fatalf("RecentCommits = %q, %v; want nil", commits, err)
	}

	diff, err := r.StagedDiff(ctx)
	if err != nil || diff != "" {
		t.Fatalf("StagedDiff = %q, %v; want empty", diff, err)
	}
}

func TestStagedChangesOnly(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	ctx := t.Context()
	commitFile(t, r, "a.txt", "one\n", "feat: add a")

	writeFile(t, r.Dir, "a.txt", "one\nstaged\n")
	gitCmd(t, r.Dir, "add", "a.txt")
	writeFile(t, r.Dir, "a.txt", "one\nstaged\nunstaged\n")

	diff, err := r.StagedDiff(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+staged") || strings.Contains(diff, "unstaged") {
		t.Errorf("StagedDiff = %q; want only staged changes", diff)
	}

	stat, err := r.StagedStat(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stat, "a.txt") || !strings.Contains(stat, "1 insertion") {
		t.Errorf("StagedStat = %q", stat)
	}
}

func TestRecentCommits(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	ctx := t.Context()
	commitFile(t, r, "a.txt", "a\n", "feat: add a")
	commitFile(t, r, "b.txt", "b\n", "fix: add b")
	commitFile(t, r, "c.txt", "c\n", "docs: add c")

	got, err := r.RecentCommits(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs: add c", "fix: add b"}; !slices.Equal(got, want) {
		t.Errorf("RecentCommits = %q; want %q", got, want)
	}
}

func TestDetachedHead(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	commitFile(t, r, "a.txt", "a\n", "feat: add a")
	gitCmd(t, r.Dir, "checkout", "--detach")

	branch, err := r.CurrentBranch(t.Context())
	if err != nil || branch != "" {
		t.Fatalf("CurrentBranch = %q, %v; want empty", branch, err)
	}
}

func TestNotARepository(t *testing.T) {
	t.Parallel()
	r := Repo{Dir: t.TempDir()}

	_, err := r.RecentCommits(t.Context(), 1)
	gitErr, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("RecentCommits error = %v; want *Error", err)
	}
	if !strings.Contains(gitErr.Stderr, "not a git repository") {
		t.Errorf("Stderr = %q", gitErr.Stderr)
	}
}
