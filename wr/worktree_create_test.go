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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestCreateWorktreeValidations(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	tests := map[string]struct {
		opts    CreateWorktreeOptions
		wantErr error
	}{
		"error: force requires name": {
			opts: CreateWorktreeOptions{
				Force: true,
			},
			wantErr: ErrForceRequiresName,
		},
		"error: invalid track mode": {
			opts: CreateWorktreeOptions{
				TrackMode: TrackMode("nope"),
			},
			wantErr: ErrInvalidTrackMode,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoDir := filepath.Join(t.TempDir(), "repo")
			g := testutil.Git(t)
			testutil.InitRepo(t, g, repoDir)

			m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
			if err != nil {
				t.Fatalf("NewManager() error: %v", err)
			}

			_, err = m.CreateWorktree(t.Context(), "feature-a", tc.opts)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestCreateAndRemoveWorktree(t *testing.T) {
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
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}
	if target.Path == "" {
		t.Fatalf("expected non-empty target path, got %+v", target)
	}
	if target.Branch != "feature-a" {
		t.Fatalf("expected branch feature-a, got %+v", target)
	}

	if _, err := os.Stat(target.Path); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}

	if err := m.Remove(t.Context(), []string{"feature-a"}, RemoveWorktreeOptions{Force: true}); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(target.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected worktree path to be removed, stat err=%v", err)
	}
}

func TestCreateWorktreeCopiesIncludedFiles(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	envPath := filepath.Join(repoDir, ".env.local")
	if err := os.WriteFile(envPath, []byte("KEY=VALUE\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.env.local): %v", err)
	}

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "--add", "wr.copy.include", ".env.local"); err != nil {
		t.Fatalf("git config --add wr.copy.include: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	target, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
		FromCurrent: true,
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	copied := filepath.Join(target.Path, ".env.local")
	b, err := os.ReadFile(copied)
	if err != nil {
		t.Fatalf("ReadFile(copied): %v", err)
	}
	if diff := cmp.Diff("KEY=VALUE\n", string(b)); diff != "" {
		t.Fatalf("copied file mismatch (-want +got):\n%s", diff)
	}
}

func TestHooksPostCreateAndPostRemove(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	postCreate := "echo hi > .hooked"
	postRemove := "echo $WORKTREE_PATH > removed.txt"
	if runtime.GOOS == "windows" {
		postCreate = "echo hi> .hooked"
		postRemove = "echo %WORKTREE_PATH%> removed.txt"
	}

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "--add", "wr.hook.postCreate", postCreate); err != nil {
		t.Fatalf("git config --add wr.hook.postCreate: %v", err)
	}
	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "--add", "wr.hook.postRemove", postRemove); err != nil {
		t.Fatalf("git config --add wr.hook.postRemove: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	target, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
		FromCurrent: true,
	})
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target.Path, ".hooked")); err != nil {
		t.Fatalf("expected postCreate hook file to exist: %v", err)
	}

	if err := m.Remove(t.Context(), []string{"feature-a"}, RemoveWorktreeOptions{Force: true}); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(repoDir, "removed.txt"))
	if err != nil {
		t.Fatalf("expected postRemove hook file to exist: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected removed.txt to contain WORKTREE_PATH, got empty")
	}
}
