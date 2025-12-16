# Go port spec: `git gtr` (Git Worktree Runner)

This repository is a Go port of `~/src/github.com/coderabbitai/git-worktree-runner`.

Primary goal: keep the **user-facing CLI** compatible with upstream `git gtr`, while providing an idiomatic **Go library API** with roughly the same surface area as the upstream Bash “library” (`lib/*.sh`).

Hard requirement: implementation must use `github.com/go-git/go-git/v6`.

---

## 1) Scope, goals, non-goals

### Goals
- **CLI parity (v1):** support the upstream commands and flags:
  - `new`, `rm`, `go`, `run`, `editor`, `ai`, `copy`, `list|ls`, `clean`, `doctor`, `adapter`, `config`, `version`, `help`.
- **Repo-scoped worktrees:** worktrees are per-repository; branch names identify worktrees.
- **Config-over-flags:** defaults are set once (via git config and/or `.gtrconfig`), commands stay simple.
- **Scripting-friendly:** stable `--porcelain` output format for `list`.
- **Portable:** macOS/Linux/Windows (Git Bash + WSL) at minimum; handle path canonicalization and separators correctly.
- **Go library:** provide stable, testable, embeddable API for other Go programs.

### Non-goals (initially)
- Implementing every upstream “adapter script” behavior line-for-line (we’ll keep the same adapter names and expected commands, but re-implement in Go).
- Implementing every Git edge-case perfectly in v1 (e.g., unusual worktree states, exotic config include graphs) — but we’ll be explicit about behavior and test it.

---

## 2) User-facing CLI API (command spec)

The Go port will be installed primarily as a Git subcommand:

- Binary name: `git-gtr` (so users run it as `git gtr ...`).
- Optional convenience binary: `gtr` (may be provided, but upstream stopped doing this due to `coreutils gtr` conflicts; keep it optional).

### Common concepts
- **Identifier:** commands take `<id|branch|worktree-name>` where:
  - `1` means the **main repository working directory**.
  - If `<identifier>` equals the currently checked-out branch in the main repo, treat it as **main repo** (upstream behavior; reduces surprises).
  - If `<identifier>` matches an existing worktree folder name (after prefix), resolve directly to that worktree.
    - This is required to disambiguate multiple worktrees on the same branch created via `--force --name` (e.g., `feature-auth-backend`, `feature-auth-frontend`).
- **Repository context:** resolve paths/config from the repository’s **common git dir** (so running `git gtr` inside an existing linked worktree still targets the same repository-wide worktree set).
- **Worktree folder naming:**
  - default folder name: `sanitize(branch)`
  - if `--name <suffix>`: `sanitize(branch) + "-" + suffix`
  - optional prefix config: `gtr.worktrees.prefix` prepended to folder name.

### `git gtr new <branch> [options]`
Create a new worktree. Worktree folder name is derived from branch, optionally with `--name`.

Options (parity with upstream):
- `--from <ref>`: base ref for creating a new branch.
- `--from-current`: base ref is the current branch in the main repo (detached HEAD falls back to default branch).
- `--track <mode>`: `auto|remote|local|none`
  - `auto`: if remote exists and local does not, create local tracking branch; else use local; else create a new branch from `--from`/default.
  - `remote`: prefer `origin/<branch>`; fail if missing.
  - `local`: use existing local branch; fail if missing.
  - `none`: always create a new local branch from `--from`.
- `--no-copy`: skip file/dir copying.
- `--no-fetch`: skip fetching `origin` before resolving refs.
- `--force`: allow multiple worktrees for the same branch (requires `--name`).
- `--name <suffix>`: worktree folder suffix (required with `--force`).
- `--yes`: non-interactive (no prompts; missing required args becomes error).

Behavioral notes:
- Print human progress to stderr; keep stdout clean unless explicitly returning machine output.
- For `--force`, enforce `--name` to keep directories distinct (upstream).
- If `<branch>` is omitted and `--yes` is not set, prompt interactively for the branch name (upstream behavior).

### `git gtr rm <id|branch>... [options]`
Remove one or more worktrees.

Options:
- `--delete-branch`: also delete the branch (after removing the worktree).
- `--force`: force removal even if dirty (implementation-defined; aligns with upstream).
- `--yes`: non-interactive (no confirmation prompts).

