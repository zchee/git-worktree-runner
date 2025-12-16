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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/config"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestResolvePaths(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		localConfig map[string]string
		env         map[string]string
		wantPrefix  string
		wantBase    func(repoDir string, env map[string]string) (string, error)
	}{
		"success: default base dir is sibling": {
			wantBase: func(repoDir string, _ map[string]string) (string, error) {
				repoDir, err := pathutil.Canonicalize(repoDir)
				if err != nil {
					return "", err
				}
				repoName := filepath.Base(repoDir)
				return pathutil.Canonicalize(filepath.Join(filepath.Dir(repoDir), repoName+"-worktrees"))
			},
		},
		"success: repo-relative base dir": {
			localConfig: map[string]string{
				"gtr.worktrees.dir":    ".worktrees",
				"gtr.worktrees.prefix": "wt-",
			},
			wantPrefix: "wt-",
			wantBase: func(repoDir string, _ map[string]string) (string, error) {
				repoDir, err := pathutil.Canonicalize(repoDir)
				if err != nil {
					return "", err
				}
				return pathutil.Canonicalize(filepath.Join(repoDir, ".worktrees"))
			},
		},
		"success: env base dir supports tilde": {
			env: map[string]string{
				"GTR_WORKTREES_DIR": "~/worktrees-test",
			},
			wantBase: func(_ string, env map[string]string) (string, error) {
				expanded, err := pathutil.ExpandTilde(env["GTR_WORKTREES_DIR"])
				if err != nil {
					return "", err
				}
				return pathutil.Canonicalize(expanded)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := testutil.Git(t)
			repoDir := filepath.Join(t.TempDir(), "repo")
			testutil.InitRepo(t, g, repoDir)

			repoDir, err := pathutil.Canonicalize(repoDir)
			if err != nil {
				t.Fatalf("Canonicalize(repoDir): %v", err)
			}

			for k, v := range tc.localConfig {
				if _, err := g.Run(t.Context(), repoDir, "config", "--local", k, v); err != nil {
					t.Fatalf("git config --local %s: %v", k, err)
				}
			}

			cfg := config.New(g, repoDir, tc.env)

			got, err := ResolvePaths(t.Context(), cfg)
			if err != nil {
				t.Fatalf("ResolvePaths() error: %v", err)
			}

			if tc.wantPrefix != "" {
				if diff := cmp.Diff(tc.wantPrefix, got.Prefix); diff != "" {
					t.Fatalf("prefix mismatch (-want +got):\n%s", diff)
				}
			}

			wantBase, err := tc.wantBase(repoDir, tc.env)
			if err != nil {
				t.Fatalf("wantBase() error: %v", err)
			}
			if diff := cmp.Diff(wantBase, got.BaseDir); diff != "" {
				t.Fatalf("base dir mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
