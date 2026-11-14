package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/madalinpopa/cmt/commit"
	"github.com/madalinpopa/cmt/vcs"
)

const recentCommitCount = 10

var errNothingToCommit = errors.New("nothing to commit; stage changes with git add, use --all, or pass paths")

func collectInput(ctx context.Context, warn io.Writer, repo vcs.Repo, changes vcs.Changes, maxDiffBytes int, opts rootOptions) (commit.Input, error) {
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
	if truncated {
		if _, err := fmt.Fprintf(warn, "warning: diff is larger than %d bytes (CMT_MAX_DIFF_BYTES); sending a partial diff\n", maxDiffBytes); err != nil {
			return commit.Input{}, err
		}
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

		DiffTruncated: truncated,
	}, nil
}
