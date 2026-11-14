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

	for _, c := range []Changes{Staged, Tracked} {
		diff, err := r.Diff(ctx, c)
		if err != nil || diff != "" {
			t.Fatalf("Diff(%v) = %q, %v; want empty", c, diff, err)
		}
	}
}

func setupMixedChanges(t *testing.T) Repo {
	t.Helper()
	r := newRepo(t)
	commitFile(t, r, "a.txt", "one\n", "feat: add a")

	writeFile(t, r.Dir, "a.txt", "one\nstaged\n")
	gitCmd(t, r.Dir, "add", "a.txt")
	writeFile(t, r.Dir, "a.txt", "one\nstaged\nunstaged\n")
	writeFile(t, r.Dir, "untracked.txt", "new\n")
	return r
}

func TestDiffStaged(t *testing.T) {
	t.Parallel()
	r := setupMixedChanges(t)
	ctx := t.Context()

	diff, err := r.Diff(ctx, Staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+staged") || strings.Contains(diff, "unstaged") {
		t.Errorf("Diff(Staged) = %q; want only staged changes", diff)
	}

	stat, err := r.Stat(ctx, Staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stat, "a.txt") || !strings.Contains(stat, "1 insertion") {
		t.Errorf("Stat(Staged) = %q", stat)
	}
}

func TestDiffTracked(t *testing.T) {
	t.Parallel()
	r := setupMixedChanges(t)
	ctx := t.Context()

	diff, err := r.Diff(ctx, Tracked)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+staged") || !strings.Contains(diff, "+unstaged") || strings.Contains(diff, "untracked.txt") {
		t.Errorf("Diff(Tracked) = %q; want staged and unstaged tracked changes only", diff)
	}

	stat, err := r.Stat(ctx, Tracked)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stat, "2 insertions") {
		t.Errorf("Stat(Tracked) = %q", stat)
	}
}

func TestDiffExcludes(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	if err := os.Mkdir(filepath.Join(r.Dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, r.Dir, ".gitattributes", "gen.go linguist-generated\ngen2.go linguist-generated=true\n")
	for _, name := range []string{"go.sum", "sub/go.sum", "gen.go", "gen2.go", "sub/keep.txt"} {
		writeFile(t, r.Dir, name, name+"\n")
	}
	gitCmd(t, r.Dir, "add", "-A")

	for _, dir := range []string{r.Dir, filepath.Join(r.Dir, "sub")} {
		diff, err := Repo{Dir: dir}.Diff(t.Context(), Staged, "**/go.sum")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diff, "+++ b/sub/keep.txt") || !strings.Contains(diff, "+++ b/.gitattributes") {
			t.Errorf("Diff from %s is missing kept files:\n%s", dir, diff)
		}
		for _, excluded := range []string{"go.sum", "sub/go.sum", "gen.go", "gen2.go"} {
			if strings.Contains(diff, "+++ b/"+excluded) {
				t.Errorf("Diff from %s contains %s:\n%s", dir, excluded, diff)
			}
		}
	}

	stat, err := r.Stat(t.Context(), Staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stat, "go.sum") || !strings.Contains(stat, "gen.go") {
		t.Errorf("Stat should list excluded files:\n%s", stat)
	}
}

func TestDiffTrackedWithoutCommits(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	writeFile(t, r.Dir, "a.txt", "one\n")
	gitCmd(t, r.Dir, "add", "a.txt")
	writeFile(t, r.Dir, "a.txt", "one\ntwo\n")

	diff, err := r.Diff(t.Context(), Tracked)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+one") || !strings.Contains(diff, "+two") {
		t.Errorf("Diff(Tracked) = %q", diff)
	}
}

func lastCommit(t *testing.T, r Repo) (message, files string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "log", "-1", "--format=%B")
	cmd.Dir = r.Dir
	msg, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.CommandContext(t.Context(), "git", "show", "--format=", "HEAD")
	cmd.Dir = r.Dir
	diff, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(msg)), string(diff)
}

func TestCommit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		changes      Changes
		wantUnstaged bool
	}{
		{Staged, false},
		{Tracked, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.changes), func(t *testing.T) {
			t.Parallel()
			r := setupMixedChanges(t)
			message := "feat(a): extend a\n\nAdd more lines to a."

			out, err := r.Commit(t.Context(), message, tt.changes)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "feat(a): extend a") {
				t.Errorf("Commit output = %q", out)
			}

			gotMsg, diff := lastCommit(t, r)
			if gotMsg != message {
				t.Errorf("commit message = %q, want %q", gotMsg, message)
			}
			if strings.Contains(diff, "+unstaged") != tt.wantUnstaged || strings.Contains(diff, "untracked.txt") {
				t.Errorf("committed diff = %q", diff)
			}
		})
	}
}

func TestCommitNothingStaged(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	commitFile(t, r, "a.txt", "a\n", "feat: add a")

	if _, err := r.Commit(t.Context(), "fix: nothing", Staged); err == nil {
		t.Error("Commit: want error")
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
