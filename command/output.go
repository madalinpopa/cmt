package command

import (
	"io"
	"os"
)

func (c *Command) SetIn(r io.Reader) {
	c.in = r
}

func (c *Command) In() io.Reader {
	if c.in != nil {
		return c.in
	}
	return os.Stdin
}

func (c *Command) SetOut(w io.Writer) {
	c.out = w
}

func (c *Command) SetErr(w io.Writer) {
	c.errOut = w
}

func (c *Command) Out() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *Command) Err() io.Writer {
	if c.errOut != nil {
		return c.errOut
	}
	return os.Stderr
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}
