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

package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// ErrHookFailed is returned when a hook command exits non-zero.
var ErrHookFailed = errors.New("hook failed")

// Options configures hook execution output.
type Options struct {
	Stdout io.Writer
	Stderr io.Writer
}

// HookError reports a failing hook.
type HookError struct {
	Phase    string
	Index    int
	Command  string
	ExitCode int
	Stderr   string
}

func (e *HookError) Error() string {
	return fmt.Sprintf("%s hook %d failed (exit %d): %s", e.Phase, e.Index, e.ExitCode, e.Command)
}

func (e *HookError) Unwrap() error { return ErrHookFailed }

// Run executes hooks sequentially in dir with env applied.
//
// Commands are executed via the platform shell:
// - Unix: /bin/sh -c <hook>
// - Windows: cmd.exe /C <hook>
func Run(ctx context.Context, phase, dir string, hooks, env []string, opts Options) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	for i, hook := range hooks {
		if hook == "" {
			continue
		}

		cmd, err := shellCommand(ctx, hook)
		if err != nil {
			return err
		}
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)

		var hookStderr bytes.Buffer
		cmd.Stdout = stdout
		cmd.Stderr = io.MultiWriter(stderr, &hookStderr)

		// Hook execution is explicitly user-configured and uses the system shell.
		if err := cmd.Run(); err == nil { //nolint:gosec
			continue
		} else {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return &HookError{
					Phase:    phase,
					Index:    i + 1,
					Command:  hook,
					ExitCode: exitErr.ExitCode(),
					Stderr:   hookStderr.String(),
				}
			}
			return err
		}
	}

	return nil
}

func shellCommand(ctx context.Context, script string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "cmd.exe", "/C", script), nil
	default:
		return exec.CommandContext(ctx, "/bin/sh", "-c", script), nil
	}
}
