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

package gtr

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/zchee/git-worktree-runner/internal/adapters"
	"github.com/zchee/git-worktree-runner/internal/platform"
)

// ErrNoAIToolConfigured is returned when no AI tool is configured.
var ErrNoAIToolConfigured = errors.New("no AI tool configured")

// ExecIO configures stdio for interactive commands (editor/ai).
type ExecIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// OpenEditor opens the target in an editor adapter.
func (m *Manager) OpenEditor(ctx context.Context, identifier, editorOverride string, io ExecIO) (int, error) {
	target, err := m.ResolveTarget(ctx, identifier)
	if err != nil {
		return 1, err
	}

	editor := editorOverride
	if editor == "" {
		editor, err = m.cfg.Default(ctx, "gtr.editor.default", "GTR_EDITOR_DEFAULT", "none", "defaults.editor")
		if err != nil {
			return 1, err
		}
	}

	if editor == "none" || editor == "" {
		if err := platform.OpenInGUI(ctx, target.Path); err != nil {
			return 1, err
		}
		return 0, nil
	}

	spec, err := adapters.ResolveEditor(editor, target.Path)
	if err != nil {
		return 1, err
	}
	spec, err = ensureCommandExists(spec)
	if err != nil {
		return 1, err
	}

	return adapters.Exec(ctx, spec, io.Stdin, io.Stdout, io.Stderr)
}

// RunAI starts an AI tool in the target directory and returns its exit code.
func (m *Manager) RunAI(ctx context.Context, identifier, toolOverride string, args []string, io ExecIO) (int, error) {
	target, err := m.ResolveTarget(ctx, identifier)
	if err != nil {
		return 1, err
	}

	tool := toolOverride
	if tool == "" {
		tool, err = m.cfg.Default(ctx, "gtr.ai.default", "GTR_AI_DEFAULT", "none", "defaults.ai")
		if err != nil {
			return 1, err
		}
	}

	if tool == "none" || tool == "" {
		return 1, ErrNoAIToolConfigured
	}

	spec, err := adapters.ResolveAI(tool, target.Path, args)
	if err != nil {
		return 1, err
	}
	spec, err = ensureCommandExists(spec)
	if err != nil {
		return 1, err
	}

	return adapters.Exec(ctx, spec, io.Stdin, io.Stdout, io.Stderr)
}

func ensureCommandExists(spec adapters.Spec) (adapters.Spec, error) {
	if filepath.IsAbs(spec.Command) {
		return spec, nil
	}
	p, err := exec.LookPath(spec.Command)
	if err != nil {
		return adapters.Spec{}, err
	}
	spec.Command = p
	return spec, nil
}
