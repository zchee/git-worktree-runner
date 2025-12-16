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
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestManagerRun(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	tests := map[string]struct {
		setupWorktree bool
		identifier    string
		argv          []string

		wantExit        int
		wantExitNonZero bool
		wantErr         bool

		wantStdoutSub string
		wantStderrSub string
	}{
		"success: runs git status in worktree": {
			setupWorktree: true,
			identifier:    "feature-a",
			argv:          []string{"git", "status"},
			wantExit:      0,
			wantStdoutSub: "On branch feature-a",
		},
		"success: non-zero exit code is not an error": {
			identifier:      "1",
			argv:            []string{"git", "rev-parse", "--verify", "refs/heads/does-not-exist"},
			wantExitNonZero: true,
			wantStderrSub:   "fatal",
			wantStdoutSub:   "",
			wantErr:         false,
			setupWorktree:   false,
		},
		"error: unknown identifier returns error": {
			identifier: "does-not-exist",
			argv:       []string{"git", "status"},
			wantErr:    true,
		},
		"error: no argv returns error": {
			identifier: "1",
			argv:       nil,
			wantErr:    true,
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

			if tc.setupWorktree {
				if _, err := m.CreateWorktree(t.Context(), "feature-a", CreateWorktreeOptions{
					FromCurrent: true,
					NoCopy:      true,
				}); err != nil {
					t.Fatalf("CreateWorktree() error: %v", err)
				}
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			gotExit, err := m.Run(t.Context(), tc.identifier, tc.argv, RunOptions{
				IO: ExecIO{
					Stdin:  strings.NewReader(""),
					Stdout: &stdout,
					Stderr: &stderr,
				},
			})

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (exit=%d)", gotExit)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error: %v (exit=%d)", err, gotExit)
			}

			if tc.wantExitNonZero {
				if gotExit == 0 {
					t.Fatalf("expected non-zero exit code, got %d", gotExit)
				}
			} else if diff := cmp.Diff(tc.wantExit, gotExit); diff != "" {
				t.Fatalf("exit code mismatch (-want +got):\n%s", diff)
			}

			if tc.wantStdoutSub != "" && !strings.Contains(stdout.String(), tc.wantStdoutSub) {
				t.Fatalf("stdout mismatch: expected substring %q, got %q", tc.wantStdoutSub, stdout.String())
			}
			if tc.wantStderrSub != "" && !strings.Contains(stderr.String(), tc.wantStderrSub) {
				t.Fatalf("stderr mismatch: expected substring %q, got %q", tc.wantStderrSub, stderr.String())
			}
		})
	}
}
