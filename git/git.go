package git

import (
	"bytes"
	"context"
	"errors"
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

func (r Repo) StagedDiff(ctx context.Context) (string, error) {
	return r.run(ctx, "diff", "--cached", "--no-color", "--no-ext-diff")
}

func (r Repo) StagedStat(ctx context.Context) (string, error) {
	return r.run(ctx, "diff", "--cached", "--no-color", "--stat")
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
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = r.Dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}
