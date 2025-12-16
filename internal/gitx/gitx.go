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

package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
)

// DetachedBranch is the branch name used for detached HEAD states.
const DetachedBranch = "(detached)"

// Open opens a repository at path and enables linked-worktree common-dir handling.
func Open(path string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open git repository: %w", err)
	}
	return repo, nil
}

// CurrentBranch returns the current branch name of repo, or "(detached)" when HEAD is detached.
func CurrentBranch(repo *git.Repository) (string, error) {
	head, err := repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return "", fmt.Errorf("read HEAD: %w", err)
	}

	switch head.Type() {
	case plumbing.SymbolicReference:
		target := head.Target()
		return target.Short(), nil
	case plumbing.HashReference:
		return DetachedBranch, nil
	default:
		return "", fmt.Errorf("unknown HEAD reference type: %s", head.Type())
	}
}

// CurrentBranchGit returns the current branch name by asking the `git` binary.
//
// This is needed for repositories using the reftable ref format, where
// `.git/HEAD` can be a placeholder (e.g. `refs/heads/.invalid`) and libraries
// that read `.git/HEAD` directly may report incorrect branch names.
func CurrentBranchGit(ctx context.Context, g gitcmd.Git, dir string) (string, error) {
	res, err := g.Run(ctx, dir, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}

	branch := strings.TrimSpace(res.Stdout)
	if branch != "" {
		return branch, nil
	}

	res, err = g.Run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --abbrev-ref HEAD: %w", err)
	}

	branch = strings.TrimSpace(res.Stdout)
	if branch == "" || branch == "HEAD" {
		return DetachedBranch, nil
	}

	return branch, nil
}

// DefaultBranchAuto resolves the default branch using origin/HEAD when possible, falling back to origin/main, origin/master, then "main".
func DefaultBranchAuto(repo *git.Repository) (string, error) {
	ref, err := repo.Reference(plumbing.NewRemoteHEADReferenceName("origin"), false)
	if err == nil {
		if ref.Type() == plumbing.SymbolicReference {
			target := ref.Target().String()
			const prefix = "refs/remotes/origin/"
			if after, ok := strings.CutPrefix(target, prefix); ok {
				return after, nil
			}
			return ref.Target().Short(), nil
		}
	}

	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("read origin/HEAD: %w", err)
	}

	if _, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/main"), true); err == nil {
		return "main", nil
	} else if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("check origin/main: %w", err)
	}

	if _, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/master"), true); err == nil {
		return "master", nil
	} else if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("check origin/master: %w", err)
	}

	return "main", nil
}
