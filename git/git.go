package git

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type Repo struct {
	Dir string
}

type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := "git " + strings.Join(e.Args, " ") + ": " + e.Err.Error()
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error {
	return e.Err
}

type Changes int

const (
	Staged Changes = iota
	Tracked
)

func (r Repo) Diff(ctx context.Context, c Changes) (string, error) {
	return r.diff(ctx, c, "--no-color", "--no-ext-diff")
}

func (r Repo) Stat(ctx context.Context, c Changes) (string, error) {
	return r.diff(ctx, c, "--no-color", "--stat")
}

func (r Repo) Commit(ctx context.Context, message string, c Changes) (string, error) {
	args := []string{"commit", "--file=-"}
	if c == Tracked {
		args = append(args, "--all")
	}
	out, err := r.runInput(ctx, strings.NewReader(message), args...)
	return strings.TrimSpace(out), err
}

func (r Repo) diff(ctx context.Context, c Changes, flags ...string) (string, error) {
	if c == Staged {
		return r.run(ctx, append([]string{"diff", "--cached"}, flags...)...)
	}

	base, err := r.headOrEmptyTree(ctx)
	if err != nil {
		return "", err
	}
	return r.run(ctx, append([]string{"diff", base}, flags...)...)
}

func (r Repo) headOrEmptyTree(ctx context.Context) (string, error) {
	hasCommits, err := r.hasCommits(ctx)
	if err != nil {
		return "", err
	}
	if hasCommits {
		return "HEAD", nil
	}

	out, err := r.runInput(ctx, strings.NewReader(""), "hash-object", "-t", "tree", "--stdin")
	return strings.TrimSpace(out), err
}

func (r Repo) CurrentBranch(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "branch", "--show-current")
	return strings.TrimSpace(out), err
}

func (r Repo) RecentCommits(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}

	hasCommits, err := r.hasCommits(ctx)
	if err != nil || !hasCommits {
		return nil, err
	}

	out, err := r.run(ctx, "log", "-n", strconv.Itoa(n), "--no-merges", "--no-color", "--format=%s")
	if err != nil {
		return nil, err
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (r Repo) hasCommits(ctx context.Context) (bool, error) {
	_, err := r.run(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

func (r Repo) run(ctx context.Context, args ...string) (string, error) {
	return r.runInput(ctx, nil, args...)
}

func (r Repo) runInput(ctx context.Context, stdin io.Reader, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = r.Dir
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}
