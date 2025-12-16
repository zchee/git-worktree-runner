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

package repoctx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
)

// ErrNotInRepo is returned when the current directory is not inside a Git repository.
var ErrNotInRepo = errors.New("not in a git repository")

// Context describes the resolved repository locations needed by git-gtr.
type Context struct {
	// StartDir is the absolute directory from which discovery started.
	StartDir string
	// WorktreeRoot is the worktree root for StartDir (git rev-parse --show-toplevel).
	WorktreeRoot string
	// CommonDir is the absolute path to the repository common git dir (typically <main>/.git).
	CommonDir string
	// MainRoot is the absolute path to the main worktree root (parent of CommonDir).
	MainRoot string
}

// Discover resolves the repository context for startDir.
//
// It uses `git rev-parse --git-common-dir` and `git rev-parse --show-toplevel`.
func Discover(ctx context.Context, g gitcmd.Git, startDir string) (Context, error) {
	if startDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Context{}, fmt.Errorf("get working directory: %w", err)
		}
		startDir = wd
	}

	startDirAbs, err := filepath.Abs(startDir)
	if err != nil {
		return Context{}, fmt.Errorf("absolute start dir: %w", err)
	}
	startDirAbs, err = pathutil.Canonicalize(startDirAbs)
	if err != nil {
		return Context{}, fmt.Errorf("canonicalize start dir: %w", err)
	}

	commonRes, err := g.Run(ctx, startDirAbs, "rev-parse", "--git-common-dir")
	if err != nil {
		return Context{}, ErrNotInRepo
	}
	commonOut := strings.TrimSpace(commonRes.Stdout)
	if commonOut == "" {
		return Context{}, fmt.Errorf("git rev-parse --git-common-dir returned empty output")
	}

	topRes, err := g.Run(ctx, startDirAbs, "rev-parse", "--show-toplevel")
	if err != nil {
		return Context{}, fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	worktreeRoot := strings.TrimSpace(topRes.Stdout)
	if worktreeRoot == "" {
		return Context{}, fmt.Errorf("git rev-parse --show-toplevel returned empty output")
	}
	worktreeRoot, err = pathutil.Canonicalize(worktreeRoot)
	if err != nil {
		return Context{}, fmt.Errorf("canonicalize worktree root: %w", err)
	}

	var commonAbs string
	if commonOut == ".git" {
		commonAbs = filepath.Join(worktreeRoot, ".git")
	} else if filepath.IsAbs(commonOut) {
		commonAbs = commonOut
	} else {
		commonAbs = filepath.Join(worktreeRoot, commonOut)
	}
	commonAbs = filepath.Clean(commonAbs)

	commonAbs, err = pathutil.Canonicalize(commonAbs)
	if err != nil {
		return Context{}, fmt.Errorf("canonicalize common dir: %w", err)
	}

	mainRoot, err := pathutil.Canonicalize(filepath.Dir(commonAbs))
	if err != nil {
		return Context{}, fmt.Errorf("canonicalize main root: %w", err)
	}

	return Context{
		StartDir:     startDirAbs,
		WorktreeRoot: worktreeRoot,
		CommonDir:    commonAbs,
		MainRoot:     mainRoot,
	}, nil
}
