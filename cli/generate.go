package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
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

	maxDiffBytes, err := parseMaxDiffBytes(os.Getenv("CMT_MAX_DIFF_BYTES"))
	if err != nil {
		return err
	}

	repo := git.Repo{}
	changes := git.Staged
	if opts.all {
		changes = git.Tracked
	}

	in, err := collectInput(ctx, repo, changes, maxDiffBytes, opts)
	if err != nil {
		return err
	}
	if in.DiffTruncated {
		if _, err := fmt.Fprintf(cmd.Err(), "warning: diff is larger than %d bytes (CMT_MAX_DIFF_BYTES); sending a partial diff\n", maxDiffBytes); err != nil {
			return err
		}
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
