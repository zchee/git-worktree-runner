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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/pathutil"
	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestCleanRemovesEmptyDirectories(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.worktrees.dir", ".worktrees"); err != nil {
		t.Fatalf("git config gtr.worktrees.dir: %v", err)
	}
	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.worktrees.prefix", "wt-"); err != nil {
		t.Fatalf("git config gtr.worktrees.prefix: %v", err)
	}

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}

	baseDir := filepath.Join(repoDir, ".worktrees")
	emptyDir := filepath.Join(baseDir, "wt-empty")
	otherEmptyDir := filepath.Join(baseDir, "other-empty")
	nonEmptyDir := filepath.Join(baseDir, "wt-nonempty")

	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(emptyDir): %v", err)
	}
	if err := os.MkdirAll(otherEmptyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(otherEmptyDir): %v", err)
	}
	if err := os.MkdirAll(nonEmptyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(nonEmptyDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "keep.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(keep.txt): %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	got, err := m.Clean(t.Context())
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Fatalf("expected empty dir removed, stat err=%v", err)
	}
	if _, err := os.Stat(otherEmptyDir); !os.IsNotExist(err) {
		t.Fatalf("expected other empty dir removed, stat err=%v", err)
	}
	if _, err := os.Stat(nonEmptyDir); err != nil {
		t.Fatalf("expected non-empty dir to remain, stat err=%v", err)
	}

	wantRemoved := []string{emptyDir, otherEmptyDir}
	slices.Sort(wantRemoved)
	slices.Sort(got.RemovedEmptyDirs)
	if diff := cmp.Diff(wantRemoved, got.RemovedEmptyDirs); diff != "" {
		t.Fatalf("removed dirs mismatch (-want +got):\n%s", diff)
	}
}
