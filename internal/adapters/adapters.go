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

package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/google/shlex"
)

// Kind identifies an adapter type.
type Kind string

const (
	KindEditor Kind = "editor"
	KindAI     Kind = "ai"
)

// Mode determines whether a command is started (detached) or run to completion.
type Mode int

const (
	ModeRun Mode = iota
	ModeStart
)

// Spec describes how to execute an adapter.
type Spec struct {
	Name    string
	Command string
	Args    []string
	Dir     string
	Mode    Mode
}

// Info describes an adapter's availability.
type Info struct {
	Kind   Kind
	Name   string
	Status string // "[ready]" or "[missing]"
	Notes  string
}

// ResolveEditor returns the execution spec for an editor adapter.
func ResolveEditor(name, path string) (Spec, error) {
	switch name {
	case "cursor":
		return Spec{Name: name, Command: "cursor", Args: []string{path}, Mode: ModeStart}, nil
	case "vscode":
		return Spec{Name: name, Command: "code", Args: []string{path}, Mode: ModeStart}, nil
	case "zed":
		return Spec{Name: name, Command: "zed", Args: []string{path}, Mode: ModeStart}, nil
	case "idea":
		return Spec{Name: name, Command: "idea", Args: []string{path}, Mode: ModeStart}, nil
	case "pycharm":
		return Spec{Name: name, Command: "pycharm", Args: []string{path}, Mode: ModeStart}, nil
	case "webstorm":
		return Spec{Name: name, Command: "webstorm", Args: []string{path}, Mode: ModeStart}, nil
	case "sublime":
		return Spec{Name: name, Command: "subl", Args: []string{path}, Mode: ModeStart}, nil
	case "atom":
		return Spec{Name: name, Command: "atom", Args: []string{path}, Mode: ModeStart}, nil
	case "emacs":
		return Spec{Name: name, Command: "emacs", Args: []string{path}, Mode: ModeStart}, nil
	case "vim":
		return Spec{Name: name, Command: "vim", Args: []string{"."}, Dir: path, Mode: ModeRun}, nil
	case "nvim":
		return Spec{Name: name, Command: "nvim", Args: []string{"."}, Dir: path, Mode: ModeRun}, nil
	case "nano":
		shell := os.Getenv("SHELL")
		if shell == "" && runtime.GOOS != "windows" {
			shell = "/bin/sh"
		}
		if shell == "" {
			// On Windows, prefer ComSpec.
			shell = os.Getenv("ComSpec")
		}
		if shell == "" {
			return Spec{}, fmt.Errorf("cannot determine shell for nano adapter")
		}
		return Spec{Name: name, Command: shell, Args: nil, Dir: path, Mode: ModeRun}, nil
	default:
		// Custom editor command: interpret "name" as a command line string and append the path as a single argument.
		argv, err := shlex.Split(name)
		if err != nil {
			return Spec{}, err
		}
		if len(argv) == 0 {
			return Spec{}, errors.New("empty editor command")
		}
		return Spec{Name: name, Command: argv[0], Args: append(argv[1:], path), Mode: ModeRun}, nil
	}
}

