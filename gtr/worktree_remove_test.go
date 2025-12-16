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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestManagerRemoveDeleteBranchConfirmation(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	tests := map[string]struct {
		yes           bool
		confirmResult bool

		wantDeleted      bool
		wantConfirmCalls int
	}{
		"success: confirm false keeps branch": {
			confirmResult:    false,
			wantDeleted:      false,
			wantConfirmCalls: 1,
		},
		"success: confirm true deletes branch": {
			confirmResult:    true,
			wantDeleted:      true,
			wantConfirmCalls: 1,
		},
		"success: yes mode bypasses confirm and deletes branch": {
			confirmResult:    false,
			yes:              true,
			wantDeleted:      true,
			wantConfirmCalls: 0,
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

			if _, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
				FromCurrent: true,
				NoCopy:      true,
			}); err != nil {
				t.Fatalf("CreateWorktree() error: %v", err)
			}

			var confirmCalls int
			opts := RemoveWorktreeOptions{
				DeleteBranch: true,
				Force:        true,
				Yes:          tc.yes,
				ConfirmDeleteBranch: func(ctx context.Context, branch string) (bool, error) {
					confirmCalls++
					_ = ctx
					if branch == "" {
						return false, errors.New("unexpected empty branch name")
					}
					return tc.confirmResult, nil
				},
			}

			if err := m.Remove(t.Context(), []string{"feature-a"}, opts); err != nil {
				t.Fatalf("Remove() error: %v", err)
			}

			if diff := cmp.Diff(tc.wantConfirmCalls, confirmCalls); diff != "" {
				t.Fatalf("confirm call count mismatch (-want +got):\n%s", diff)
			}

			assertBranchDeleted(t, g, repoDir, "feature-a", tc.wantDeleted)
		})
	}
}

func assertBranchDeleted(t *testing.T, g gitcmd.Git, repoDir, branch string, wantDeleted bool) {
	t.Helper()

	_, err := g.Run(t.Context(), repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if wantDeleted {
		if err == nil {
			t.Fatalf("expected branch %q deleted, but it still exists", branch)
		}
		var exitErr *gitcmd.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode != 1 {
			t.Fatalf("expected exit code 1 for missing branch, got err=%v", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("expected branch %q to exist, got err=%v", branch, err)
	}
}
