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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestManagerCopy(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	target, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
		FromCurrent: true,
		NoCopy:      true,
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	srcFile := filepath.Join(repoDir, ".env.local")
	if err := os.WriteFile(srcFile, []byte("A=B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.env.local): %v", err)
	}

	got, err := m.Copy(t.Context(), []string{"feature-a"}, CopyOptions{
		Patterns: []string{".env.local"},
	})
	if err != nil {
		t.Fatalf("Copy() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(got), got)
	}
	if diff := cmp.Diff(target.Path, got[0].Target.Path); diff != "" {
		t.Fatalf("target path mismatch (-want +got):\n%s", diff)
	}

	dstFile := filepath.Join(target.Path, ".env.local")
	b, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("ReadFile(dst): %v", err)
	}
	if diff := cmp.Diff("A=B\n", string(b)); diff != "" {
		t.Fatalf("copied contents mismatch (-want +got):\n%s", diff)
	}
}

func TestManagerCopyDryRun(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	target, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
		FromCurrent: true,
		NoCopy:      true,
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	srcFile := filepath.Join(repoDir, ".env.local")
	if err := os.WriteFile(srcFile, []byte("A=B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.env.local): %v", err)
	}

	got, err := m.Copy(t.Context(), []string{"feature-a"}, CopyOptions{
		Patterns: []string{".env.local"},
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("Copy() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(got), got)
	}
	if diff := cmp.Diff([]string{".env.local"}, got[0].CopiedFiles); diff != "" {
		t.Fatalf("copied files mismatch (-want +got):\n%s", diff)
	}

	dstFile := filepath.Join(target.Path, ".env.local")
	if _, err := os.Stat(dstFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run to not create %q, stat err=%v", dstFile, err)
	}
}

func TestManagerCopyAllSkipsMissingTargets(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	target, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
		FromCurrent: true,
		NoCopy:      true,
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Create a directory under the default worktrees base dir that is not a Git worktree.
	orphanDir := filepath.Join(tmp, "repo-worktrees", "orphan")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", orphanDir, err)
	}

	srcFile := filepath.Join(repoDir, ".env.local")
	if err := os.WriteFile(srcFile, []byte("A=B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.env.local): %v", err)
	}

	results, err := m.Copy(t.Context(), nil, CopyOptions{
		All:      true,
		Patterns: []string{".env.local"},
	})
	if err != nil {
		t.Fatalf("Copy(All) error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Target.Path != target.Path {
		t.Fatalf("expected copy target %q, got %+v", target.Path, results)
	}

	if _, err := os.Stat(filepath.Join(orphanDir, ".env.local")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan dir to be skipped, stat err=%v", err)
	}
}
