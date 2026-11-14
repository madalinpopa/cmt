package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
