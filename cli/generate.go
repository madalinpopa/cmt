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

var errNothingStaged = errors.New("no staged changes; stage files with git add first")

func runRoot(ctx context.Context, cmd *command.Command, opts rootOptions, args []string) error {
	if len(args) > 0 {
		return &command.UsageError{Message: "unexpected arguments: " + strings.Join(args, " ")}
	}
	if opts.all || !opts.dryRun {
		return errors.New("only --dry-run is implemented so far")
	}

	in, err := collectInput(ctx, git.Repo{}, opts)
	if err != nil {
		return err
	}

	msg, err := commit.Generate(ctx, anthropic.New(opts.model), in)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(cmd.Out(), msg)
	return err
}

func collectInput(ctx context.Context, repo git.Repo, opts rootOptions) (commit.Input, error) {
	diff, err := repo.StagedDiff(ctx)
	if err != nil {
		return commit.Input{}, err
	}
	if strings.TrimSpace(diff) == "" {
		return commit.Input{}, errNothingStaged
	}

	stat, err := repo.StagedStat(ctx)
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
