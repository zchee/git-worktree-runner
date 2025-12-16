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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zchee/git-worktree-runner/internal/lock"
	"github.com/zchee/git-worktree-runner/internal/worktrees"
)

// CleanResult describes the effect of Clean.
type CleanResult struct {
	RemovedEmptyDirs []string
}

// Clean prunes stale worktree metadata and removes empty worktree directories.
func (m *Manager) Clean(ctx context.Context) (CleanResult, error) {
	lockPath := filepath.Join(m.repoCtx.CommonDir, "wr.lock")
	l, err := lock.Acquire(ctx, lockPath, 30*time.Second)
	if err != nil {
		return CleanResult{}, err
	}
	defer func() { _ = l.Release() }()

	// Best-effort prune (matches upstream).
	_, _ = m.git.Run(ctx, m.repoCtx.MainRoot, "worktree", "prune")

	paths, err := worktrees.ResolvePaths(ctx, m.cfg)
	if err != nil {
		return CleanResult{}, err
	}

	if _, err := os.Stat(paths.BaseDir); err != nil {
		if os.IsNotExist(err) {
			return CleanResult{}, nil
		}
		return CleanResult{}, err
	}

	entries, err := os.ReadDir(paths.BaseDir)
	if err != nil {
		return CleanResult{}, err
	}

	var removed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(paths.BaseDir, e.Name())
		children, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		if len(children) != 0 {
			continue
		}
		if err := os.Remove(dirPath); err != nil {
			return CleanResult{}, fmt.Errorf("remove empty directory %q: %w", dirPath, err)
		}
		removed = append(removed, dirPath)
	}

	return CleanResult{RemovedEmptyDirs: removed}, nil
}