### `git gtr go <id|branch>`
Print the absolute path of the worktree (or main repo for `1`) to **stdout** for shell navigation:

`cd "$(git gtr go my-branch)"`

Human messages (like “Branch: …”) go to stderr.

### `git gtr run <id|branch> <command...>`
Run an arbitrary command in the target directory. Exit code is the command’s exit code.

### `git gtr editor <id|branch> [--editor <name>]`
Open worktree in editor.

Resolution:
- `--editor` overrides config.
- otherwise uses `gtr.editor.default`.
- if editor is `none`, open in the OS file browser (best-effort).

### `git gtr ai <id|branch> [--ai <name>] [-- args...]`
Start an AI tool in the target directory.

Resolution:
- `--ai` overrides config.
- otherwise uses `gtr.ai.default`.
- arguments after `--` are passed to the tool.

If AI is `none`, return an error with a hint to configure `gtr.ai.default`.

### `git gtr copy <target>... [options] [-- <pattern>...]`
Copy files from a source (default main repo) into existing worktree(s).

Options:
- `--from <id|branch>`: source directory (default `1`).
- `-a, --all`: copy to all worktrees.
- `-n, --dry-run`: print what would be copied.

Patterns:
- Patterns after `--` override config.
- If no patterns provided, use merged patterns from:
  - `gtr.copy.include` + `.worktreeinclude` file.
- Exclusions are always applied from:
  - `gtr.copy.exclude`.

### `git gtr list [--porcelain]`
List main repo + worktrees.

Human mode:
- Table-like output: branch and path (main repo first), sorted by branch.

Porcelain mode:
- One line per entry:
  - `path<TAB>branch<TAB>status`
- `status` is one of: `ok|detached|locked|prunable|missing` (upstream names).

### `git gtr clean`
Clean stale/prunable worktrees and remove empty leftover directories.

### `git gtr doctor`
Health check: git availability (if required), repo discovery, config sanity, adapter availability.

### `git gtr adapter`
List built-in adapter names and whether they appear runnable on this system.

### `git gtr config {get|set|add|unset} <key> [value] [--global]`
Config management (wrapper around the chosen config backend):
- `get`: print value(s)
- `set`: set single value
- `add`: append multi-valued key
- `unset`: remove all values for key

### `git gtr version`
Print: `git gtr version X.Y.Z`

### `git gtr help`
Print a help page (similar content to upstream, not necessarily identical text).

### Output, verbosity, and prompts (behavioral contract)
To keep the CLI scriptable:
- **Stdout** is reserved for machine-consumable outputs:
  - `git gtr go` prints only the path to stdout.
  - `git gtr list --porcelain` prints only porcelain lines to stdout.
- **Stderr** receives all human-oriented output:
  - progress steps, warnings, errors, and interactive prompts.
- No implicit JSON output in v1. (`--porcelain` is the supported machine format.)
- `--yes` disables prompts; missing required inputs become hard errors.

---

## 3) Configuration model (keys, env vars, precedence)

### Supported config keys (same as upstream)
- `gtr.worktrees.dir`: base directory for worktrees.
- `gtr.worktrees.prefix`: prefix added to each worktree folder.
- `gtr.defaultBranch`: default branch for new branches (`auto|main|master|<branch>`).
- `gtr.editor.default`: default editor adapter name or `none`.
- `gtr.ai.default`: default AI adapter name or `none`.
- `gtr.copy.include` (multi): file globs to copy on create/copy.
- `gtr.copy.exclude` (multi): file globs to exclude.
- `gtr.copy.includeDirs` (multi): directory names to copy entirely (e.g., `node_modules`).
- `gtr.copy.excludeDirs` (multi): excluded directory patterns (e.g., `node_modules/.cache`, `*/.npm`).
- `gtr.hook.postCreate` (multi): commands executed after worktree creation.
- `gtr.hook.postRemove` (multi): commands executed after worktree removal.

### Supported env vars (same as upstream)
- `GTR_WORKTREES_DIR`
- `GTR_WORKTREES_PREFIX`
- `GTR_DEFAULT_BRANCH`
- `GTR_EDITOR_DEFAULT`
- `GTR_AI_DEFAULT`

