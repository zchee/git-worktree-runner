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

package gitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Git runs the system `git` command.
type Git struct {
	Path string
	Env  []string
}

// New returns a Git runner that executes the `git` binary found in PATH.
func New() (Git, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return Git{}, fmt.Errorf("find git in PATH: %w", err)
	}

	return Git{Path: path}, nil
}

// Result contains the captured output of a command invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ExitError is returned when a command exits with a non-zero status.
type ExitError struct {
	Path     string
	Args     []string
	Dir      string
	ExitCode int
	Stderr   string
}

func (e *ExitError) Error() string {
	if len(e.Args) == 0 {
		return fmt.Sprintf("%s failed (exit %d)", e.Path, e.ExitCode)
	}

	cmd := strings.Join(append([]string{e.Path}, e.Args...), " ")
	if e.Dir != "" {
		return fmt.Sprintf("%s failed in %s (exit %d): %s", cmd, e.Dir, e.ExitCode, e.Stderr)
	}

	return fmt.Sprintf("%s failed (exit %d): %s", cmd, e.ExitCode, e.Stderr)
}

// Run executes `git` with args in dir and returns captured output.
func (g Git) Run(ctx context.Context, dir string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, g.Path, args...) //nolint:gosec // This is an intentional wrapper around the system `git`.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), g.Env...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{
		Stdout: strings.TrimSuffix(stdout.String(), "\n"),
		Stderr: strings.TrimSuffix(stderr.String(), "\n"),
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return result, err
	}

	if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, &ExitError{
			Path:     g.Path,
			Args:     append([]string(nil), args...),
			Dir:      dir,
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		}
	}

	return result, err
}
