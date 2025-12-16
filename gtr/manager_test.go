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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestManagerListAndResolveTarget(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	g, err := gitcmd.New()
	if err != nil {
		t.Fatalf("gitcmd.New() error: %v", err)
	}

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	worktreeDir := filepath.Join(tmp, "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	repoDir, err = pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	entries, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	var foundMain bool
	var foundWT bool
	for _, e := range entries {
		if e.Target.IsMain && e.Target.Path == repoDir {
			foundMain = true
		}
		if !e.Target.IsMain && e.Target.Path == worktreeDir && e.Target.Branch == "foo" {
			foundWT = true
		}
	}
	if !foundMain {
		t.Fatalf("expected main repo entry for %q, got %+v", repoDir, entries)
	}
	if !foundWT {
		t.Fatalf("expected worktree entry for %q, got %+v", worktreeDir, entries)
	}

	mainTarget, err := m.ResolveTarget(t.Context(), "1")
	if err != nil {
		t.Fatalf("ResolveTarget(1) error: %v", err)
	}
	if diff := cmp.Diff(true, mainTarget.IsMain); diff != "" {
		t.Fatalf("isMain mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(repoDir, mainTarget.Path); diff != "" {
		t.Fatalf("path mismatch (-want +got):\n%s", diff)
	}

	wtTarget, err := m.ResolveTarget(t.Context(), "foo")
	if err != nil {
		t.Fatalf("ResolveTarget(foo) error: %v", err)
	}
	if wtTarget.IsMain {
		t.Fatalf("expected non-main worktree target, got %+v", wtTarget)
	}
	if diff := cmp.Diff(worktreeDir, wtTarget.Path); diff != "" {
		t.Fatalf("path mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("foo", wtTarget.Branch); diff != "" {
		t.Fatalf("branch mismatch (-want +got):\n%s", diff)
	}
}

func TestManagerListIncludesMissingBaseDirEntries(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	baseDir := filepath.Join(tmp, "repo-worktrees")
	missingDir := filepath.Join(baseDir, "orphan")
	if err := os.MkdirAll(missingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", missingDir, err)
	}

	missingDir, err := pathutil.Canonicalize(missingDir)
	if err != nil {
		t.Fatalf("Canonicalize(missingDir): %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	entries, err := m.List(t.Context())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	var found bool
	for _, e := range entries {
		if e.Target.Path != missingDir {
			continue
		}
		found = true
		if diff := cmp.Diff(WorktreeStatusMissing, e.Status); diff != "" {
			t.Fatalf("status mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(gitx.DetachedBranch, e.Target.Branch); diff != "" {
			t.Fatalf("branch mismatch (-want +got):\n%s", diff)
		}
	}
	if !found {
		t.Fatalf("expected missing entry for %q, got %+v", missingDir, entries)
	}
}
