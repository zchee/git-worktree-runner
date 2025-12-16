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
	"path/filepath"
	"time"

	"github.com/zchee/git-worktree-runner/internal/copy"
	"github.com/zchee/git-worktree-runner/internal/gitcmd"
	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/hooks"
	"github.com/zchee/git-worktree-runner/internal/lock"
	"github.com/zchee/git-worktree-runner/internal/naming"
	"github.com/zchee/git-worktree-runner/internal/worktrees"
)

// ErrForceRequiresName is returned when CreateWorktreeOptions.Force is true and NameSuffix is empty.
var ErrForceRequiresName = errors.New("--force requires --name to distinguish worktrees")

// ErrInvalidTrackMode is returned when CreateWorktreeOptions.TrackMode is unknown.
var ErrInvalidTrackMode = errors.New("invalid track mode")

// TrackMode controls how a new worktree tracks branches.
type TrackMode string

const (
	TrackModeAuto   TrackMode = "auto"
	TrackModeRemote TrackMode = "remote"
	TrackModeLocal  TrackMode = "local"
	TrackModeNone   TrackMode = "none"
)

// CreateWorktreeOptions configures worktree creation.
type CreateWorktreeOptions struct {
	FromRef     string
	FromCurrent bool
	TrackMode   TrackMode
	NoCopy      bool
	NoFetch     bool
	Force       bool
	NameSuffix  string
}

// CreateWorktree creates a new linked worktree.
func (m *Manager) CreateWorktree(ctx context.Context, branch string, opts CreateWorktreeOptions) (Target, error) {
	if branch == "" {
		return Target{}, fmt.Errorf("branch name required")
	}

	if opts.Force && opts.NameSuffix == "" {
		return Target{}, ErrForceRequiresName
	}

	trackMode := opts.TrackMode
	if trackMode == "" {
		trackMode = TrackModeAuto
	}
	switch trackMode {
	case TrackModeAuto, TrackModeRemote, TrackModeLocal, TrackModeNone:
	default:
		return Target{}, fmt.Errorf("%w: %q", ErrInvalidTrackMode, trackMode)
	}

	paths, err := worktrees.ResolvePaths(ctx, m.cfg)
	if err != nil {
		return Target{}, err
	}

	worktreeName := naming.SanitizeBranchName(branch)
	if opts.NameSuffix != "" {
		worktreeName = worktreeName + "-" + opts.NameSuffix
	}
	worktreePath := filepath.Join(paths.BaseDir, paths.Prefix+worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return Target{}, fmt.Errorf("worktree already exists at %s", worktreePath)
	}

	if err := os.MkdirAll(paths.BaseDir, 0o755); err != nil {
		return Target{}, fmt.Errorf("create worktrees base dir %q: %w", paths.BaseDir, err)
	}

	lockPath := filepath.Join(m.repoCtx.CommonDir, "wr.lock")
	l, err := lock.Acquire(ctx, lockPath, 30*time.Second)
	if err != nil {
		return Target{}, err
	}
	defer func() { _ = l.Release() }()

	if !opts.NoFetch {
		// Match upstream behavior: fetch is best-effort.
		_, _ = m.git.Run(ctx, m.repoCtx.MainRoot, "fetch", "origin")
	}

	fromRef := opts.FromRef
	if fromRef == "" {
		if opts.FromCurrent {
			current, err := gitx.CurrentBranchGit(ctx, m.git, m.repoCtx.MainRoot)
			if err != nil {
				return Target{}, err
			}
			if current != gitx.DetachedBranch {
				fromRef = current
			}
		}
		if fromRef == "" {
			fromRef, err = m.resolveDefaultBranch(ctx)
			if err != nil {
				return Target{}, err
			}
		}
	}

	remoteExists, err := m.refExists(ctx, plumbingRemoteBranchRef("origin", branch))
	if err != nil {
		return Target{}, err
	}
	localExists, err := m.refExists(ctx, plumbingLocalBranchRef(branch))
	if err != nil {
		return Target{}, err
	}

	forceFlag := []string{}
	if opts.Force {
		forceFlag = append(forceFlag, "--force")
	}

	switch trackMode {
	case TrackModeRemote:
		if !remoteExists {
			return Target{}, fmt.Errorf("remote branch origin/%s does not exist", branch)
		}
		if localExists {
			if err := m.gitWorktreeAdd(ctx, forceFlag, worktreePath, branch); err != nil {
				return Target{}, err
			}
			break
		}
		if err := m.gitWorktreeAddNewBranch(ctx, forceFlag, worktreePath, branch, "origin/"+branch); err != nil {
			// Fallback to match upstream behavior when the branch already exists.
			if err2 := m.gitWorktreeAdd(ctx, forceFlag, worktreePath, branch); err2 != nil {
				return Target{}, err
			}
		}

	case TrackModeLocal:
		if !localExists {
			return Target{}, fmt.Errorf("local branch %s does not exist", branch)
		}
		if err := m.gitWorktreeAdd(ctx, forceFlag, worktreePath, branch); err != nil {
			return Target{}, err
		}

	case TrackModeNone:
		if err := m.gitWorktreeAddNewBranch(ctx, forceFlag, worktreePath, branch, fromRef); err != nil {
			return Target{}, err
		}

	case TrackModeAuto:
		if remoteExists && !localExists {
			// Create local tracking branch first (ignore error).
			_, _ = m.git.Run(ctx, m.repoCtx.MainRoot, "branch", "--track", branch, "origin/"+branch)
			if err := m.gitWorktreeAdd(ctx, forceFlag, worktreePath, branch); err != nil {
				return Target{}, err
			}
			break
		}
		if localExists {
			if err := m.gitWorktreeAdd(ctx, forceFlag, worktreePath, branch); err != nil {
				return Target{}, err
			}
			break
		}
		if err := m.gitWorktreeAddNewBranch(ctx, forceFlag, worktreePath, branch, fromRef); err != nil {
			return Target{}, err
		}
	}

	if !opts.NoCopy {
		if err := m.copyIntoWorktree(ctx, worktreePath); err != nil {
			return Target{}, err
		}
	}

	if err := m.runHooks(ctx, "postCreate", worktreePath, map[string]string{
		"REPO_ROOT":     m.repoCtx.MainRoot,
		"WORKTREE_PATH": worktreePath,
		"BRANCH":        branch,
	}); err != nil {
		return Target{}, err
	}

	return Target{
		IsMain: false,
		Path:   worktreePath,
		Branch: branch,
	}, nil
}

