package command

import (
	"flag"
	"io"
)

func (c *Command) Flags() *flag.FlagSet {
	if c.flags == nil {
		c.flags = newFlagSet(c.Name())
	}
	return c.flags
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}
