# git-worktree-runner

git-worktree-runner (a.k.a. `git-gtr`) is a Go port of [github.com/coderabbitai/git-worktree-runner](https://github.com/coderabbitai/git-worktree-runner) with some additional features.

## Install

This project builds a Git subcommand binary named `git-gtr` (so you run it as `git gtr ...`).

## Requirements

- Git installed (and new enough for `git worktree`).
- Go 1.25+ (only needed to build from source).

### With `go install` (if you have a tagged release)

```bash
go install github.com/zchee/git-worktree-runner/cmd/git-gtr@latest
```

### From source (recommended during development)

```bash
git clone https://github.com/zchee/git-worktree-runner.git
cd git-worktree-runner

make build/git-gtr

# Ensure the `git-gtr` binary is on your PATH.
# Example (macOS/Linux):
ln -sf "$(pwd)/bin/git-gtr" /usr/local/bin/git-gtr
# or
sudo ln -sf "$(pwd)/bin/git-gtr" /usr/local/bin/git-gtr
```

## Quick start

```bash
cd /path/to/your/repo

# Optional one-time setup (per repo or --global)
git gtr config set gtr.editor.default cursor
git gtr config set gtr.ai.default claude

# Create a worktree for a branch (creates a sibling folder by default)
git gtr new feature-auth --yes

# List worktrees
git gtr list
git gtr list --porcelain   # path<TAB>branch<TAB>status

# Navigate (shell-friendly)
cd "$(git gtr go feature-auth)"

# Run a command in a worktree directory (exit code propagates)
git gtr run feature-auth git status

# Remove the worktree
git gtr rm feature-auth --yes
```

## Commands (v1)

The CLI targets upstream parity for the core UX:

- `git gtr new <branch> [options]` — create a worktree
- `git gtr rm <id|branch|worktree-name>... [options]` — remove worktree(s)
  - `--delete-branch` prompts before deleting the branch (skip prompts with `--yes`)
- `git gtr go <id|branch|worktree-name>` — print absolute path to stdout
- `git gtr run <id|branch|worktree-name> <command...>` — run command in that directory
- `git gtr list [--porcelain]` — list main repo + worktrees
- `git gtr copy <target>... [options] [-- <pattern>...]` — copy files between worktrees
- `git gtr editor <id|branch|worktree-name> [--editor <name>]`
- `git gtr ai <id|branch|worktree-name> [--ai <name>] [-- args...]`
- `git gtr clean` — prune stale worktrees and remove empty directories in the configured base dir
- `git gtr doctor` — basic health check
- `git gtr adapter` — list built-in adapters and availability
- `git gtr config {get|set|add|unset} <key> [value] [--global]`
- `git gtr version`, `git gtr help`

## Configuration

Configuration is resolved with this precedence (highest to lowest):

1. local git config (`.git/config`)
2. `.gtrconfig` (repo root, gitconfig syntax)
3. global git config (`~/.gitconfig`)
4. system git config
5. environment variables
6. hard-coded defaults

Common keys:

- `gtr.worktrees.dir`: base directory for worktrees
  - default: `<repo-parent>/<repo-name>-worktrees`
  - supports absolute paths, repo-relative paths, and `~` expansion
- `gtr.worktrees.prefix`: prefix added to each worktree folder name
- `gtr.defaultBranch`: `auto|main|master|<branch>`
- `gtr.editor.default`: editor adapter name or `none`
- `gtr.ai.default`: AI adapter name or `none`
  - `cursor`: prefers `cursor-agent`, then tries `cursor cli` (varies by Cursor version), then falls back to `cursor`
- `gtr.copy.include` / `gtr.copy.exclude` (multi): file globs for copying
- `gtr.copy.includeDirs` / `gtr.copy.excludeDirs` (multi): directory copy rules
- `gtr.hook.postCreate` / `gtr.hook.postRemove` (multi): hook commands

Environment variables supported:

- `GTR_WORKTREES_DIR`
- `GTR_WORKTREES_PREFIX`
- `GTR_DEFAULT_BRANCH`
- `GTR_EDITOR_DEFAULT`
- `GTR_AI_DEFAULT`

## Development

```bash
make test
make fmt
make lint
```

This repository uses integration-style tests (real `git`, real filesystem; no mocks).

## Shell completions

Upstream ships completion scripts; this Go port includes equivalents in `./completions/`:

- Zsh: `completions/_git-gtr`
  - Install: copy/symlink into a directory in your `$fpath` (example: `~/.zsh/completions/`), then run `autoload -Uz compinit && compinit`.
- Bash: `completions/gtr.bash`
  - Install: add `source /path/to/git-worktree-runner/completions/gtr.bash` to your `~/.bashrc`.
- Fish: `completions/gtr.fish`
  - Install: `ln -s /path/to/git-worktree-runner/completions/gtr.fish ~/.config/fish/completions/` then `exec fish`.

## Templates

Example configuration and helper scripts live under `./templates/`:

- `templates/.gtrconfig.example` → copy to `<repo-root>/.gtrconfig` (gitconfig syntax)
- `templates/gtr.config.example` → reference for common `git config` keys
- `templates/setup-example.sh` → example one-time repo setup (`git config --local ...`)
- `templates/run_services.example.sh` → example “run multiple services” helper
