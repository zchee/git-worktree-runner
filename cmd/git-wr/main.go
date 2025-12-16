// Copyright 2025 The git-worktree-runner Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/zchee/git-worktree-runner/internal/adapters"
	"github.com/zchee/git-worktree-runner/internal/version"
	"github.com/zchee/git-worktree-runner/wr"
)

const (
	exitSuccess   = 0
	exitFailure   = 1
	exitUsage     = 2
	versionPrefix = "git wr version"
)

type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func newExitCodeError(code int) error {
	return &exitCodeError{code: code}
}

func errorFromExitCode(code int) error {
	if code == exitSuccess {
		return nil
	}
	return newExitCodeError(code)
}

// VersionInfo describes the build version displayed to users.
type VersionInfo struct {
	Version string
}

// Runner executes the `git-wr` CLI.
type Runner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Version VersionInfo
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// Run runs the CLI using the provided context and returns the desired process exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	runner := Runner{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Version: VersionInfo{
			Version: version.Version,
		},
	}

	return runner.Run(ctx, args)
}

// Run executes the CLI and returns a process exit code.
func (r Runner) Run(ctx context.Context, args []string) int {
	cmd := r.newRootCommand()
	cmd.SetArgs(args)
	cmd.SetIn(r.Stdin)
	cmd.SetOut(r.Stdout)
	cmd.SetErr(r.Stderr)

	if err := cmd.ExecuteContext(ctx); err == nil {
		return exitSuccess
	} else {
		var exitErr *exitCodeError
		if errors.As(err, &exitErr) {
			return exitErr.code
		}
		if unknownCmd, ok := parseUnknownCommand(err); ok {
			fmt.Fprintf(r.Stderr, "Unknown command: %s\n", unknownCmd)
			fmt.Fprint(r.Stderr, "Use 'git wr help' for available commands\n")
			return exitUsage
		}
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitUsage
	}
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `git wr - Git worktree runner

PHILOSOPHY: Configuration over flags. Set defaults once, then use simple commands.

USAGE:
  git wr <command> [args...]

CORE COMMANDS:
  new <branch> [options]      Create a new worktree
  rm <id|name>... [options]   Remove worktree(s)
  go <id|name>                Print worktree path for shell navigation
  run <id|name> <cmd...>      Run a command in a worktree
  list [--porcelain]          List worktrees

INTEGRATIONS:
  editor <id|name> [--editor <name>]     Open worktree in editor
  ai <id|name> [--ai <name>] [-- args]   Start AI tool in worktree

SETUP & MAINTENANCE:
  copy <target>... [-- <pattern>...]     Copy files between worktrees
  clean                                 Remove stale/prunable worktrees
  doctor                                Health check
  adapter                               List adapters
  config {get|set|add|unset} <key> ...   Manage configuration
  version                               Show version
  help                                  Show this help
`)
}

func (r Runner) newRootCommand() *cobra.Command {
	showVersion := false

	root := &cobra.Command{
		Use:           "wr",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			writeHelp(cmd.OutOrStdout())
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !showVersion {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", versionPrefix, r.Version.Version)
			return newExitCodeError(exitSuccess)
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = args
		writeHelp(cmd.OutOrStdout())
	})

	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version.")

	root.AddCommand(
		r.newCommand("list", []string{"ls"}, r.runList),
		r.newCommand("go", nil, r.runGo),
		r.newCommand("run", nil, r.runRun),
		r.newCommand("new", nil, r.runNew),
		r.newCommand("rm", nil, r.runRemove),
		r.newCommand("copy", nil, r.runCopy),
		r.newCommand("config", nil, r.runConfig),
		r.newCommand("editor", nil, r.runEditor),
		r.newCommand("ai", nil, r.runAI),
		r.newCommand("clean", nil, r.runClean),
		r.newCommand("doctor", nil, r.runDoctor),
		r.newCommand("adapter", []string{"adapters"}, r.runAdapters),
		r.versionCommand(),
	)

	return root
}

func (r Runner) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "version",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", versionPrefix, r.Version.Version)
			return nil
		},
	}
}

func (r Runner) newCommand(use string, aliases []string, run func(ctx context.Context, args []string) int) *cobra.Command {
	cmd := &cobra.Command{
		Use:                use,
		Aliases:            aliases,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errorFromExitCode(run(cmd.Context(), args))
		},
	}
	return cmd
}

