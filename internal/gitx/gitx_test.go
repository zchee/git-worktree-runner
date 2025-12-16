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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestCurrentBranch(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	got, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error: %v", err)
	}
	if got == "" {
		t.Fatalf("CurrentBranch() returned empty")
	}
	if got == DetachedBranch {
		t.Fatalf("expected non-detached branch, got %q", got)
	}
}

func TestCurrentBranchDetached(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "checkout", "--detach"); err != nil {
		t.Fatalf("git checkout --detach: %v", err)
	}

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	got, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error: %v", err)
	}
	if diff := cmp.Diff(DetachedBranch, got); diff != "" {
		t.Fatalf("branch mismatch (-want +got):\n%s", diff)
	}
}

func TestCurrentBranchGit(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	got, err := CurrentBranchGit(t.Context(), g, repoDir)
	if err != nil {
		t.Fatalf("CurrentBranchGit() error: %v", err)
	}
	if got == "" {
		t.Fatalf("CurrentBranchGit() returned empty")
	}
	if got == DetachedBranch {
		t.Fatalf("expected non-detached branch, got %q", got)
	}
}

func TestCurrentBranchGitDetached(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "checkout", "--detach"); err != nil {
		t.Fatalf("git checkout --detach: %v", err)
	}

	got, err := CurrentBranchGit(t.Context(), g, repoDir)
	if err != nil {
		t.Fatalf("CurrentBranchGit() error: %v", err)
	}
	if diff := cmp.Diff(DetachedBranch, got); diff != "" {
		t.Fatalf("branch mismatch (-want +got):\n%s", diff)
	}
}

func TestCurrentBranchGitReftable(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", repoDir, err)
	}

	if _, err := g.Run(t.Context(), repoDir, "init", "--ref-format=reftable", "--initial-branch=main"); err != nil {
		t.Fatalf("git init --ref-format=reftable: %v", err)
	}

	head, err := os.ReadFile(filepath.Join(repoDir, ".git", "HEAD"))
	if err != nil {
		t.Fatalf("ReadFile(.git/HEAD): %v", err)
	}
	if !strings.Contains(string(head), ".invalid") {
		t.Fatalf("expected reftable placeholder HEAD, got %q", string(head))
	}

	got, err := CurrentBranchGit(t.Context(), g, repoDir)
	if err != nil {
		t.Fatalf("CurrentBranchGit() error: %v", err)
	}
	if diff := cmp.Diff("main", got); diff != "" {
		t.Fatalf("branch mismatch (-want +got):\n%s", diff)
	}
}

func TestDefaultBranchAutoFallback(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	got, err := DefaultBranchAuto(repo)
	if err != nil {
		t.Fatalf("DefaultBranchAuto() error: %v", err)
	}

	if diff := cmp.Diff("main", got); diff != "" {
		t.Fatalf("default branch mismatch (-want +got):\n%s", diff)
	}
}

func TestDefaultBranchAutoOriginHEAD(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "update-ref", "refs/remotes/origin/main", "HEAD"); err != nil {
		t.Fatalf("git update-ref: %v", err)
	}
	if _, err := g.Run(t.Context(), repoDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("git symbolic-ref: %v", err)
	}

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	got, err := DefaultBranchAuto(repo)
	if err != nil {
		t.Fatalf("DefaultBranchAuto() error: %v", err)
	}
	if diff := cmp.Diff("main", got); diff != "" {
		t.Fatalf("default branch mismatch (-want +got):\n%s", diff)
	}
}
