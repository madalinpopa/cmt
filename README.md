# cmt

Generate git commit messages from your staged changes with Claude, then commit them.

> **Status:** early development — the commands below describe the planned v1 interface.

## How it works

1. Reads your staged diff (`git diff --cached`), the changed-file summary, recent commits (to match your style), and the current branch name.
2. Sends them to the Anthropic API and gets back a structured [Conventional Commits](https://www.conventionalcommits.org/) message.
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

[y] commit  [e] edit  [r] regenerate  [n] cancel
```

| Key | Action |
|-----|--------|
| `y` | Commit with the generated message |
| `e` | Open the message in `$EDITOR`, then commit |
| `r` | Ask Claude for a new message |
| `n` | Cancel without committing |

## Commands and flags

```text
cmt [flags]
```

| Flag | Description |
|------|-------------|
| `-a`, `--all` | Stage all modified tracked files before generating (like `git commit -a`) |
| `-y`, `--yes` | Commit without asking for confirmation |
| `--dry-run` | Print the generated message without committing |
| `-c`, `--context "..."` | Give the model a hint, e.g. `-c "fixes #12"` |
| `--model <id>` | Use another Claude model (default: `claude-haiku-5-5`) |
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

# Use a more capable model
cmt --model claude-opus-5-5
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key (optional if you used `ant auth login`) |
| `CMT_MODEL` | Default model; the `--model` flag overrides it |
| `CMT_MAX_DIFF_BYTES` | Diff size limit sent to the model (default: `200000`) |
| `EDITOR` | Editor opened by the `e` action |

## Large diffs

Lockfiles (`go.sum`, `package-lock.json`, ...), binary files, and generated files are left out of what gets sent to the model. If the diff is still bigger than `CMT_MAX_DIFF_BYTES`, `cmt` warns you and sends the file summary plus a partial diff instead of quietly cutting it.

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