func (r Runner) newManager(ctx context.Context) (*wr.Manager, error) {
	m, err := wr.NewManager(ctx, wr.ManagerOptions{})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func parseUnknownCommand(err error) (cmd string, ok bool) {
	msg := err.Error()
	const prefix = "unknown command \""
	if !strings.HasPrefix(msg, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(msg, prefix)
	before, _, ok0 := strings.Cut(rest, "\"")
	if !ok0 {
		return "", false
	}
	return before, true
}

func (r Runner) runList(ctx context.Context, args []string) int {
	porcelain := false
	for _, a := range args {
		if a == "--porcelain" {
			porcelain = true
		}
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	entries, err := m.List(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	if porcelain {
		for _, e := range entries {
			fmt.Fprintf(r.Stdout, "%s\t%s\t%s\n", e.Target.Path, e.Target.Branch, e.Status)
		}
		return exitSuccess
	}

	fmt.Fprintln(r.Stdout, "Git Worktrees")
	fmt.Fprintln(r.Stdout)
	fmt.Fprintf(r.Stdout, "%-30s %s\n", "BRANCH", "PATH")
	fmt.Fprintf(r.Stdout, "%-30s %s\n", "------", "----")

	for _, e := range entries {
		branch := e.Target.Branch
		if e.Target.IsMain {
			branch += " [main repo]"
		}
		fmt.Fprintf(r.Stdout, "%-30s %s\n", branch, e.Target.Path)
	}

	fmt.Fprintln(r.Stdout)
	fmt.Fprintln(r.Stdout, "Tip: Use 'git wr list --porcelain' for machine-readable output")
	return exitSuccess
}

func (r Runner) promptLine(prompt string) (string, error) {
	fmt.Fprintf(r.Stderr, "[?] %s ", prompt)
	reader := bufio.NewReader(r.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (r Runner) promptYesNo(prompt string) (bool, error) {
	line, err := r.promptLine(prompt + " [y/N]:")
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (r Runner) runNew(ctx context.Context, args []string) int {
	var (
		branch      string
		fromRef     string
		fromCurrent bool
		trackMode   = "auto"
		noCopy      bool
		noFetch     bool
		force       bool
		nameSuffix  string
		yes         bool
	)
	for i := 0; i < len(args); {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --from requires a value")
				return exitUsage
			}
			fromRef = args[i+1]
			i += 2
		case "--from-current":
			fromCurrent = true
			i++
		case "--track":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --track requires a value")
				return exitUsage
			}
			trackMode = args[i+1]
			i += 2
		case "--no-copy":
			noCopy = true
			i++
		case "--no-fetch":
			noFetch = true
			i++
		case "--force":
			force = true
			i++
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --name requires a value")
				return exitUsage
			}
			nameSuffix = args[i+1]
			i += 2
		case "--yes":
			yes = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(r.Stderr, "[x] Unknown flag: %s\n", args[i])
				return exitUsage
			}
			if branch != "" {
				fmt.Fprintln(r.Stderr, "[x] Usage: git wr new <branch> [options]")
				return exitUsage
			}
			branch = args[i]
			i++
		}
	}

	if branch == "" {
		if yes {
			fmt.Fprintln(r.Stderr, "[x] Branch name required in non-interactive mode (--yes)")
			return exitUsage
		}
		var err error
		branch, err = r.promptLine("Enter branch name:")
		if err != nil {
			fmt.Fprintf(r.Stderr, "[x] %v\n", err)
			return exitFailure
		}
		if branch == "" {
			fmt.Fprintln(r.Stderr, "[x] Branch name required")
			return exitUsage
		}
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	target, err := m.CreateWorktree(ctx, branch, wr.CreateWorktreeOptions{
		FromRef:     fromRef,
		FromCurrent: fromCurrent,
		TrackMode:   wr.TrackMode(trackMode),
		NoCopy:      noCopy,
		NoFetch:     noFetch,
		Force:       force,
		NameSuffix:  nameSuffix,
	})
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	fmt.Fprintf(r.Stderr, "[OK] Worktree created: %s\n", target.Path)
	return exitSuccess
}

func (r Runner) runRemove(ctx context.Context, args []string) int {
	var (
		deleteBranch bool
		force        bool
		yes          bool
		idents       []string
	)
	for i := 0; i < len(args); {
		switch args[i] {
		case "--delete-branch":
			deleteBranch = true
			i++
		case "--force":
			force = true
			i++
		case "--yes":
			yes = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(r.Stderr, "[x] Unknown flag: %s\n", args[i])
				return exitUsage
			}
			idents = append(idents, args[i])
			i++
		}
	}

	if len(idents) == 0 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr rm <id|branch|worktree-name> [<id|branch|worktree-name>...]")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	opts := wr.RemoveWorktreeOptions{
		DeleteBranch: deleteBranch,
		Force:        force,
		Yes:          yes,
	}
	if deleteBranch && !yes {
		opts.ConfirmDeleteBranch = func(ctx context.Context, branch string) (bool, error) {
			_ = ctx
			return r.promptYesNo(fmt.Sprintf("Also delete branch %q?", branch))
		}
	}

	if err := m.Remove(ctx, idents, opts); err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	return exitSuccess
}

func (r Runner) runCopy(ctx context.Context, args []string) int {
	source := "1"
	allMode := false
	dryRun := false
	var targets []string
	var patterns []string

	for i := 0; i < len(args); {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --from requires a value")
				return exitUsage
			}
			source = args[i+1]
			i += 2
		case "-a", "--all":
			allMode = true
			i++
		case "-n", "--dry-run":
			dryRun = true
			i++
		case "--":
			i++
			patterns = append(patterns, args[i:]...)
			i = len(args)
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(r.Stderr, "[x] Unknown flag: %s\n", args[i])
				return exitUsage
			}
			targets = append(targets, args[i])
			i++
		}
	}

	if !allMode && len(targets) == 0 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr copy <target>... [-n] [-a] [--from <source>] [-- <pattern>...]")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	results, err := m.Copy(ctx, targets, wr.CopyOptions{
		From:          source,
		All:           allMode,
		DryRun:        dryRun,
		Patterns:      patterns,
		PreservePaths: true,
	})
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	for _, res := range results {
		if dryRun {
			fmt.Fprintf(r.Stderr, "==> [dry-run] Would copy to: %s\n", res.Target.Branch)
		} else {
			fmt.Fprintf(r.Stderr, "==> Copying to: %s\n", res.Target.Branch)
		}
		for _, f := range res.CopiedFiles {
			if dryRun {
				fmt.Fprintf(r.Stderr, "[dry-run] Would copy: %s\n", f)
			} else {
				fmt.Fprintf(r.Stderr, "Copied %s\n", f)
			}
		}
	}

	return exitSuccess
}