### `.gtrconfig` and `.worktreeinclude`
- `.gtrconfig` lives in the **main worktree root** (i.e., the parent of the repository common dir) and uses gitconfig syntax. It is intended to be committed for team defaults.
- `.worktreeinclude` is also read from the **main worktree root** and uses `.gitignore`-style patterns for files to copy (merged into `gtr.copy.include`).

### Precedence (highest → lowest)
Match upstream semantics as closely as practical:
1. local git config (`.git/config`)
2. `.gtrconfig` (repo root)
3. global git config (`~/.gitconfig`)
4. system git config
5. environment variables
6. hard-coded defaults

Open question: do we depend on `git config` binary for (3)/(4), or implement config parsing ourselves? (See §6 and §10.)

---

## 4) Go library API (public surface)

Target import path: `github.com/zchee/git-worktree-runner/gtr`

### High-level API (recommended for users)

```go
package gtr

type Manager struct {
    // constructed via NewManager; holds resolved config and repo context
}

type ManagerOptions struct {
    // If empty, use os.Getwd().
    StartDir string

    // Non-interactive behavior: never prompt.
    Yes bool

    // Optional: override config sources (tests).
    Env map[string]string
}

func NewManager(ctx context.Context, opts ManagerOptions) (*Manager, error)

type Target struct {
    IsMain bool
    Path   string // absolute
    Branch string // "(detached)" allowed
}

func (m *Manager) ResolveTarget(ctx context.Context, identifier string) (Target, error)

type CreateWorktreeOptions struct {
    FromRef      string
    FromCurrent  bool
    TrackMode    TrackMode // Auto|Remote|Local|None
    NoCopy       bool
    NoFetch      bool
    Force        bool
    NameSuffix   string // required if Force
    Yes          bool    // overrides ManagerOptions.Yes
}

// CreateWorktree creates a new linked worktree for branch (CLI: `git gtr new`).
func (m *Manager) CreateWorktree(ctx context.Context, branch string, opts CreateWorktreeOptions) (Target, error)

type RemoveWorktreeOptions struct {
    DeleteBranch bool
    Force        bool
    Yes          bool
}

func (m *Manager) Remove(ctx context.Context, identifiers []string, opts RemoveWorktreeOptions) error

type RunOptions struct {
    Env   []string // KEY=VALUE
    Stdout io.Writer
    Stderr io.Writer
}

func (m *Manager) Run(ctx context.Context, identifier string, argv []string, opts RunOptions) (exitCode int, err error)

type CopyOptions struct {
    From      string
    All       bool
    DryRun    bool
    Patterns  []string // if empty, use config + .worktreeinclude
    PreservePaths bool // default true
}

func (m *Manager) Copy(ctx context.Context, targets []string, opts CopyOptions) error

type ListEntry struct {
    Target Target
    Status WorktreeStatus
}

func (m *Manager) List(ctx context.Context) ([]ListEntry, error)
```

### Low-level helpers (Bash-lib parity)
Expose equivalents for upstream `lib/*.sh` functions where they are stable and useful:
- `DiscoverRepoRoot(...)`
- `SanitizeBranchName(...)`
- `CanonicalizePath(...)`
- `ResolveBaseDir(...)`
- `ResolveDefaultBranch(...)`
- `CurrentBranch(...)`
- `WorktreeStatus(...)`

These will be thin wrappers over internal implementation, with full tests.

### Error taxonomy
Prefer typed errors for stable handling:
- `ErrNotInRepo`
- `ErrTargetNotFound`
- `ErrInvalidTrackMode`
- `ErrForceRequiresName`
- `ErrWorktreeAlreadyExists`
- `ErrUnsafePattern`
- `ErrNoPatterns`
- `ErrNoAIToolConfigured`

### CLI exit codes
Baseline (v1):
- `0`: success
- `1`: operational failure (git/IO/config/hook failures, target not found, etc.)
- `2`: usage error (invalid flags/args, missing required params in `--yes` mode)

Special case:
- `git gtr run ...` exits with the executed command’s exit code (and only uses `1/2` for its own argument/target resolution failures).

---

## 5) Project structure (Go)

Proposed layout:

