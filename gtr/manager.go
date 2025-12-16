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
	"os"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v6"

	"github.com/zchee/git-worktree-runner/internal/config"
	"github.com/zchee/git-worktree-runner/internal/gitcmd"
	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/naming"
	"github.com/zchee/git-worktree-runner/internal/repoctx"
	"github.com/zchee/git-worktree-runner/internal/worktrees"
)

// WorktreeStatus describes the status of a worktree as reported by Git.
type WorktreeStatus string

const (
	WorktreeStatusOK       WorktreeStatus = "ok"
	WorktreeStatusDetached WorktreeStatus = "detached"
	WorktreeStatusLocked   WorktreeStatus = "locked"
	WorktreeStatusPrunable WorktreeStatus = "prunable"
	WorktreeStatusMissing  WorktreeStatus = "missing"
)

// ErrTargetNotFound is returned when a worktree cannot be resolved from an identifier.
var ErrTargetNotFound = errors.New("worktree target not found")

// ManagerOptions configures Manager construction.
type ManagerOptions struct {
	// StartDir is where repository discovery begins. If empty, os.Getwd is used.
	StartDir string
	// Yes forces non-interactive behavior.
	Yes bool
	// Env overrides environment variables for config resolution (tests).
	Env map[string]string
}

// Manager manages git worktree operations for a single repository.
type Manager struct {
	git gitcmd.Git

	repoCtx repoctx.Context
	repo    *git.Repository

	cfg config.Resolver

	yes bool
}

// Target identifies a worktree or the main repository.
type Target struct {
	IsMain bool
	Path   string
	Branch string
}

// ListEntry is one row in `git gtr list --porcelain`.
type ListEntry struct {
	Target Target
	Status WorktreeStatus
}

// NewManager discovers the repository from opts.StartDir and returns a Manager bound to that repository.
func NewManager(ctx context.Context, opts ManagerOptions) (*Manager, error) {
	g, err := gitcmd.New()
	if err != nil {
		return nil, err
	}

	rc, err := repoctx.Discover(ctx, g, opts.StartDir)
	if err != nil {
		return nil, err
	}

	repo, err := gitx.Open(rc.MainRoot)
	if err != nil {
		return nil, err
	}

	return &Manager{
		git:     g,
		repoCtx: rc,
		repo:    repo,
		cfg:     config.New(g, rc.MainRoot, opts.Env),
		yes:     opts.Yes,
	}, nil
}

// MainRoot returns the main repository worktree root.
func (m *Manager) MainRoot() string {
	return m.repoCtx.MainRoot
}

func (m *Manager) currentBranch(ctx context.Context, dir string) (string, error) {
	return gitx.CurrentBranchGit(ctx, m.git, dir)
}

// ResolveTarget resolves identifier into a concrete Target.
//
// identifier can be:
// - "1" for the main repository
// - a branch name
// - a worktree directory name (after sanitization and optional prefix)
func (m *Manager) ResolveTarget(ctx context.Context, identifier string) (Target, error) {
	if identifier == "" {
		return Target{}, fmt.Errorf("%w: empty identifier", ErrTargetNotFound)
	}

	entries, err := worktrees.ListPorcelain(ctx, m.repoCtx.CommonDir, m.repoCtx.MainRoot, m.currentBranch)
	if err != nil {
		return Target{}, err
	}

	byPath := map[string]worktrees.PorcelainEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}

	mainEntry, ok := byPath[m.repoCtx.MainRoot]
	if !ok {
		branch, err := m.currentBranch(ctx, m.repoCtx.MainRoot)
		if err != nil {
			return Target{}, err
		}
		mainEntry = worktrees.PorcelainEntry{
			Path:     m.repoCtx.MainRoot,
			Branch:   branch,
			Detached: branch == gitx.DetachedBranch,
		}
	}

	if identifier == "1" {
		return Target{IsMain: true, Path: m.repoCtx.MainRoot, Branch: mainEntry.Branch}, nil
	}

	if mainEntry.Branch != gitx.DetachedBranch && identifier == mainEntry.Branch {
		return Target{IsMain: true, Path: m.repoCtx.MainRoot, Branch: mainEntry.Branch}, nil
	}

	paths, err := worktrees.ResolvePaths(ctx, m.cfg)
	if err != nil {
		return Target{}, err
	}

	candidate := filepath.Join(paths.BaseDir, paths.Prefix+naming.SanitizeBranchName(identifier))
	if e, ok := byPath[candidate]; ok {
		return Target{IsMain: false, Path: e.Path, Branch: e.Branch}, nil
	}
	if _, err := os.Stat(candidate); err == nil {
		branch, err := m.currentBranch(ctx, candidate)
		if err != nil {
			return Target{}, err
		}
		return Target{IsMain: false, Path: candidate, Branch: branch}, nil
	}

	for _, e := range entries {
		if e.Path == m.repoCtx.MainRoot {
			continue
		}
		if e.Branch == identifier {
			return Target{IsMain: false, Path: e.Path, Branch: e.Branch}, nil
		}
	}

	return Target{}, fmt.Errorf("%w: %s", ErrTargetNotFound, identifier)
}

// List returns all known worktrees, including the main repository worktree.
func (m *Manager) List(ctx context.Context) ([]ListEntry, error) {
	porcelainEntries, err := worktrees.ListPorcelain(ctx, m.repoCtx.CommonDir, m.repoCtx.MainRoot, m.currentBranch)
	if err != nil {
		return nil, err
	}

	paths, err := worktrees.ResolvePaths(ctx, m.cfg)
	if err != nil {
		return nil, err
	}

	byPath := map[string]worktrees.PorcelainEntry{}
	seenPaths := map[string]struct{}{}
	for _, e := range porcelainEntries {
		byPath[e.Path] = e
		seenPaths[e.Path] = struct{}{}
	}

	// Match upstream behavior: also list directories under the configured worktrees dir.
	if dirs, err := os.ReadDir(paths.BaseDir); err == nil {
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			if !strings.HasPrefix(d.Name(), paths.Prefix) {
				continue
			}
			seenPaths[filepath.Join(paths.BaseDir, d.Name())] = struct{}{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	var out []ListEntry
	for path := range seenPaths {
		e, ok := byPath[path]

		branch := e.Branch
		if !ok {
			branch = gitx.DetachedBranch
			if b, err := m.currentBranch(ctx, path); err == nil && b != "" {
				branch = b
			}
		}

		status := WorktreeStatusMissing
		if ok {
			status = WorktreeStatusOK
			if e.Locked {
				status = WorktreeStatusLocked
			} else if e.Prunable {
				status = WorktreeStatusPrunable
			} else if e.Detached {
				status = WorktreeStatusDetached
			}
		}

		out = append(out, ListEntry{
			Target: Target{
				IsMain: path == m.repoCtx.MainRoot,
				Path:   path,
				Branch: branch,
			},
			Status: status,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Target.IsMain != out[j].Target.IsMain {
			return out[i].Target.IsMain
		}
		if out[i].Target.Branch != out[j].Target.Branch {
			return out[i].Target.Branch < out[j].Target.Branch
		}
		return out[i].Target.Path < out[j].Target.Path
	})

	return out, nil
}
