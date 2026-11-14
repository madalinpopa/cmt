package command

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

type Command struct {
	Use     string
	Short   string
	Long    string
	Example string
	Run     func(ctx context.Context, c *Command, args []string) error

	flags    *flag.FlagSet
	required []string

	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (c *Command) Name() string {
	name, _, _ := strings.Cut(c.Use, " ")
	return name
}

func (c *Command) Execute(ctx context.Context, args []string) error {
	fs := c.Flags()

	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		c.PrintUsage(c.Out())
		return nil
	}

	if err != nil {
		return &UsageError{Message: err.Error()}
	}

	if missing := c.missingRequired(fs); len(missing) > 0 {
		return &UsageError{
			Message: fmt.Sprintf("required flag(s) not set: %s", strings.Join(missing, ", ")),
		}
	}

	if c.Run == nil {
		c.PrintUsage(c.Out())
		return nil
	}
	return c.Run(ctx, c, fs.Args())
}

func (c *Command) PrintUsage(w io.Writer) {
	var b strings.Builder

	if desc := cmp.Or(c.Long, c.Short); desc != "" {
		b.WriteString(strings.TrimSpace(desc))
		b.WriteString("\n\n")
	}

	fmt.Fprintf(&b, "Usage:\n  %s\n", cmp.Or(c.Use, c.Name()))

	fs := c.Flags()
	hasFlags := false
	fs.VisitAll(func(*flag.Flag) { hasFlags = true })
	if hasFlags {
		b.WriteString("\nFlags:\n")
		fs.SetOutput(&b)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}

	if c.Example != "" {
		b.WriteString("\nExamples:\n")
		for line := range strings.Lines(strings.TrimSpace(c.Example)) {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	_, _ = io.WriteString(w, b.String())
}
