package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
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

type Changes struct {
	All   bool
	Paths []string
}

func (c Changes) againstHead() bool {
	return c.All || len(c.Paths) > 0
}

func (c Changes) pathspecs() []string {
	if len(c.Paths) > 0 {
		return slices.Clone(c.Paths)
	}
	return []string{":/"}
}

func (r Repo) Diff(ctx context.Context, c Changes, excludeGlobs ...string) (string, error) {
	pathspecs := append(c.pathspecs(),
		":(top,exclude,attr:linguist-generated)",
		":(top,exclude,attr:linguist-generated=true)",
	)
	for _, glob := range excludeGlobs {
		pathspecs = append(pathspecs, ":(top,exclude,glob)"+glob)
	}
	return r.diff(ctx, c, []string{"--no-ext-diff"}, pathspecs)
}

func (r Repo) Stat(ctx context.Context, c Changes) (string, error) {
	return r.diff(ctx, c, []string{"--stat"}, c.pathspecs())
}

func (r Repo) Snapshot(ctx context.Context, c Changes) (string, error) {
	out, err := r.diff(ctx, c, []string{"--no-ext-diff", "--binary", "--full-index"}, c.pathspecs())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(out))
	return hex.EncodeToString(sum[:]), nil
}

func (r Repo) Commit(ctx context.Context, message string, c Changes) (string, error) {
	out, err := r.runInput(ctx, strings.NewReader(message), commitArgs(c, "--file=-")...)
	return strings.TrimSpace(out), err
}

func (r Repo) CommitEdited(ctx context.Context, message string, c Changes, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	f, err := os.CreateTemp("", "cmt-*.txt")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(f.Name())) }()

	if _, err := f.WriteString(message); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}

	args := commitArgs(c, "--file="+f.Name(), "--edit")
	cmd := r.command(ctx, args)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return &Error{Args: args, Err: err}
	}
	return nil
}

func commitArgs(c Changes, flags ...string) []string {
	args := append([]string{"commit"}, flags...)
	if c.All {
		args = append(args, "--all")
	}
	if len(c.Paths) > 0 {
		args = append(args, "--only", "--")
		args = append(args, c.Paths...)
	}
	return args
}

func (r Repo) diff(ctx context.Context, c Changes, flags, pathspecs []string) (string, error) {
	args := []string{"diff", "--cached"}
	if c.againstHead() {
		base, err := r.headOrEmptyTree(ctx)
		if err != nil {
			return "", err
		}
		args = []string{"diff", base}
	}

	args = append(args, "--no-color", "--no-relative", "--no-textconv")
	args = append(args, flags...)
	args = append(args, "--")
	args = append(args, pathspecs...)
	return r.run(ctx, args...)
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
	cmd := r.command(ctx, args)
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.String(), nil
}

func (r Repo) command(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = r.Dir
	return cmd
}
