package cli

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/madalinpopa/cmt/command"
	"github.com/madalinpopa/cmt/commit"
	"github.com/madalinpopa/cmt/git"
	"github.com/madalinpopa/cmt/provider/anthropic"
	"github.com/madalinpopa/cmt/provider/copilot"
)

const recentCommitCount = 10

var errNothingToCommit = errors.New("nothing to commit; stage changes with git add, use --all, or pass paths")

type runner struct {
	repo        git.Repo
	newProvider func(name, model string) (commit.Provider, error)
	getenv      func(key string) string
}

func defaultRunner() runner {
	return runner{
		newProvider: newProvider,
		getenv:      os.Getenv,
	}
}

func newProvider(name, model string) (commit.Provider, error) {
	switch name {
	case "", "anthropic":
		return anthropic.New(cmp.Or(model, anthropic.DefaultModel)), nil
	case "copilot":
		return copilot.New(cmp.Or(model, copilot.DefaultModel))
	default:
		return nil, &command.UsageError{Message: fmt.Sprintf("unknown provider %q; use anthropic or copilot", name)}
	}
}

func (r runner) run(ctx context.Context, cmd *command.Command, opts rootOptions, args []string) (err error) {
	if opts.all && len(args) > 0 {
		return &command.UsageError{Message: "--all cannot be combined with paths"}
	}
	interactive := !opts.dryRun && !opts.yes
	if interactive && !isTerminal(cmd.In()) {
		return errors.New("stdin is not a terminal; use --yes to commit or --dry-run to only print the message")
	}

	maxDiffBytes, err := parseMaxDiffBytes(r.getenv("CMT_MAX_DIFF_BYTES"))
	if err != nil {
		return err
	}

	provider, err := r.newProvider(opts.provider, opts.model)
	if err != nil {
		return err
	}
	if c, ok := provider.(io.Closer); ok {
		defer func() { err = errors.Join(err, c.Close()) }()
	}

	repo := r.repo
	changes := git.Changes{All: opts.all, Paths: args}

	in, err := collectInput(ctx, repo, changes, maxDiffBytes, opts)
	if err != nil {
		return err
	}
	if in.DiffTruncated {
		if _, err := fmt.Fprintf(cmd.Err(), "warning: diff is larger than %d bytes (CMT_MAX_DIFF_BYTES); sending a partial diff\n", maxDiffBytes); err != nil {
			return err
		}
	}

	generate := func() (commit.Message, error) {
		return commit.Generate(ctx, provider, in)
	}

	msg, err := generate()
	if err != nil {
		return err
	}

	switch {
	case opts.dryRun:
		_, err = fmt.Fprintln(cmd.Out(), msg)
		return err
	case opts.yes:
		return commitMessage(ctx, cmd, repo, msg.String(), changes)
	default:
		return confirmAndCommit(ctx, cmd, repo, changes, msg, generate)
	}
}

func confirmAndCommit(ctx context.Context, cmd *command.Command, repo git.Repo, changes git.Changes, msg commit.Message, regenerate func() (commit.Message, error)) error {
	answers := bufio.NewReader(cmd.In())
	for {
		if _, err := fmt.Fprintf(cmd.Out(), "%s\n\n", msg); err != nil {
			return err
		}

		c, err := askChoice(answers, cmd.Out())
		if err != nil {
			return err
		}

		switch c {
		case choiceCommit:
			return commitMessage(ctx, cmd, repo, msg.String(), changes)
		case choiceEdit:
			return repo.CommitEdited(ctx, msg.String(), changes, cmd.In(), cmd.Out(), cmd.Err())
		case choiceCancel:
			_, err := fmt.Fprintln(cmd.Out(), "Canceled.")
			return err
		case choiceRegenerate:
			if msg, err = regenerate(); err != nil {
				return err
			}
		}
	}
}

func commitMessage(ctx context.Context, cmd *command.Command, repo git.Repo, message string, changes git.Changes) error {
	out, err := repo.Commit(ctx, message, changes)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.Out(), out)
	return err
}

func collectInput(ctx context.Context, repo git.Repo, changes git.Changes, maxDiffBytes int, opts rootOptions) (commit.Input, error) {
	stat, err := repo.Stat(ctx, changes)
	if err != nil {
		return commit.Input{}, err
	}
	if strings.TrimSpace(stat) == "" {
		return commit.Input{}, errNothingToCommit
	}

	diff, err := repo.Diff(ctx, changes, excludedGlobs()...)
	if err != nil {
		return commit.Input{}, err
	}
	diff, truncated := truncateDiff(diff, maxDiffBytes)

	commits, err := repo.RecentCommits(ctx, recentCommitCount)
	if err != nil {
		return commit.Input{}, err
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return commit.Input{}, err
	}

	style := commit.Conventional
	if opts.plain {
		style = commit.Plain
	}

	return commit.Input{
		Diff:    diff,
		Stat:    stat,
		Commits: commits,
		Branch:  branch,
		Hint:    opts.context,
		Style:   style,

		DiffTruncated: truncated,
	}, nil
}
