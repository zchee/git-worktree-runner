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

package repoctx

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestDiscoverMainRepo(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	testutil.InitRepo(t, g, repoDir)

	got, err := Discover(t.Context(), g, repoDir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	repoDir, err = filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	wantCommon := filepath.Join(repoDir, ".git")
	if diff := cmp.Diff(repoDir, got.MainRoot); diff != "" {
		t.Fatalf("MainRoot mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantCommon, got.CommonDir); diff != "" {
		t.Fatalf("CommonDir mismatch (-want +got):\n%s", diff)
	}
}

func TestDiscoverFromWorktree(t *testing.T) {
	t.Parallel()

	g := testutil.Git(t)

	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "repo")
	worktreeDir := filepath.Join(tmp, "wt1")
	testutil.InitRepo(t, g, mainDir)
	testutil.AddWorktree(t, g, mainDir, worktreeDir, "foo")

	got, err := Discover(t.Context(), g, worktreeDir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	mainDir, err = filepath.EvalSymlinks(mainDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	wantCommon := filepath.Join(mainDir, ".git")
	if diff := cmp.Diff(mainDir, got.MainRoot); diff != "" {
		t.Fatalf("MainRoot mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantCommon, got.CommonDir); diff != "" {
		t.Fatalf("CommonDir mismatch (-want +got):\n%s", diff)
	}
}
