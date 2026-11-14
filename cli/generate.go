package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/madalinpopa/cmt/command"
	"github.com/madalinpopa/cmt/commit"
	"github.com/madalinpopa/cmt/git"
	"github.com/madalinpopa/cmt/provider/anthropic"
)

const recentCommitCount = 10

var errNothingToCommit = errors.New("nothing to commit; stage changes with git add or use --all")

func runRoot(ctx context.Context, cmd *command.Command, opts rootOptions, args []string) error {
	if len(args) > 0 {
		return &command.UsageError{Message: "unexpected arguments: " + strings.Join(args, " ")}
	}
	if !opts.dryRun && !opts.yes {
		return errors.New("interactive confirmation is not implemented yet; use --yes or --dry-run")
	}

	repo := git.Repo{}
	changes := git.Staged
	if opts.all {
		changes = git.Tracked
	}

	in, err := collectInput(ctx, repo, changes, opts)
	if err != nil {
		return err
	}

	msg, err := commit.Generate(ctx, anthropic.New(opts.model), in)
	if err != nil {
		return err
	}

	if opts.dryRun {
		_, err = fmt.Fprintln(cmd.Out(), msg)
		return err
	}

	out, err := repo.Commit(ctx, msg.String(), changes)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.Out(), out)
	return err
}

func collectInput(ctx context.Context, repo git.Repo, changes git.Changes, opts rootOptions) (commit.Input, error) {
	diff, err := repo.Diff(ctx, changes)
	if err != nil {
		return commit.Input{}, err
	}
	if strings.TrimSpace(diff) == "" {
		return commit.Input{}, errNothingToCommit
	}

	stat, err := repo.Stat(ctx, changes)
	if err != nil {
		return commit.Input{}, err
	}

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
	}, nil
}
