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
	"github.com/madalinpopa/cmt/provider/anthropic"
	"github.com/madalinpopa/cmt/provider/copilot"
	"github.com/madalinpopa/cmt/vcs"
)

const recentCommitCount = 10

var (
	errNothingToCommit = errors.New("nothing to commit; stage changes with git add, use --all, or pass paths")
	errChangesModified = errors.New("changes were modified while the message was being generated; run cmt again")
)

type runner struct {
	repo        vcs.Repo
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
	changes := vcs.Changes{All: opts.all, Paths: args}

	generate := func() (draft, error) {
		snapshot, err := repo.Snapshot(ctx, changes)
		if err != nil {
			return draft{}, err
		}
		in, err := collectInput(ctx, cmd.Err(), repo, changes, maxDiffBytes, opts)
		if err != nil {
			return draft{}, err
		}
		msg, err := commit.Generate(ctx, provider, in)
		return draft{msg: msg, snapshot: snapshot}, err
	}

	d, err := generate()
	if err != nil {
		return err
	}

	switch {
	case opts.dryRun:
		_, err = fmt.Fprintln(cmd.Out(), d.msg)
		return err
	case opts.yes:
		changed, err := d.changed(ctx, repo, changes)
		if err != nil {
			return err
		}
		if changed {
			return errChangesModified
		}
		return commitMessage(ctx, cmd, repo, d.msg.String(), changes)
	default:
		return confirmAndCommit(ctx, cmd, repo, changes, d, generate)
	}
}

type draft struct {
	msg      commit.Message
	snapshot string
}

func (d draft) changed(ctx context.Context, repo vcs.Repo, changes vcs.Changes) (bool, error) {
	current, err := repo.Snapshot(ctx, changes)
	return current != d.snapshot, err
}

func confirmAndCommit(ctx context.Context, cmd *command.Command, repo vcs.Repo, changes vcs.Changes, d draft, regenerate func() (draft, error)) error {
	answers := bufio.NewReader(cmd.In())
	for {
		if _, err := fmt.Fprintf(cmd.Out(), "%s\n\n", d.msg); err != nil {
			return err
		}

		c, err := askChoice(answers, cmd.Out())
		if err != nil {
			return err
		}
		if c, err = regenerateIfChanged(ctx, cmd, repo, changes, d, c); err != nil {
			return err
		}

		switch c {
		case choiceCommit:
			return commitMessage(ctx, cmd, repo, d.msg.String(), changes)
		case choiceEdit:
			return repo.CommitEdited(ctx, d.msg.String(), changes, cmd.In(), cmd.Out(), cmd.Err())
		case choiceCancel:
			_, err := fmt.Fprintln(cmd.Out(), "Canceled.")
			return err
		case choiceRegenerate:
			if d, err = regenerate(); err != nil {
				return err
			}
		}
	}
}

func regenerateIfChanged(ctx context.Context, cmd *command.Command, repo vcs.Repo, changes vcs.Changes, d draft, c choice) (choice, error) {
	if c != choiceCommit && c != choiceEdit {
		return c, nil
	}
	changed, err := d.changed(ctx, repo, changes)
	if err != nil || !changed {
		return c, err
	}
	_, err = fmt.Fprintln(cmd.Err(), "Changes were modified after the message was generated; regenerating.")
	return choiceRegenerate, err
}

func commitMessage(ctx context.Context, cmd *command.Command, repo vcs.Repo, message string, changes vcs.Changes) error {
	out, err := repo.Commit(ctx, message, changes)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.Out(), out)
	return err
}

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
