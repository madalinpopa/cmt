package cli

import (
	"cmp"
	"context"
	"os"

	"github.com/madalinpopa/cmt/command"
	"github.com/madalinpopa/cmt/provider/anthropic"
)

type rootOptions struct {
	all     bool
	yes     bool
	dryRun  bool
	context string
	model   string
	plain   bool
}

func NewRootCommand() *command.Command {
	var opts rootOptions

	cmd := &command.Command{
		Use:   "cmt [flags] [--] [path...]",
		Short: "Generate git commit messages from your staged changes with Claude, then commit them.",
		Example: `cmt -a
cmt --dry-run
cmt -c "closes #42"
cmt -a -y
cmt --dry-run -- README.md cli/
cmt --model claude-opus-5-5`,
		Run: func(ctx context.Context, cmd *command.Command, args []string) error {
			return runRoot(ctx, cmd, opts, args)
		},
	}

	fs := cmd.Flags()
	fs.BoolVar(&opts.all, "a", false, "shorthand for --all")
	fs.BoolVar(&opts.all, "all", false, "stage all modified tracked files before generating")
	fs.BoolVar(&opts.yes, "y", false, "shorthand for --yes")
	fs.BoolVar(&opts.yes, "yes", false, "commit without asking for confirmation")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print the generated message without committing")
	fs.StringVar(&opts.context, "c", "", "shorthand for --context")
	fs.StringVar(&opts.context, "context", "", "give the model a hint, e.g. \"fixes #12\"")
	fs.StringVar(&opts.model, "model", cmp.Or(os.Getenv("CMT_MODEL"), anthropic.DefaultModel), "Claude model to use")
	fs.BoolVar(&opts.plain, "plain", false, "write a free-form message instead of Conventional Commits")

	return cmd
}
