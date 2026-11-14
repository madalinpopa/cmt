<h1 align="center">cmt</h1>

<p align="center">
  Conventional Commit messages generated from your git diff, by Claude or GitHub Copilot.
</p>

<p align="center">
  <a href="https://github.com/madalinpopa/cmt/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/madalinpopa/cmt/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/madalinpopa/cmt/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/madalinpopa/cmt?include_prereleases&sort=semver"></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/madalinpopa/cmt"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/madalinpopa/cmt"></a>
</p>

<p align="center">
  <img alt="cmt demo" src="docs/demo.gif" width="880">
</p>

`cmt` reads your changes, asks a model for a [Conventional Commits](https://www.conventionalcommits.org/) message, and commits with `git commit`, so hooks, signing, and git configuration still apply. The model receives the diff, a file summary, the current branch, and your recent commit subjects.

The interface may change before v1.

## Installation

Homebrew (macOS and Linux):

```sh
brew install --cask madalinpopa/tap/cmt
```

Go 1.27.4 or newer:

```sh
go install github.com/madalinpopa/cmt/cmd/cmt@latest
```

Prebuilt archives for Linux and macOS (amd64, arm64) are attached to each [release](https://github.com/madalinpopa/cmt/releases).

`cmt` requires `git` in `PATH` and credentials for one provider: an Anthropic API key, or a signed-in [GitHub Copilot CLI](https://github.com/github/copilot-cli).

## Quick start

```sh
export ANTHROPIC_API_KEY=<your-api-key>
git add -p
cmt
```

`cmt` prints the message and waits for an answer:

| Answer | Action |
|--------|--------|
| `y` | Commit with the generated message |
| `e` | Edit the message in your git editor, then commit; an empty message cancels |
| `r` | Generate a new message |
| `n` | Cancel |

If the changes are modified while the message is being generated, `cmt` regenerates before committing.

## Usage

```text
cmt [flags] [--] [path...]
```

| Flag | Description |
|------|-------------|
| `-a`, `--all` | Include unstaged changes to tracked files, like `git commit -a` |
| `-y`, `--yes` | Commit without confirmation |
| `--dry-run` | Print the message to stdout without committing |
| `-c`, `--context <text>` | Give the model a hint, e.g. `"closes #42"` |
| `--provider <name>` | `anthropic` (default) or `copilot` |
| `--model <id>` | Model ID; defaults to `claude-haiku-5-5` (Anthropic) or `auto` (Copilot) |
| `--plain` | Free-form message instead of Conventional Commits |

Without paths, `cmt` commits the index. With paths, it commits only those paths, like `git commit --only`, and leaves the rest of the index untouched. `--all` cannot be combined with paths.

### Examples

```sh
cmt                                  # review the message, then commit staged changes
cmt -a -y                            # commit all tracked changes without confirmation
cmt -y -- README.md cli/             # commit only these paths
cmt --dry-run -c "closes #42"        # print the message only
cmt --provider copilot --model gpt-5-mini
cmt --plain                          # free-form message
```

The confirmation menu needs a terminal. In scripts, CI, and editors use `--yes` or `--dry-run`. `--dry-run` writes only the message to stdout, so it can be inserted into a commit buffer: `:0r !cmt --dry-run` in Neovim, `:insert-output cmt --dry-run` in Helix.

Exit codes: `0` success, `1` error, `2` invalid flags or arguments.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | | Anthropic API key |
| `CMT_PROVIDER` | `anthropic` | Default provider |
| `CMT_MODEL` | provider default | Default model |
| `CMT_MAX_DIFF_BYTES` | `200000` | Maximum diff size sent to the model |
| `COPILOT_CLI_PATH` | `copilot` in `PATH` | Path to the Copilot CLI |

Flags take precedence over environment variables. The edit action resolves the editor in the same order as git: `GIT_EDITOR`, `core.editor`, `VISUAL`, `EDITOR`.

The Copilot provider runs the Copilot CLI through the [GitHub Copilot SDK](https://github.com/github/copilot-sdk); sign in once with `copilot`. Requests count against your Copilot plan.

### Data sent to the model

Lockfiles, `*.pb.go`, `*.min.js`, `*.min.css`, and files marked `linguist-generated` in `.gitattributes` are listed in the file summary but their content is not sent. Binary content is never sent. Diffs larger than `CMT_MAX_DIFF_BYTES` are cut at file boundaries, with a warning on stderr.

## Development

Requires Go 1.27.4+, [Task](https://taskfile.dev), and [golangci-lint](https://golangci-lint.run).

| Command | Description |
|---------|-------------|
| `task run -- <args>` | Run from source |
| `task test` | Run the tests |
| `task lint` | Lint and verify `go.mod` and `go.sum` are tidy |
| `task format` | Format the code |
| `task check` | Format, then lint |

| Package | Responsibility |
|---------|----------------|
| [`cmd/cmt`](cmd/cmt) | Entry point; signal handling and exit codes |
| [`internal/cli`](internal/cli) | Root command, input collection, diff filtering, confirmation |
| [`internal/command`](internal/command) | Command and flag parsing on top of the standard `flag` package |
| [`internal/commit`](internal/commit) | Prompt construction, message validation and formatting |
| [`internal/provider`](internal/provider) | Anthropic and Copilot providers |
| [`internal/vcs`](internal/vcs) | Git operations through the `git` binary |

The demo is recorded with [VHS](https://github.com/charmbracelet/vhs) and calls the configured provider:

```sh
vhs docs/demo.tape
```

## Releases

Every push and pull request runs lint, tests with the race detector, and a GoReleaser snapshot build. Pushing a `vX.Y.Z` tag from `main` publishes a GitHub release and updates the Homebrew cask.

```sh
task release:patch   # or release:minor, release:major
```

## Contributing

1. Open an issue for significant changes before starting work.
2. Create a branch from `main`.
3. Write commits in the [Conventional Commits](https://www.conventionalcommits.org/) format.
4. Run `task check` and `task test` before pushing.
5. Open a pull request with a clear description of the change and its motivation.

## License

[MIT](LICENSE)
