package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/madalinpopa/cmt/internal/command"
	"github.com/madalinpopa/cmt/internal/commit"
	"github.com/madalinpopa/cmt/internal/provider/anthropic"
	"github.com/madalinpopa/cmt/internal/provider/copilot"
	"github.com/madalinpopa/cmt/internal/vcs"
)

var errChangesModified = errors.New("changes were modified while the message was being generated; run cmt again")

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
