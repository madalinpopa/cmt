# cmt

Generate git commit messages from your staged changes with Claude or GitHub Copilot, then commit them.

> **Status:** early development — the interface may still change before v1.

## How it works

1. Reads the diff of what will be committed, the changed-file summary, recent commit subjects (to match your style), and the current branch name.
2. Sends them to the selected provider (Anthropic API or GitHub Copilot) and gets back a structured [Conventional Commits](https://www.conventionalcommits.org/) message.
3. Shows the message and asks what to do: commit, edit, regenerate, or cancel.
4. Commits with `git commit`, so your hooks, signing, and git config still apply.

## Installation

```sh
go install github.com/madalinpopa/cmt@latest
```

## Authentication

`cmt` uses the official Anthropic Go SDK and picks up credentials automatically. Either:

```sh
export ANTHROPIC_API_KEY=sk-ant-...
```

or sign in once with the Anthropic CLI:

```sh
ant auth login
```

### GitHub Copilot

With `--provider copilot`, `cmt` uses the [GitHub Copilot SDK](https://github.com/github/copilot-sdk), which runs the [Copilot CLI](https://github.com/github/copilot-cli) in the background. Install the CLI, sign in once with `copilot`, and make sure `copilot` is in your `PATH` (or set `COPILOT_CLI_PATH`). The CLI also accepts a token from `COPILOT_GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_TOKEN`. Requests count against your Copilot plan.

```sh
cmt --provider copilot
```

## Usage

```sh
git add -p
cmt
```

Example session:

```text
$ cmt
feat(git): add staged diff reader

Read the staged diff and file summary via the git binary so hooks
and signing configuration are respected.

[y] commit  [e] edit  [r] regenerate  [n] cancel:
```

Type the letter and press Enter.

| Key | Action |
|-----|--------|
| `y` | Commit with the generated message |
| `e` | Open the message in your git editor, then commit (saving an empty message cancels) |
| `r` | Ask the model for a new message |
| `n` | Cancel without committing |

## Commands and flags

```text
cmt [flags] [--] [path...]
```

Without paths, `cmt` commits what is staged. With paths, it commits the current content of those paths only, like `git commit -- <path>...`, and leaves the rest of the index alone. Put flags before paths: everything after the first path is treated as a path.

The menu needs a terminal. When stdin is not a terminal (scripts, CI, editor commands), use `--yes` or `--dry-run`.

| Flag | Description |
|------|-------------|
| `-a`, `--all` | Include unstaged changes to tracked files (like `git commit -a`); cannot be combined with paths |
| `-y`, `--yes` | Commit without asking for confirmation |
| `--dry-run` | Print the generated message to stdout without committing |
| `-c`, `--context "..."` | Give the model a hint, e.g. `-c "fixes #12"` |
| `--provider <name>` | Model provider: `anthropic` (default) or `copilot` |
| `--model <id>` | Model to use (default: `claude-haiku-5-5` for `anthropic`, `auto` for `copilot`) |
| `--plain` | Write a free-form message instead of Conventional Commits |
| `-h`, `--help` | Show help |

### Examples

```sh
# Stage everything and review the message
cmt -a

# Only see what would be generated
cmt --dry-run

# Add context the diff doesn't show
cmt -c "closes #42, requested by support"

# Commit without the prompt (e.g. in scripts)
cmt -a -y

# Commit only some files, leaving the rest staged
cmt -- README.md cli/

# Use a more capable model
cmt --model claude-opus-5-5

# Use GitHub Copilot with a specific model
cmt --provider copilot --model gpt-5-mini
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key (optional if you used `ant auth login`) |
| `CMT_PROVIDER` | Default provider; the `--provider` flag overrides it |
| `CMT_MODEL` | Default model; the `--model` flag overrides it |
| `COPILOT_CLI_PATH` | Path to the Copilot CLI when it is not in `PATH` |
| `CMT_MAX_DIFF_BYTES` | Diff size limit sent to the model (default: `200000`) |
| `EDITOR` | Editor opened by the `e` action, unless `GIT_EDITOR`, `core.editor`, or `VISUAL` is set (same order as git) |

## Large diffs

Lockfiles (`go.sum`, `package-lock.json`, ...), minified and protobuf-generated files, and files marked `linguist-generated` in `.gitattributes` are left out of the diff sent to the model; binary files are sent only as a one-line note. They still appear in the file summary. If the diff is still bigger than `CMT_MAX_DIFF_BYTES`, `cmt` warns you on stderr and sends the file summary plus a partial diff cut at file boundaries instead of quietly cutting it.

## Editor integration

With `--dry-run`, stdout contains only the message; warnings and errors go to stderr. That makes `cmt` easy to call from an editor. `cmt` reads changes from git, so save the buffer first.

Fill the commit message buffer opened by `git commit`:

| Editor | Command |
|--------|---------|
| Neovim | `:0r !cmt --dry-run` |
| Helix | `:insert-output cmt --dry-run` |

Commit the current file directly:

| Editor | Command |
|--------|---------|
| Neovim | `:w \| !cmt -y -- %:p` |
| Helix | `:write` then `:sh cmt -y -- %{buffer_name}` |

## Development

Requires [Task](https://taskfile.dev) and [golangci-lint](https://golangci-lint.run).

```sh
task run -- --help   # run the CLI with arguments
task test            # run all tests
task format          # format Go code
task lint            # run golangci-lint and check go.mod/go.sum are tidy
task check           # format, then lint
```

## License

[MIT](LICENSE)
