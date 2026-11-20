package cli

import (
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

func askChoice(ctx context.Context, r io.Reader, w io.Writer) (choice, error) {
	for {
		if _, err := fmt.Fprint(w, "[y] commit  [e] edit  [r] regenerate  [n] cancel: "); err != nil {
			return 0, err
		}

		line, readErr := readLineContext(ctx, r)
		if ctx.Err() != nil {
			return 0, context.Cause(ctx)
		}
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

type lineResult struct {
	line string
	err  error
}

func readLineContext(ctx context.Context, r io.Reader) (string, error) {
	lines := make(chan lineResult, 1)
	go func() {
		line, err := readLine(r)
		lines <- lineResult{line, err}
	}()
	select {
	case <-ctx.Done():
		return "", context.Cause(ctx)
	case res := <-lines:
		return res.line, res.err
	}
}

func readLine(r io.Reader) (string, error) {
	var line []byte
	var buf [1]byte
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			line = append(line, buf[0])
			if buf[0] == '\n' {
				return string(line), nil
			}
		}
		if err != nil {
			return string(line), err
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
	for {
		if _, err := fmt.Fprintf(cmd.Out(), "%s\n\n", d.msg); err != nil {
			return err
		}

		c, err := askChoice(ctx, cmd.In(), cmd.Out())
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