func (r Runner) runGo(ctx context.Context, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr go <id|branch|worktree-name>")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	target, err := m.ResolveTarget(ctx, args[0])
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	if target.IsMain {
		fmt.Fprintln(r.Stderr, "Main repo")
	} else {
		fmt.Fprintf(r.Stderr, "Worktree: %s\n", target.Branch)
	}
	fmt.Fprintf(r.Stderr, "Branch: %s\n", target.Branch)

	fmt.Fprintln(r.Stdout, target.Path)
	return exitSuccess
}

func (r Runner) runConfig(ctx context.Context, args []string) int {
	global := false
	action := ""
	key := ""
	value := ""

	for _, a := range args {
		switch a {
		case "--global", "global":
			global = true
		case "get", "set", "add", "unset":
			if action == "" {
				action = a
			}
		default:
			if key == "" {
				key = a
				continue
			}
			if value == "" && (action == "set" || action == "add") {
				value = a
				continue
			}
		}
	}

	if action == "" {
		action = "get"
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	switch action {
	case "get":
		if key == "" {
			fmt.Fprintln(r.Stderr, "[x] Usage: git wr config get <key> [--global]")
			return exitUsage
		}
		values, err := m.ConfigGet(ctx, key, global)
		if err != nil {
			fmt.Fprintf(r.Stderr, "[x] %v\n", err)
			return exitFailure
		}
		for _, v := range values {
			fmt.Fprintln(r.Stdout, v)
		}
		return exitSuccess

	case "set":
		if key == "" || value == "" {
			fmt.Fprintln(r.Stderr, "[x] Usage: git wr config set <key> <value> [--global]")
			return exitUsage
		}
		if err := m.ConfigSet(ctx, key, value, global); err != nil {
			fmt.Fprintf(r.Stderr, "[x] %v\n", err)
			return exitFailure
		}
		fmt.Fprintf(r.Stderr, "[OK] Config set: %s = %s\n", key, value)
		return exitSuccess

	case "add":
		if key == "" || value == "" {
			fmt.Fprintln(r.Stderr, "[x] Usage: git wr config add <key> <value> [--global]")
			return exitUsage
		}
		if err := m.ConfigAdd(ctx, key, value, global); err != nil {
			fmt.Fprintf(r.Stderr, "[x] %v\n", err)
			return exitFailure
		}
		fmt.Fprintf(r.Stderr, "[OK] Config added: %s = %s\n", key, value)
		return exitSuccess

	case "unset":
		if key == "" {
			fmt.Fprintln(r.Stderr, "[x] Usage: git wr config unset <key> [--global]")
			return exitUsage
		}
		if err := m.ConfigUnset(ctx, key, global); err != nil {
			fmt.Fprintf(r.Stderr, "[x] %v\n", err)
			return exitFailure
		}
		fmt.Fprintf(r.Stderr, "[OK] Config unset: %s\n", key)
		return exitSuccess

	default:
		fmt.Fprintf(r.Stderr, "[x] Unknown config action: %s\n", action)
		return exitUsage
	}
}

func (r Runner) runEditor(ctx context.Context, args []string) int {
	editor := ""
	identifier := ""

	for i := 0; i < len(args); {
		switch args[i] {
		case "--editor":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --editor requires a value")
				return exitUsage
			}
			editor = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(r.Stderr, "[x] Unknown flag: %s\n", args[i])
				return exitUsage
			}
			if identifier != "" {
				fmt.Fprintln(r.Stderr, "[x] Usage: git wr editor <id|branch|worktree-name> [--editor <name>]")
				return exitUsage
			}
			identifier = args[i]
			i++
		}
	}

	if identifier == "" {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr editor <id|branch|worktree-name> [--editor <name>]")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	exitCode, err := m.OpenEditor(ctx, identifier, editor, wr.ExecIO{
		Stdin:  r.Stdin,
		Stdout: r.Stdout,
		Stderr: r.Stderr,
	})
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		if exitCode != 0 {
			return exitCode
		}
		return exitFailure
	}

	return exitCode
}

