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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/gitx"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestListPorcelain(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		return gitx.CurrentBranchGit(ctx, g, dir)
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	got := map[string]PorcelainEntry{}
	for _, e := range entries {
		got[e.Path] = e
	}

	mainEntry, ok := got[repoDir]
	if !ok {
		t.Fatalf("expected main entry path %q in %+v", repoDir, entries)
	}
	if mainEntry.Path == "" || mainEntry.Branch == "" {
		t.Fatalf("expected populated main entry, got %+v", mainEntry)
	}

	wtEntry, ok := got[worktreeDir]
	if !ok {
		t.Fatalf("expected worktree entry path %q in %+v", worktreeDir, entries)
	}
	if diff := cmp.Diff("foo", wtEntry.Branch); diff != "" {
		t.Fatalf("worktree branch mismatch (-want +got):\n%s", diff)
	}
	if wtEntry.Detached {
		t.Fatalf("expected non-detached worktree, got %+v", wtEntry)
	}
}

func TestListPorcelainUsesMetaHEADWhenAvailable(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		_ = ctx
		_ = dir
		return "", errors.New("unexpected branch resolver call")
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	got := map[string]PorcelainEntry{}
	for _, e := range entries {
		got[e.Path] = e
	}

	if _, ok := got[repoDir]; !ok {
		t.Fatalf("expected main entry path %q in %+v", repoDir, entries)
	}
	if diff := cmp.Diff("foo", got[worktreeDir].Branch); diff != "" {
		t.Fatalf("worktree branch mismatch (-want +got):\n%s", diff)
	}
}

func TestListPorcelainDetachedWorktree(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	if _, err := g.Run(t.Context(), worktreeDir, "checkout", "--detach"); err != nil {
		t.Fatalf("git checkout --detach: %v", err)
	}

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		return gitx.CurrentBranchGit(ctx, g, dir)
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	var found *PorcelainEntry
	for _, e := range entries {
		if e.Path == worktreeDir {
			e := e
			found = &e
			break
		}
	}
	if found == nil {
		t.Fatalf("expected worktree entry path %q in %+v", worktreeDir, entries)
	}
	if !found.Detached {
		t.Fatalf("expected detached worktree, got %+v", *found)
	}
	if diff := cmp.Diff("(detached)", found.Branch); diff != "" {
		t.Fatalf("branch mismatch (-want +got):\n%s", diff)
	}
}

