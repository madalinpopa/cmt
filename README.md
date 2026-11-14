# cmt

`cmt` is a command-line tool that writes a git commit message for your changes and then commits them. It sends the diff, a file summary, the current branch, and your recent commit subjects to Claude (Anthropic API) or GitHub Copilot, and returns a [Conventional Commits](https://www.conventionalcommits.org/) message that you can accept, edit, or regenerate. It is meant for developers who want consistent commit messages without writing each one by hand.

The commit itself is made with `git commit`, so your hooks, signing, and git config still apply.

> **Status:** early development — the interface may still change before v1.

## Installation and quick start

Requirements:

- Go 1.27.4 or newer (from `go.mod`)
- `git` in your `PATH`
- Credentials for one provider: an Anthropic API key, or the [GitHub Copilot CLI](https://github.com/github/copilot-cli) signed in

Install with Homebrew (macOS and Linux):

```sh
brew install --cask madalinpopa/tap/cmt
```

Or with Go:

```sh
go install github.com/madalinpopa/cmt@latest
```

Set up credentials for the default provider (Anthropic):

```sh
export ANTHROPIC_API_KEY=<your-api-key>
```

Stage a change and run `cmt` inside the repository:

```sh
git add -p
cmt
```

`cmt` prints the generated message and a menu. Type `y` and press Enter to commit.

## Usage

```text
cmt [flags] [--] [path...]
```

| Flag | Description |
|------|-------------|
| `-a`, `--all` | Include unstaged changes to tracked files, like `git commit -a`; cannot be combined with paths |
| `-y`, `--yes` | Commit without asking for confirmation |
| `--dry-run` | Print the generated message to stdout without committing |
| `-c`, `--context "..."` | Give the model a hint, e.g. `-c "fixes #12"` |
| `--provider <name>` | `anthropic` (default) or `copilot` |
| `--model <id>` | Model to use (default: `claude-haiku-5-5` for `anthropic`, `auto` for `copilot`) |
| `--plain` | Write a free-form message instead of Conventional Commits |
| `-h`, `--help` | Show help |

Without paths, `cmt` commits what is staged. With paths, it commits the current content of those paths only, like `git commit --only -- <path>...`, and leaves the rest of the index alone. Put flags before paths.

### Example 1: review and commit staged changes

The default mode. `cmt` shows the message and waits for your choice.

```sh
cmt
```

Output (the message text is illustrative; it depends on your diff and the model):

```text
feat(git): add staged diff reader

Read the staged diff and file summary via the git binary so hooks
and signing configuration are respected.

[y] commit  [e] edit  [r] regenerate  [n] cancel:
```

| Answer | Action |
|--------|--------|
| `y` | Commit with the generated message |
| `e` | Open the message in your git editor, then commit (an empty message cancels) |
| `r` | Read the changes again and generate a new message |
| `n` | Cancel without committing; prints `Canceled.` |

Before committing with `y` or `e`, `cmt` checks that the changes still match what the model saw. If they changed in the meantime, it prints `Changes were modified after the message was generated; regenerating.` and asks again.

The menu requires a terminal. In scripts, CI, or editor commands, use `--yes` or `--dry-run`.

### Example 2: preview a message without committing

Useful for checking the result first, or for inserting the message from an editor. Only the message goes to stdout; warnings and errors go to stderr.

```sh
cmt --dry-run -c "closes #42"
```

Output (from the test `TestDryRunWritesOnlyMessageToStdout`, which uses a fake provider):

```text
feat(a): extend files

Add lines.
```

In the commit message buffer opened by `git commit`, Neovim users can run `:0r !cmt --dry-run` and Helix users `:insert-output cmt --dry-run`.

### Example 3: commit only some files, without the prompt

Commits the current content of `README.md` and `cli/` and leaves anything else staged as it is.

```sh
cmt -y -- README.md cli/
```

Output: the summary printed by `git commit`, for example (illustrative):

```text
[main 1a2b3c4] docs(readme): describe dry-run output
 1 file changed, 4 insertions(+)
```

With `--yes`, if the changes are modified while the message is generated, `cmt` commits nothing and exits with:

```text
error: changes were modified while the message was being generated; run cmt again
```

With nothing to commit, it exits with:

```text
error: nothing to commit; stage changes with git add, use --all, or pass paths
```

Exit codes: `0` on success, `1` on errors, `2` on invalid flags or arguments.

## Configuration

### Providers

**Anthropic** (default) uses the official Anthropic Go SDK, which reads credentials from the environment, for example `ANTHROPIC_API_KEY`.

**GitHub Copilot** runs the Copilot CLI in the background through the [GitHub Copilot SDK](https://github.com/github/copilot-sdk). Install the CLI, sign in once with `copilot`, and make sure `copilot` is in your `PATH` (or set `COPILOT_CLI_PATH`). Requests count against your Copilot plan.

```sh
cmt --provider copilot --model gpt-5-mini
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | — | Anthropic API key, required for the `anthropic` provider unless the SDK finds other credentials |
| `CMT_PROVIDER` | `anthropic` | Default provider; `--provider` overrides it |
| `CMT_MODEL` | provider default | Default model; `--model` overrides it |
| `CMT_MAX_DIFF_BYTES` | `200000` | Maximum diff size sent to the model; must be a positive integer |
| `COPILOT_CLI_PATH` | `copilot` from `PATH` | Path to the Copilot CLI |
| `EDITOR` | — | Editor for the `e` action, used when `GIT_EDITOR`, `core.editor`, and `VISUAL` are not set (same order as git) |

### What is sent to the model

Lockfiles (`go.sum`, `package-lock.json`, `yarn.lock`, `Cargo.lock`, and similar), `*.pb.go`, `*.min.js`, `*.min.css`, and files marked `linguist-generated` in `.gitattributes` are left out of the diff but still listed in the file summary. Binary files are not sent as content. If the diff is larger than `CMT_MAX_DIFF_BYTES`, `cmt` prints a warning on stderr and sends a partial diff cut at file boundaries.

## Troubleshooting

| Error | Fix |
|-------|-----|
| `stdin is not a terminal; use --yes to commit or --dry-run to only print the message` | The confirmation menu needs a terminal. Pass `--yes` or `--dry-run` in scripts and editors. |
| `nothing to commit; stage changes with git add, use --all, or pass paths` | Stage changes first, or use `-a` or paths. |
| `--all cannot be combined with paths` | Use either `-a` or paths, not both. |
| `copilot: GitHub Copilot CLI not found in PATH; install it or set COPILOT_CLI_PATH` | Install the Copilot CLI or set `COPILOT_CLI_PATH` to its location. |
| `anthropic: ...` authentication errors | Check that `ANTHROPIC_API_KEY` is set in the shell that runs `cmt`. |
| `warning: diff is larger than ... bytes (CMT_MAX_DIFF_BYTES); sending a partial diff` | The model saw only part of the diff. Commit in smaller pieces or raise `CMT_MAX_DIFF_BYTES`. |

## Development

Clone the repository and use the [Task](https://taskfile.dev) targets from `Taskfile.yml`, or the equivalent Go commands:

| Task | Command | Runs |
|------|---------|------|
| `task run -- <args>` | `go run . <args>` | Run `cmt` from source |
| `task test` | `go test ./...` | Run all tests |
| `task lint` | `golangci-lint run ./...` and `go mod tidy -diff` | Lint and check that `go.mod`/`go.sum` are tidy |
| `task format` | `go fmt ./...` and `golangci-lint fmt ./...` | Format code |
| `task check` | — | `format` followed by `lint` |

Linting requires [golangci-lint](https://golangci-lint.run); its configuration is in [`.golangci.yml`](.golangci.yml).

### Project layout

| Package | Responsibility |
|---------|----------------|
| [`main.go`](main.go) | Entry point; wires signal handling and exit codes |
| [`cli/`](cli) | Root command, flags, input collection, diff filtering and truncation, confirmation menu |
| [`command/`](command) | Minimal command and flag-parsing framework on top of the standard `flag` package |
| [`commit/`](commit) | Prompt construction, message validation, and formatting |
| [`provider/anthropic/`](provider/anthropic), [`provider/copilot/`](provider/copilot) | Model providers |
| [`vcs/`](vcs) | Git operations (diff, stat, snapshot, commit) run through the `git` binary |

## License

[MIT](LICENSE)