```
cmd/
  git-gtr/              # main CLI entry (git subcommand)
  gtr/                  # optional convenience wrapper (maybe)
gtr/                    # public library API
internal/
  cli/                  # command parsing, help text, porcelain formatting
  config/               # config resolution: git config + .gtrconfig + env
  gitx/                 # go-git repository helpers (refs, remotes, HEAD)
  worktree/             # create/remove/list/status + locking
  copy/                 # file/dir copy logic
  hooks/                # hook execution
  adapters/             # editor/ai adapter registry + exec
  ui/                   # logging, prompts (stderr), colors (optional)
```

Guiding rule: CLI is thin; business logic lives in `gtr/` + `internal/*`.

---

## 6) go-git/v6 usage and backend strategy

### Reality check: “worktree” in go-git
`go-git`’s `Worktree` type is a working directory + index abstraction (like `git add/checkout`), not a wrapper for `git worktree add`.

However, go-git *does* understand `.git` files (`gitdir: …`) and `commondir`, which are the building blocks of Git’s linked worktrees.

### Locking and concurrency (must-have)
Even when we delegate to the system `git` (Strategy A), we still need **process-level serialization** for `gtr` operations that mutate:
- the worktree administrative area (`.git/worktrees/*`)
- the configured base directory (creating/removing worktree folders)
- post-create copy + hooks (to avoid concurrent copies clobbering each other)

Proposal:
- Use a **single per-repository lock** acquired for the duration of:
  - `new`, `rm`, `copy`, `clean`
- Do **not** lock for:
  - `go`, `run`, `editor`, `ai`, `list` (read-only or user-driven side effects)

Lock file location:
- Prefer the repo’s *common* git dir (worktree-safe): `<git-common-dir>/gtr.lock`
  - This should work whether invoked from the main repo or any linked worktree.

Lock mechanics (portable):
- Unix: `flock(2)` advisory lock on the file.
- Windows: `LockFileEx` on the file.
- If we want to avoid extra deps, we can implement with `golang.org/x/sys/{unix,windows}`.

Failure mode:
- If lock acquisition times out (default, e.g., 30s), return a clear error:
  - “Another git gtr operation is in progress for this repository.”

### Two viable implementation strategies

#### Strategy A (safer v1): shell out to system `git` for worktree plumbing
- Use go-git for:
  - repo discovery, reading refs, determining current branch, default branch heuristics.
  - parsing `.gtrconfig` (gitconfig syntax) where helpful.
- Use `os/exec` for:
  - `git worktree add/remove/list/prune`
  - `git fetch` (if we want identical behavior to upstream)
  - `git config` (global/system precedence and include support)

Implementation notes:
- Always open repos with `git.PlainOpenWithOptions(..., DetectDotGit=true, EnableDotGitCommonDir=true)` so running `git gtr` *inside an existing worktree* still resolves the main repo’s common directory correctly.

Pros:
- Maximum compatibility with Git semantics and edge cases.
- Much less risk compared to re-implementing `git worktree`.

Cons:
- Requires `git` binary in PATH (same as upstream).
- More subprocesses; must be careful with quoting, output, and Windows behavior.

#### Strategy B (ambitious v2): native linked-worktree creation via filesystem + go-git
- Create linked worktrees by writing:
  - worktree `.git` file (`gitdir: <common>/.git/worktrees/<id>`)
  - admin dir files: `commondir`, `gitdir`, `HEAD`, and necessary scaffolding
- Then use go-git checkout/reset to populate working directory and index.

Pros:
- Single static binary can work even without `git` (except for user workflows that rely on git).
- Full control and testability.

Cons:
- High risk of subtle incompatibilities with Git versions/platforms.
- Requires rigorous integration testing with real Git CLI to validate produced worktrees are usable.

Decision (proposed):
- Implement Strategy A first for correctness; keep internal interfaces so Strategy B can be added later without changing CLI or public library API.

---

## 7) Testing strategy (no mocks)

We will rely on **integration-style tests** using real temporary repositories and filesystem operations.

Test categories:
1. **Pure unit tests** (fast):
   - branch name sanitization
   - path canonicalization and base dir resolution
   - config precedence (where deterministic)
   - pattern validation (reject `..` and absolute paths)
2. **Repo integration tests** (real git repo on disk):
   - create/remove/list targets
   - branch tracking modes (`auto|remote|local|none`)
   - `go` output (stdout vs stderr behavior)
   - copy patterns + exclude behavior
   - hooks execution environment variables
