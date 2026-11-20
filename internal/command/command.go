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

	flags *flag.FlagSet

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

	positional, err := parseInterleaved(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return c.PrintUsage(c.Out())
	}

	if err != nil {
		return &UsageError{Message: err.Error()}
	}

	if c.Run == nil {
		return c.PrintUsage(c.Out())
	}
	return c.Run(ctx, c, positional)
}

func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			return append(positional, rest...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func (c *Command) PrintUsage(w io.Writer) error {
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

	_, err := io.WriteString(w, b.String())
	return err
}
