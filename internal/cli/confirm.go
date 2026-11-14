package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/madalinpopa/cmt/internal/command"
	"github.com/madalinpopa/cmt/internal/vcs"
)

type choice int

const (
	choiceCommit choice = iota
	choiceEdit
	choiceRegenerate
	choiceCancel
)

var errNoAnswer = errors.New("canceled: no answer on stdin")

func askChoice(r *bufio.Reader, w io.Writer) (choice, error) {
	for {
		if _, err := fmt.Fprint(w, "[y] commit  [e] edit  [r] regenerate  [n] cancel: "); err != nil {
			return 0, err
		}

		line, readErr := r.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return choiceCommit, nil
		case "e", "edit":
			return choiceEdit, nil
		case "r", "regenerate":
			return choiceRegenerate, nil
		case "n", "no":
			return choiceCancel, nil
		}

		if errors.Is(readErr, io.EOF) {
			return 0, errNoAnswer
		}
		if readErr != nil {
			return 0, readErr
		}
		if _, err := fmt.Fprintln(w, "Please answer y, e, r, or n."); err != nil {
			return 0, err
		}
	}
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
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