3. **Cross-platform behavior** (CI matrix):
   - macOS, Linux, Windows (Git Bash/WSL)

If Strategy A is used, tests will invoke the system `git` binary (this is not a “mock service”; it’s an integration dependency).

---

## 8) Compatibility notes vs upstream

- Keep CLI flags/behavior stable where users depend on them (`--porcelain`, `--` separator, special id `1`).
- Adapters: keep upstream adapter names (editor: `cursor`, `vscode`, `zed`, …; ai: `aider`, `claude`, `codex`, …), but implement in Go:
  - “ready/missing” is determined by `exec.LookPath` rules.
  - allow custom commands via config/env (equivalent to upstream “generic adapter fallback”).
- Logging format: keep the same *intent* (OK/warn/error/step to stderr), not exact text.

---

## 9) Implementation plan (detailed checklist)

This plan is intentionally granular to keep progress measurable and to surface risk early.

1. Create `cmd/git-gtr` skeleton and top-level `main`.
2. Implement stderr/stdout discipline (porcelain vs human output).
3. Add a minimal command router (subcommands + help/version).
4. Add `gtr` public package with `Manager` constructor.
5. Implement repo discovery (search parents for `.git`, handle `.git` file).
6. Open repository via `go-git/v6` with `EnableDotGitCommonDir=true`.
7. Implement branch name sanitization (match upstream char class + trim).
8. Implement path canonicalization (resolve symlinks, absolute paths).
9. Implement base dir resolution (`gtr.worktrees.dir` / env / default sibling).
10. Implement warning when base dir is inside repo and not ignored.
11. Implement default branch resolution:
    - config override
    - else detect `origin/HEAD` equivalent (via refs), fallback to `origin/main|master`.
12. Define worktree naming rules (prefix + sanitized + optional suffix).
13. Implement `ResolveTarget` including special `1` and “identifier == main current branch”.
14. Implement `list --porcelain` format and stable sorting.
15. Implement `new` flags parsing and validation (`--force` requires `--name`).
16. Implement `track` modes mapping to git behaviors.
17. Implement worktree creation backend (Strategy A first):
    - `fetch` (unless `--no-fetch`)
    - `worktree add` + tracking branch logic for `auto`
18. Implement `.worktreeinclude` parsing.
19. Implement file glob matching with `**` semantics (choose approach; add tests).
20. Implement safe pattern validation (reject absolute/`..`).
21. Implement file copy (preserve paths by default) + dry-run.
22. Implement includeDirs/excludeDirs directory copying + exclusion pruning; add tests.
23. Implement hooks:
    - read multi-valued hook configs
    - run in correct directory
    - export `REPO_ROOT`, `WORKTREE_PATH`, `BRANCH`
24. Implement `rm`:
    - remove worktree
    - optional branch deletion
    - postRemove hooks
25. Implement `go`:
    - path to stdout
    - metadata to stderr
26. Implement `run`:
    - run command in target dir
    - propagate exit code
27. Implement `editor`:
    - adapter registry
    - fallback to file browser if editor `none`
28. Implement `ai`:
    - adapter registry
    - passthrough args after `--`
29. Implement `copy`:
    - resolve source and targets
    - `--all` support
30. Implement `clean`:
    - prune stale worktrees (Strategy A: `git worktree prune`)
    - remove empty directories in base dir
31. Implement `doctor` diagnostics.
32. Implement `adapter` listing.
33. Implement `config` command (scope local/global) via chosen backend.
34. Add exhaustive tests for each exported function/method.
35. Add CI jobs for macOS/Linux/Windows.
36. Add docs: README usage + install instructions.
37. Add release packaging (optional).

---

## 10) Open questions (need user decision)

1. **Backend:** Are we allowed to depend on the system `git` binary for worktree operations (`git worktree add/remove/list/prune`) in v1, or must this be 100% native Go?
2. **Config precedence:** Should we support global/system gitconfig with full include semantics (likely requires shelling out to `git config`), or is “local + .gtrconfig + env” acceptable for v1?
3. **Compatibility target:** Do we need to preserve “worktree is the same branch as main repo => treat as main repo” behavior? (It’s in upstream; it’s a bit surprising but useful.)