func (r Runner) runAI(ctx context.Context, args []string) int {
	tool := ""
	identifier := ""
	var toolArgs []string

	for i := 0; i < len(args); {
		switch args[i] {
		case "--ai":
			if i+1 >= len(args) {
				fmt.Fprintln(r.Stderr, "[x] --ai requires a value")
				return exitUsage
			}
			tool = args[i+1]
			i += 2
		case "--":
			toolArgs = append(toolArgs, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(r.Stderr, "[x] Unknown flag: %s\n", args[i])
				return exitUsage
			}
			if identifier != "" {
				fmt.Fprintln(r.Stderr, "[x] Usage: git wr ai <id|branch|worktree-name> [--ai <name>] [-- args...]")
				return exitUsage
			}
			identifier = args[i]
			i++
		}
	}

	if identifier == "" {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr ai <id|branch|worktree-name> [--ai <name>] [-- args...]")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	exitCode, err := m.RunAI(ctx, identifier, tool, toolArgs, wr.ExecIO{
		Stdin:  r.Stdin,
		Stdout: r.Stdout,
		Stderr: r.Stderr,
	})
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitCode
	}

	return exitCode
}

func (r Runner) runClean(ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr clean")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	result, err := m.Clean(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	if len(result.RemovedEmptyDirs) == 0 {
		fmt.Fprintln(r.Stderr, "[OK] Cleanup complete (no empty directories found)")
		return exitSuccess
	}
	fmt.Fprintf(r.Stderr, "[OK] Cleanup complete (%d directories removed)\n", len(result.RemovedEmptyDirs))
	return exitSuccess
}

func (r Runner) runDoctor(ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr doctor")
		return exitUsage
	}

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	report, err := m.Doctor(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	wr.WriteDoctorReport(r.Stdout, report)
	return exitSuccess
}

func (r Runner) runAdapters(ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr adapter")
		return exitUsage
	}

	fmt.Fprintln(r.Stdout, "Available Adapters")
	fmt.Fprintln(r.Stdout)

	fmt.Fprintln(r.Stdout, "Editor Adapters:")
	fmt.Fprintln(r.Stdout)
	fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", "NAME", "STATUS", "NOTES")
	fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", "---------------", "------------", "-----")

	editors, _ := adapters.Probe(ctx, adapters.KindEditor)
	for _, a := range editors {
		fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", a.Name, a.Status, a.Notes)
	}

	fmt.Fprintln(r.Stdout)
	fmt.Fprintln(r.Stdout, "AI Tool Adapters:")
	fmt.Fprintln(r.Stdout)
	fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", "NAME", "STATUS", "NOTES")
	fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", "---------------", "------------", "-----")

	ais, _ := adapters.Probe(ctx, adapters.KindAI)
	for _, a := range ais {
		fmt.Fprintf(r.Stdout, "%-15s %-12s %s\n", a.Name, a.Status, a.Notes)
	}

	return exitSuccess
}

func (r Runner) runRun(ctx context.Context, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(r.Stderr, "[x] Usage: git wr run <id|branch|worktree-name> <command...>")
		return exitUsage
	}

	identifier := args[0]
	command := args[1:]

	m, err := r.newManager(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	target, err := m.ResolveTarget(ctx, identifier)
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	if target.IsMain {
		fmt.Fprintln(r.Stderr, "==> Running in: main repo")
	} else {
		fmt.Fprintf(r.Stderr, "==> Running in: %s\n", target.Branch)
	}
	fmt.Fprintf(r.Stderr, "Command: %s\n\n", strings.Join(command, " "))

	exitCode, err := m.Run(ctx, identifier, command, wr.RunOptions{
		IO: wr.ExecIO{
			Stdin:  r.Stdin,
			Stdout: r.Stdout,
			Stderr: r.Stderr,
		},
	})
	if err != nil {
		fmt.Fprintf(r.Stderr, "[x] %v\n", err)
		return exitFailure
	}

	return exitCode
}