func (m *Manager) copyIntoWorktree(ctx context.Context, worktreePath string) error {
	includes, err := m.cfg.All(ctx, "wr.copy.include", "copy.include")
	if err != nil {
		return err
	}
	fileIncludes, err := m.cfg.WorktreeIncludePatterns()
	if err != nil {
		return err
	}
	includes = append(includes, fileIncludes...)

	excludes, err := m.cfg.All(ctx, "wr.copy.exclude", "copy.exclude")
	if err != nil {
		return err
	}

	if len(includes) > 0 {
		if _, err := copy.CopyFiles(ctx, m.repoCtx.MainRoot, worktreePath, includes, excludes, copy.Options{PreservePaths: true}); err != nil {
			return err
		}
	}

	includeDirs, err := m.cfg.All(ctx, "wr.copy.includeDirs", "copy.includeDirs")
	if err != nil {
		return err
	}
	excludeDirs, err := m.cfg.All(ctx, "wr.copy.excludeDirs", "copy.excludeDirs")
	if err != nil {
		return err
	}

	if len(includeDirs) > 0 {
		if _, err := copy.CopyDirectories(ctx, m.repoCtx.MainRoot, worktreePath, includeDirs, excludeDirs); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) runHooks(ctx context.Context, phase, dir string, env map[string]string) error {
	values, err := m.cfg.All(ctx, "wr.hook."+phase, "hooks."+phase)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}

	var envPairs []string
	for k, v := range env {
		envPairs = append(envPairs, k+"="+v)
	}

	return hooks.Run(ctx, phase, dir, values, envPairs, hooks.Options{})
}

func (m *Manager) resolveDefaultBranch(ctx context.Context) (string, error) {
	configured, err := m.cfg.Default(ctx, "wr.defaultBranch", "GTR_DEFAULT_BRANCH", "auto", "")
	if err != nil {
		return "", err
	}
	if configured != "auto" {
		return configured, nil
	}

	return gitx.DefaultBranchAuto(m.repo)
}

func (m *Manager) refExists(ctx context.Context, ref string) (bool, error) {
	_, err := m.git.Run(ctx, m.repoCtx.MainRoot, "show-ref", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}

	var ee *gitcmd.ExitError
	if errors.As(err, &ee) && ee.ExitCode == 1 {
		return false, nil
	}
	return false, err
}

func (m *Manager) gitWorktreeAdd(ctx context.Context, forceFlag []string, path, branch string) error {
	args := append([]string{"worktree", "add"}, forceFlag...)
	args = append(args, path, branch)
	_, err := m.git.Run(ctx, m.repoCtx.MainRoot, args...)
	return err
}

func (m *Manager) gitWorktreeAddNewBranch(ctx context.Context, forceFlag []string, path, branch, fromRef string) error {
	args := append([]string{"worktree", "add"}, forceFlag...)
	args = append(args, path, "-b", branch, fromRef)
	_, err := m.git.Run(ctx, m.repoCtx.MainRoot, args...)
	return err
}

func plumbingRemoteBranchRef(remote, branch string) string {
	return "refs/remotes/" + remote + "/" + branch
}

func plumbingLocalBranchRef(branch string) string {
	return "refs/heads/" + branch
}
