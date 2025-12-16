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

package worktrees

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
)

// BranchResolver returns the current branch name for dir.
//
// Callers should return gitx.DetachedBranch for detached HEAD.
type BranchResolver func(ctx context.Context, dir string) (string, error)

// PorcelainEntry represents one entry from `git worktree list --porcelain`.
type PorcelainEntry struct {
	Path     string
	Branch   string
	Detached bool
	Locked   bool
	Prunable bool
}

// ListPorcelain lists the main repository worktree and all linked worktrees by scanning
// the repository's common git directory (typically "<mainRoot>/.git/worktrees").
func ListPorcelain(ctx context.Context, commonDir, mainRoot string, resolveBranch BranchResolver) ([]PorcelainEntry, error) {
	commonDir, err := pathutil.Canonicalize(commonDir)
	if err != nil {
		return nil, fmt.Errorf("canonicalize common dir: %w", err)
	}

	mainRoot, err = pathutil.Canonicalize(mainRoot)
	if err != nil {
		return nil, fmt.Errorf("canonicalize main root: %w", err)
	}

	mainBranch, mainDetached, err := worktreeBranchFromMeta(commonDir)
	if err != nil {
		return nil, err
	}
	// Reftable repositories store a placeholder symref in HEAD.
	if mainBranch == ".invalid" || mainBranch == "" {
		mainBranch, err = resolveBranch(ctx, mainRoot)
		if err != nil {
			return nil, err
		}
		if mainBranch == "" {
			mainBranch = gitx.DetachedBranch
		}
		mainDetached = mainBranch == gitx.DetachedBranch
	}
	if mainBranch == gitx.DetachedBranch {
		mainDetached = true
	}

	out := []PorcelainEntry{
		{
			Path:     mainRoot,
			Branch:   mainBranch,
			Detached: mainDetached,
		},
	}

	worktreesDir := filepath.Join(commonDir, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, nil
		}
		return nil, fmt.Errorf("read worktrees dir %q: %w", worktreesDir, err)
	}

	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !e.IsDir() {
			continue
		}

		metaDir := filepath.Join(worktreesDir, e.Name())

		wtPath, err := worktreePathFromMeta(metaDir)
		if err != nil {
			return nil, err
		}

		locked, err := fileExists(filepath.Join(metaDir, "locked"))
		if err != nil {
			return nil, err
		}

		prunable, err := isPrunableWorktree(wtPath)
		if err != nil {
			return nil, err
		}

		metaBranch, metaDetached, err := worktreeBranchFromMeta(metaDir)
		if err != nil {
			return nil, err
		}

		branch := metaBranch
		detached := metaDetached || metaBranch == gitx.DetachedBranch
		if branch == "" {
			branch = gitx.DetachedBranch
			detached = true
		}

		// Reftable repositories store a placeholder symref in HEAD.
		// When the worktree itself is available, ask the caller to resolve the branch.
		if branch == ".invalid" && !prunable {
			resolved, err := resolveBranch(ctx, wtPath)
			if err != nil {
				return nil, err
			}
			if resolved == "" {
				resolved = gitx.DetachedBranch
			}
			branch = resolved
			detached = resolved == gitx.DetachedBranch
		} else if branch == ".invalid" {
			branch = gitx.DetachedBranch
			detached = true
		}

		out = append(out, PorcelainEntry{
			Path:     wtPath,
			Branch:   branch,
			Detached: detached,
			Locked:   locked,
			Prunable: prunable,
		})
	}

	return out, nil
}

func worktreePathFromMeta(metaDir string) (string, error) {
	gitdirFile := filepath.Join(metaDir, "gitdir")
	b, err := os.ReadFile(gitdirFile)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", gitdirFile, err)
	}

	gitdirPath := strings.TrimSpace(string(b))
	if gitdirPath == "" {
		return "", fmt.Errorf("empty gitdir file: %q", gitdirFile)
	}
	if !filepath.IsAbs(gitdirPath) {
		gitdirPath = filepath.Join(metaDir, gitdirPath)
	}

	gitdirPath, err = pathutil.Canonicalize(gitdirPath)
	if err != nil {
		return "", fmt.Errorf("canonicalize gitdir path %q: %w", gitdirPath, err)
	}

	worktreePath := filepath.Dir(gitdirPath)
	worktreePath, err = pathutil.Canonicalize(worktreePath)
	if err != nil {
		return "", fmt.Errorf("canonicalize worktree path %q: %w", worktreePath, err)
	}

	return worktreePath, nil
}

func worktreeBranchFromMeta(metaDir string) (branch string, detached bool, err error) {
	headFile := filepath.Join(metaDir, "HEAD")
	b, err := os.ReadFile(headFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return gitx.DetachedBranch, true, nil
		}
		return "", false, fmt.Errorf("read %q: %w", headFile, err)
	}

	line := strings.TrimSpace(string(b))
	if line == "" {
		return gitx.DetachedBranch, true, nil
	}

	const refPrefix = "ref: "
	if after, ok := strings.CutPrefix(line, refPrefix); ok {
		ref := after
		const headsPrefix = "refs/heads/"
		if after, ok := strings.CutPrefix(ref, headsPrefix); ok {
			return after, false, nil
		}
		return ref, false, nil
	}

	return gitx.DetachedBranch, true, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func isPrunableWorktree(worktreePath string) (bool, error) {
	fi, err := os.Stat(worktreePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if !fi.IsDir() {
		return true, nil
	}

	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}
