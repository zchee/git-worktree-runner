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
	"fmt"
	"path/filepath"
	"time"

	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/lock"
)

// RemoveWorktreeOptions configures worktree removal.
type RemoveWorktreeOptions struct {
	DeleteBranch bool
	Force        bool
	Yes          bool

	// ConfirmDeleteBranch controls whether a branch should be deleted when DeleteBranch is true.
	//
	// When Yes is true (or ManagerOptions.Yes was set when creating the Manager), confirmation is skipped and
	// the branch is deleted (matching upstream `--yes`).
	// When Yes is false and ConfirmDeleteBranch is non-nil, it is called to decide whether to delete.
	// When Yes is false and ConfirmDeleteBranch is nil, the branch is deleted (library default).
	ConfirmDeleteBranch func(ctx context.Context, branch string) (bool, error)
}

// Remove removes one or more worktrees identified by identifiers.
func (m *Manager) Remove(ctx context.Context, identifiers []string, opts RemoveWorktreeOptions) error {
	if len(identifiers) == 0 {
		return fmt.Errorf("at least one identifier is required")
	}

	lockPath := filepath.Join(m.repoCtx.CommonDir, "gtr.lock")
	l, err := lock.Acquire(ctx, lockPath, 30*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = l.Release() }()

	var errs []error

	for _, id := range identifiers {
		target, err := m.ResolveTarget(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if target.IsMain {
			errs = append(errs, fmt.Errorf("cannot remove main repository"))
			continue
		}

		args := []string{"worktree", "remove"}
		if opts.Force {
			args = append(args, "--force")
		}
		args = append(args, target.Path)

		if _, err := m.git.Run(ctx, m.repoCtx.MainRoot, args...); err != nil {
			errs = append(errs, err)
			continue
		}

		if opts.DeleteBranch && target.Branch != "" && target.Branch != gitx.DetachedBranch {
			yes := opts.Yes || m.yes

			deleteBranch := true
			if !yes && opts.ConfirmDeleteBranch != nil {
				ok, err := opts.ConfirmDeleteBranch(ctx, target.Branch)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				deleteBranch = ok
			}
			if deleteBranch {
				if _, err := m.git.Run(ctx, m.repoCtx.MainRoot, "branch", "-D", target.Branch); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		}

		if err := m.runHooks(ctx, "postRemove", m.repoCtx.MainRoot, map[string]string{
			"REPO_ROOT":     m.repoCtx.MainRoot,
			"WORKTREE_PATH": target.Path,
			"BRANCH":        target.Branch,
		}); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}
