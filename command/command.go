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

// Execute parses args, validates required flags and calls Run with the
// positional arguments. Flags may appear before or after positional
// arguments; everything after a "--" terminator is treated as positional.
//
// -h/--help prints usage to Out and returns nil. Parse failures and missing
// required flags are returned as *UsageError.
func (c *Command) Execute(ctx context.Context, args []string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	fs := c.Flags()
	positional, err := parseInterspersed(fs, args)
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
	return c.Run(ctx, c, positional)
}

// parseInterspersed parses flags that may be mixed with positional arguments.
// The standard flag package stops at the first non-flag argument, so parsing
// resumes after each positional argument until args are exhausted or a "--"
// terminator is consumed.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}

		consumed := len(rest) - len(fs.Args())
		terminated := consumed > 0 && rest[consumed-1] == "--"
		rest = fs.Args()

		if terminated {
			positional = append(positional, rest...)
			break
		}
		// fs.Parse only stops early at a non-flag argument, so rest[0]
		// (if any) is positional.
		if len(rest) > 0 {
			positional = append(positional, rest[0])
			rest = rest[1:]
		}
	}
	return positional, nil
}

// PrintUsage writes the command's help text to w.
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

	io.WriteString(w, b.String())
}