func TestWorktreePathFromMeta(t *testing.T) {
	t.Parallel()

	t.Run("absolute", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		metaDir := filepath.Join(root, "meta")
		if err := os.MkdirAll(metaDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(metaDir): %v", err)
		}

		worktreeDir := filepath.Join(root, "wt")
		if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktreeDir): %v", err)
		}
		if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: /dev/null\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(worktree .git): %v", err)
		}

		gitdirPath := filepath.Join(worktreeDir, ".git")
		if err := os.WriteFile(filepath.Join(metaDir, "gitdir"), []byte(gitdirPath+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(meta gitdir): %v", err)
		}

		got, err := worktreePathFromMeta(metaDir)
		if err != nil {
			t.Fatalf("worktreePathFromMeta() error: %v", err)
		}
		want, err := pathutil.Canonicalize(worktreeDir)
		if err != nil {
			t.Fatalf("Canonicalize(worktreeDir): %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("worktree path mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("relative", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		metaDir := filepath.Join(root, "meta")
		if err := os.MkdirAll(metaDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(metaDir): %v", err)
		}

		worktreeDir := filepath.Join(root, "wt")
		if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktreeDir): %v", err)
		}
		if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: /dev/null\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(worktree .git): %v", err)
		}

		gitdirAbs := filepath.Join(worktreeDir, ".git")
		gitdirRel, err := filepath.Rel(metaDir, gitdirAbs)
		if err != nil {
			t.Fatalf("Rel(metaDir, gitdirAbs): %v", err)
		}
		if err := os.WriteFile(filepath.Join(metaDir, "gitdir"), []byte(gitdirRel+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(meta gitdir): %v", err)
		}

		got, err := worktreePathFromMeta(metaDir)
		if err != nil {
			t.Fatalf("worktreePathFromMeta() error: %v", err)
		}
		want, err := pathutil.Canonicalize(worktreeDir)
		if err != nil {
			t.Fatalf("Canonicalize(worktreeDir): %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("worktree path mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty_gitdir_file", func(t *testing.T) {
		t.Parallel()

		metaDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(metaDir, "gitdir"), []byte("\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(meta gitdir): %v", err)
		}

		if _, err := worktreePathFromMeta(metaDir); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestWorktreeBranchFromMeta(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		writeHead    bool
		headContents string
		wantBranch   string
		wantDetached bool
	}

	cases := []testCase{
		{
			name:         "missing_HEAD_file",
			writeHead:    false,
			wantBranch:   "(detached)",
			wantDetached: true,
		},
		{
			name:         "empty_HEAD_file",
			writeHead:    true,
			headContents: "\n",
			wantBranch:   "(detached)",
			wantDetached: true,
		},
		{
			name:         "symbolic_head_local_branch",
			writeHead:    true,
			headContents: "ref: refs/heads/foo\n",
			wantBranch:   "foo",
			wantDetached: false,
		},
		{
			name:         "symbolic_head_other_ref",
			writeHead:    true,
			headContents: "ref: refs/remotes/origin/main\n",
			wantBranch:   "refs/remotes/origin/main",
			wantDetached: false,
		},
		{
			name:         "hash_head_detached",
			writeHead:    true,
			headContents: "0123456789012345678901234567890123456789\n",
			wantBranch:   "(detached)",
			wantDetached: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metaDir := t.TempDir()
			if tc.writeHead {
				if err := os.WriteFile(filepath.Join(metaDir, "HEAD"), []byte(tc.headContents), 0o644); err != nil {
					t.Fatalf("WriteFile(meta HEAD): %v", err)
				}
			}

			gotBranch, gotDetached, err := worktreeBranchFromMeta(metaDir)
			if err != nil {
				t.Fatalf("worktreeBranchFromMeta() error: %v", err)
			}
			if diff := cmp.Diff(tc.wantBranch, gotBranch); diff != "" {
				t.Fatalf("branch mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantDetached, gotDetached); diff != "" {
				t.Fatalf("detached mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListPorcelainLockedWorktree(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	if _, err := g.Run(t.Context(), repoDir, "worktree", "lock", worktreeDir); err != nil {
		t.Fatalf("git worktree lock: %v", err)
	}

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		return gitx.CurrentBranchGit(ctx, g, dir)
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	var found *PorcelainEntry
	for _, e := range entries {
		if e.Path == worktreeDir {
			e := e
			found = &e
			break
		}
	}
	if found == nil {
		t.Fatalf("expected worktree entry path %q in %+v", worktreeDir, entries)
	}
	if !found.Locked {
		t.Fatalf("expected locked worktree, got %+v", *found)
	}
}

func TestListPorcelainPrunableWorktree(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")
	testutil.InitRepo(t, g, repoDir)
	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatalf("RemoveAll(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		return gitx.CurrentBranchGit(ctx, g, dir)
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	var found *PorcelainEntry
	for _, e := range entries {
		if e.Path == worktreeDir {
			e := e
			found = &e
			break
		}
	}
	if found == nil {
		t.Fatalf("expected worktree entry path %q in %+v", worktreeDir, entries)
	}
	if !found.Prunable {
		t.Fatalf("expected prunable worktree, got %+v", *found)
	}
}

func TestListPorcelainReftableRepo(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)
	repoDir := filepath.Join(t.TempDir(), "repo")
	worktreeDir := filepath.Join(t.TempDir(), "wt1")

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir): %v", err)
	}
	if _, err := g.Run(t.Context(), repoDir, "init", "--ref-format=reftable", "--initial-branch=main"); err != nil {
		t.Fatalf("git init --ref-format=reftable: %v", err)
	}

	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("hi\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", readmePath, err)
	}
	if _, err := g.Run(t.Context(), repoDir, "add", "README.md"); err != nil {
		t.Fatalf("git add README.md (%q): %v", repoDir, err)
	}
	if _, err := g.Run(t.Context(), repoDir, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit (%q): %v", repoDir, err)
	}

	testutil.AddWorktree(t, g, repoDir, worktreeDir, "foo")

	repoDir, err := pathutil.Canonicalize(repoDir)
	if err != nil {
		t.Fatalf("Canonicalize(repoDir): %v", err)
	}
	worktreeDir, err = pathutil.Canonicalize(worktreeDir)
	if err != nil {
		t.Fatalf("Canonicalize(worktreeDir): %v", err)
	}

	resolveBranch := func(ctx context.Context, dir string) (string, error) {
		return gitx.CurrentBranchGit(ctx, g, dir)
	}

	entries, err := ListPorcelain(t.Context(), filepath.Join(repoDir, ".git"), repoDir, resolveBranch)
	if err != nil {
		t.Fatalf("ListPorcelain() error: %v", err)
	}

	got := map[string]PorcelainEntry{}
	for _, e := range entries {
		got[e.Path] = e
	}

	mainEntry, ok := got[repoDir]
	if !ok {
		t.Fatalf("expected main entry path %q in %+v", repoDir, entries)
	}
	if diff := cmp.Diff("main", mainEntry.Branch); diff != "" {
		t.Fatalf("main branch mismatch (-want +got):\n%s", diff)
	}

	wtEntry, ok := got[worktreeDir]
	if !ok {
		t.Fatalf("expected worktree entry path %q in %+v", worktreeDir, entries)
	}
	if diff := cmp.Diff("foo", wtEntry.Branch); diff != "" {
		t.Fatalf("worktree branch mismatch (-want +got):\n%s", diff)
	}
}