// ResolveAI returns the execution spec for an AI adapter.
func ResolveAI(name, dir string, extraArgs []string) (Spec, error) {
	switch name {
	case "aider":
		return Spec{Name: name, Command: "aider", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
	case "codex":
		return Spec{Name: name, Command: "codex", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
	case "continue":
		return Spec{Name: name, Command: "cn", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
	case "gemini":
		return Spec{Name: name, Command: "gemini", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
	case "opencode":
		return Spec{Name: name, Command: "opencode", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
	case "cursor":
		if _, err := exec.LookPath("cursor-agent"); err == nil {
			return Spec{Name: name, Command: "cursor-agent", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
		}
		// Upstream tries `cursor cli ...` first (CLI shape varies by version), then falls back to `cursor ...`.
		return Spec{Name: name, Command: "cursor", Args: append([]string{"cli"}, extraArgs...), Dir: dir, Mode: ModeRun}, nil
	case "claude":
		home, _ := os.UserHomeDir()
		candidate := filepath.Join(home, ".claude", "local", "claude")
		if fi, err := os.Stat(candidate); err == nil && fi.Mode().IsRegular() {
			return Spec{Name: name, Command: candidate, Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
		}
		if _, err := exec.LookPath("claude"); err == nil {
			return Spec{Name: name, Command: "claude", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
		}
		if _, err := exec.LookPath("claude-code"); err == nil {
			return Spec{Name: name, Command: "claude-code", Args: extraArgs, Dir: dir, Mode: ModeRun}, nil
		}
		return Spec{}, fmt.Errorf("Claude Code not found")
	default:
		argv, err := shlex.Split(name)
		if err != nil {
			return Spec{}, err
		}
		if len(argv) == 0 {
			return Spec{}, errors.New("empty ai command")
		}
		return Spec{Name: name, Command: argv[0], Args: append(argv[1:], extraArgs...), Dir: dir, Mode: ModeRun}, nil
	}
}

// Exec executes spec with stdio attached. For ModeStart, it starts and returns without waiting.
func Exec(ctx context.Context, spec Spec, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if spec.Mode == ModeStart {
		cmd := exec.CommandContext(ctx, spec.Command, spec.Args...) //nolint:gosec
		cmd.Dir = spec.Dir
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			return 1, err
		}
		return 0, nil
	}

	// Cursor CLI has multiple invocation styles depending on version.
	// Upstream tries `cursor cli ...` first and falls back to `cursor ...`.
	if spec.Name == "cursor" && filepath.Base(spec.Command) == "cursor" && len(spec.Args) > 0 && spec.Args[0] == "cli" {
		cmd := exec.CommandContext(ctx, spec.Command, spec.Args...) //nolint:gosec
		cmd.Dir = spec.Dir
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = io.Discard

		if err := cmd.Run(); err == nil {
			return 0, nil
		} else {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return 1, err
			}
		}

		cmdFallback := exec.CommandContext(ctx, spec.Command, spec.Args[1:]...) //nolint:gosec
		cmdFallback.Dir = spec.Dir
		cmdFallback.Stdin = stdin
		cmdFallback.Stdout = stdout
		cmdFallback.Stderr = stderr

		if err := cmdFallback.Run(); err == nil {
			return 0, nil
		} else {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode(), err
			}
			return 1, err
		}
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...) //nolint:gosec
	cmd.Dir = spec.Dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err == nil {
		return 0, nil
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
}

// ListBuiltins returns the built-in adapter names for kind.
func ListBuiltins(kind Kind) []string {
	switch kind {
	case KindEditor:
		return []string{"atom", "cursor", "emacs", "idea", "nano", "nvim", "pycharm", "sublime", "vim", "vscode", "webstorm", "zed"}
	case KindAI:
		return []string{"aider", "claude", "codex", "continue", "cursor", "gemini", "opencode"}
	default:
		return nil
	}
}

// Probe returns availability info for built-in adapters.
func Probe(ctx context.Context, kind Kind) ([]Info, error) {
	_ = ctx

	names := ListBuiltins(kind)
	var out []Info
	for _, name := range names {
		info := Info{Kind: kind, Name: name, Status: "[missing]"}
		switch kind {
		case KindEditor:
			_, err := editorLookPath(name)
			if err == nil {
				info.Status = "[ready]"
			} else {
				info.Notes = "Not found in PATH"
			}
		case KindAI:
			_, err := aiLookPath(name)
			if err == nil {
				info.Status = "[ready]"
			} else {
				info.Notes = "Not found in PATH"
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func editorLookPath(name string) (string, error) {
	switch name {
	case "cursor":
		return exec.LookPath("cursor")
	case "vscode":
		return exec.LookPath("code")
	case "zed":
		return exec.LookPath("zed")
	case "idea":
		return exec.LookPath("idea")
	case "pycharm":
		return exec.LookPath("pycharm")
	case "webstorm":
		return exec.LookPath("webstorm")
	case "vim":
		return exec.LookPath("vim")
	case "nvim":
		return exec.LookPath("nvim")
	case "emacs":
		return exec.LookPath("emacs")
	case "sublime":
		return exec.LookPath("subl")
	case "nano":
		return exec.LookPath("nano")
	case "atom":
		return exec.LookPath("atom")
	default:
		return "", fmt.Errorf("unknown editor adapter: %s", name)
	}
}

func aiLookPath(name string) (string, error) {
	switch name {
	case "aider":
		return exec.LookPath("aider")
	case "codex":
		return exec.LookPath("codex")
	case "continue":
		return exec.LookPath("cn")
	case "cursor":
		if p, err := exec.LookPath("cursor-agent"); err == nil {
			return p, nil
		}
		return exec.LookPath("cursor")
	case "gemini":
		return exec.LookPath("gemini")
	case "opencode":
		return exec.LookPath("opencode")
	case "claude":
		home, _ := os.UserHomeDir()
		candidate := filepath.Join(home, ".claude", "local", "claude")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if p, err := exec.LookPath("claude"); err == nil {
			return p, nil
		}
		return exec.LookPath("claude-code")
	default:
		return "", fmt.Errorf("unknown ai adapter: %s", name)
	}
}
