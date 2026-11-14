package command

import (
	"flag"
	"io"
	"slices"
)

func (c *Command) Flags() *flag.FlagSet {
	if c.flags == nil {
		c.flags = newFlagSet(c.Name())
	}
	return c.flags
}

func (c *Command) MarkRequired(names ...string) {
	c.required = append(c.required, names...)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func isFlagArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

func (c *Command) missingRequired(fs *flag.FlagSet) []string {
	wanted := slices.Clone(c.required)
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	var missing []string
	for _, name := range wanted {
		if !set[name] {
			missing = append(missing, "--"+name)
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}
