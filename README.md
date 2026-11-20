<h1 align="center">cmt</h1>

<p align="center">
  Conventional Commit messages generated from your git diff, by Claude, OpenAI, Gemini, or GitHub Copilot.
</p>

<p align="center">
  <a href="https://github.com/madalinpopa/cmt/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/madalinpopa/cmt/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/madalinpopa/cmt/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/madalinpopa/cmt?include_prereleases&sort=semver"></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/madalinpopa/cmt"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-blue"></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset="docs/demo-light.gif">
    <img alt="cmt demo" src="docs/demo.gif" width="880">
  </picture>
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

`cmt` requires `git` in `PATH` and credentials for one provider: an Anthropic API key, an OpenAI API key, a Gemini API key, or a signed-in [GitHub Copilot CLI](https://github.com/github/copilot-cli).

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
| `--provider <name>` | `anthropic` (default), `openai`, `gemini`, or `copilot` |
| `--model <id>` | Model ID; defaults to `claude-haiku-5-5` (Anthropic), `gpt-5.4-mini` (OpenAI), `gemini-3.5-flash` (Gemini), or `auto` (Copilot) |
| `--plain` | Free-form message instead of Conventional Commits |

Without paths, `cmt` commits the index. With paths, it commits only those paths, like `git commit --only`, and leaves the rest of the index untouched. `--all` cannot be combined with paths.

### Examples

```sh
cmt                                  # review the message, then commit staged changes
cmt -a -y                            # commit all tracked changes without confirmation
cmt -y -- README.md cli/             # commit only these paths
cmt --dry-run -c "closes #42"        # print the message only
cmt --provider openai
cmt --provider gemini
cmt --provider copilot --model gpt-5-mini
cmt --plain                          # free-form message
```

The confirmation menu needs a terminal. In scripts, CI, and editors use `--yes` or `--dry-run`. `--dry-run` writes only the message to stdout, so it can be inserted into a commit buffer: `:0r !cmt --dry-run` in Neovim, `:insert-output cmt --dry-run` in Helix.

Exit codes: `0` success, `1` error, `2` invalid flags or arguments.

## How I use it

I let `cmt` prefill the commit message whenever I run a plain `git commit`, through a global `prepare-commit-msg` hook. Git opens my editor with the generated message already in place, so I can review it, tweak it, or replace it before saving.

Create the hook in a git template directory:

```sh
mkdir -p ~/.config/git/template/hooks
cat > ~/.config/git/template/hooks/prepare-commit-msg <<'EOF'
#!/bin/sh

[ -z "$2" ] || exit 0
command -v cmt >/dev/null 2>&1 || exit 0

tmp="$1.cmt"
if command cmt --dry-run >"$tmp" && [ -s "$tmp" ]; then
  cat "$1" >>"$tmp" && mv "$tmp" "$1"
fi
rm -f "$tmp"
exit 0
EOF
chmod +x ~/.config/git/template/hooks/prepare-commit-msg
git config --global init.templateDir ~/.config/git/template
```

New clones and `git init` repositories get the hook automatically. To add it to an existing repository, run `git init` inside it; this is safe and does not overwrite hooks that already exist.

How it behaves:

- It only runs for a plain `git commit`. Commits made with `-m` or `-F`, merges, squashes, amends, and commits made by `cmt` itself keep their message untouched.
- If `commit.template` is set, git treats every plain `git commit` as using a template, so the hook never runs. Unset it with `git config --global --unset commit.template` to use the hook.
- It works with `git commit -a` and `git commit -- <path>`, because `cmt --dry-run` reads the index that git is about to commit.
- If `cmt` is not installed or fails, for example because the API key is missing, the commit continues with the usual empty message.
- Repositories that set `core.hooksPath`, as Husky and lefthook do, ignore `.git/hooks`, so the hook does not run there.

You can still write commit messages yourself:

- `git commit -m "message"` or `git commit -F file` uses your message directly and skips generation entirely.
- In a plain `git commit`, edit or delete the generated message before saving. Saving an empty message aborts the commit as usual.
- `git -c core.hooksPath=/dev/null commit` opens an empty editor for a single commit by disabling all hooks for it. `--no-verify` does not help here, because it skips `pre-commit` and `commit-msg` but not `prepare-commit-msg`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | | Anthropic API key |
| `OPENAI_API_KEY` | | OpenAI API key |
| `GEMINI_API_KEY` | | Gemini API key; `GOOGLE_API_KEY` also works and takes precedence |
| `CMT_PROVIDER` | `anthropic` | Default provider |
| `CMT_MODEL` | provider default | Default model |
| `CMT_MAX_DIFF_BYTES` | `200000` | Maximum diff size sent to the model |
| `COPILOT_CLI_PATH` | `copilot` in `PATH` | Path to the Copilot CLI |

Flags take precedence over environment variables. The edit action resolves the editor in the same order as git: `GIT_EDITOR`, `core.editor`, `VISUAL`, `EDITOR`.

The Copilot provider runs the Copilot CLI through the [Copilot SDK](https://github.com/microsoft/copilot-sdk); sign in once with `copilot`. Requests count against your Copilot plan.

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
| [`internal/provider`](internal/provider) | Anthropic, OpenAI, Gemini, and Copilot providers |
| [`internal/vcs`](internal/vcs) | Git operations through the `git` binary |

## Contributing

1. Open an issue for significant changes before starting work.
2. Create a branch from `main`.
3. Write commits in the [Conventional Commits](https://www.conventionalcommits.org/) format.
4. Run `task check` and `task test` before pushing.
5. Open a pull request with a clear description of the change and its motivation.

## License

[MIT](LICENSE)
