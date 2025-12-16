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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestManagerConfigSetGetAddUnset(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	if err := m.ConfigSet(t.Context(), "wr.worktrees.prefix", "wt-", false); err != nil {
		t.Fatalf("ConfigSet() error: %v", err)
	}

	values, err := m.ConfigGet(t.Context(), "wr.worktrees.prefix", false)
	if err != nil {
		t.Fatalf("ConfigGet() error: %v", err)
	}
	if diff := cmp.Diff([]string{"wt-"}, values); diff != "" {
		t.Fatalf("values mismatch (-want +got):\n%s", diff)
	}

	if err := m.ConfigAdd(t.Context(), "wr.copy.include", ".env.local", false); err != nil {
		t.Fatalf("ConfigAdd() error: %v", err)
	}
	if err := m.ConfigAdd(t.Context(), "wr.copy.include", ".env.example", false); err != nil {
		t.Fatalf("ConfigAdd() error: %v", err)
	}

	values, err = m.ConfigGet(t.Context(), "wr.copy.include", false)
	if err != nil {
		t.Fatalf("ConfigGet() error: %v", err)
	}
	if diff := cmp.Diff([]string{".env.local", ".env.example"}, values); diff != "" {
		t.Fatalf("values mismatch (-want +got):\n%s", diff)
	}

	if err := m.ConfigUnset(t.Context(), "wr.copy.include", false); err != nil {
		t.Fatalf("ConfigUnset() error: %v", err)
	}
	values, err = m.ConfigGet(t.Context(), "wr.copy.include", false)
	if err != nil {
		t.Fatalf("ConfigGet() error: %v", err)
	}
	if diff := cmp.Diff([]string(nil), values); diff != "" {
		t.Fatalf("values mismatch (-want +got):\n%s", diff)
	}
}
