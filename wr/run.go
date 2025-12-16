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

package wr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// RunOptions configures Manager.Run.
type RunOptions struct {
	// Env is a list of KEY=VALUE pairs appended to the current process environment.
	Env []string

	IO ExecIO
}

// Run executes argv in the target directory and returns the command's exit code.
//
// If the command exits with a non-zero status, Run returns that exit code and a nil error.
func (m *Manager) Run(ctx context.Context, identifier string, argv []string, opts RunOptions) (exitCode int, err error) {
	if len(argv) == 0 {
		return 1, fmt.Errorf("no command specified")
	}

	target, err := m.ResolveTarget(ctx, identifier)
	if err != nil {
		return 1, err
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // This command intentionally executes user-provided programs.
	cmd.Dir = target.Path

	if opts.IO.Stdin != nil {
		cmd.Stdin = opts.IO.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	if opts.IO.Stdout != nil {
		cmd.Stdout = opts.IO.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.IO.Stderr != nil {
		cmd.Stderr = opts.IO.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if len(opts.Env) != 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	if err := cmd.Run(); err == nil {
		return 0, nil
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
}
