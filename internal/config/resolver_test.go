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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestResolverDefault(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		localValue  string
		fileValue   string
		globalValue string
		envValue    string
		want        string
	}{
		"success: local overrides file and global": {
			localValue:  "vscode",
			fileValue:   "cursor",
			globalValue: "zed",
			envValue:    "vim",
			want:        "vscode",
		},
		"success: file overrides global": {
			fileValue:   "cursor",
			globalValue: "zed",
			envValue:    "vim",
			want:        "cursor",
		},
		"success: global overrides env": {
			globalValue: "zed",
			envValue:    "vim",
			want:        "zed",
		},
		"success: env overrides fallback": {
			envValue: "vim",
			want:     "vim",
		},
		"success: fallback when unset": {
			want: "none",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := testutil.Git(t)
			repoDir := filepath.Join(t.TempDir(), "repo")
			testutil.InitRepo(t, g, repoDir)

			if tc.globalValue != "" {
				if _, err := g.Run(t.Context(), repoDir, "config", "--global", "wr.editor.default", tc.globalValue); err != nil {
					t.Fatalf("git config --global: %v", err)
				}
			}

			if tc.fileValue != "" {
				content := []byte("[defaults]\n\teditor = " + tc.fileValue + "\n")
				if err := os.WriteFile(filepath.Join(repoDir, ".wrconfig"), content, 0o644); err != nil {
					t.Fatalf("WriteFile(.wrconfig): %v", err)
				}
			}

			if tc.localValue != "" {
				if _, err := g.Run(t.Context(), repoDir, "config", "--local", "wr.editor.default", tc.localValue); err != nil {
					t.Fatalf("git config --local: %v", err)
				}
			}

			env := map[string]string{}
			if tc.envValue != "" {
				env["GTR_EDITOR_DEFAULT"] = tc.envValue
			}

			r := New(g, repoDir, env)

			got, err := r.Default(t.Context(), "wr.editor.default", "GTR_EDITOR_DEFAULT", "none", "defaults.editor")
			if err != nil {
				t.Fatalf("Default() error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("value mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolverAll(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		local  []string
		file   []string
		global []string
		system []string
		want   []string
	}{
		"success: merges and deduplicates in precedence order": {
			local:  []string{"a", "b"},
			file:   []string{"b", "c"},
			global: []string{"c", "d"},
			system: []string{"d", "e"},
			want:   []string{"a", "b", "c", "d", "e"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := testutil.Git(t)
			repoDir := filepath.Join(t.TempDir(), "repo")
			testutil.InitRepo(t, g, repoDir)

			for _, v := range tc.local {
				if _, err := g.Run(t.Context(), repoDir, "config", "--local", "--add", "wr.copy.include", v); err != nil {
					t.Fatalf("git config --local --add: %v", err)
				}
			}
			for _, v := range tc.global {
				if _, err := g.Run(t.Context(), repoDir, "config", "--global", "--add", "wr.copy.include", v); err != nil {
					t.Fatalf("git config --global --add: %v", err)
				}
			}
			for _, v := range tc.system {
				if _, err := g.Run(t.Context(), repoDir, "config", "--system", "--add", "wr.copy.include", v); err != nil {
					t.Fatalf("git config --system --add: %v", err)
				}
			}

			if len(tc.file) > 0 {
				var b []byte
				b = append(b, []byte("[copy]\n")...)
				for _, v := range tc.file {
					b = append(b, []byte("\tinclude = "+v+"\n")...)
				}
				if err := os.WriteFile(filepath.Join(repoDir, ".wrconfig"), b, 0o644); err != nil {
					t.Fatalf("WriteFile(.wrconfig): %v", err)
				}
			}

			r := New(g, repoDir, nil)

			got, err := r.All(t.Context(), "wr.copy.include", "copy.include")
			if err != nil {
				t.Fatalf("All() error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("values mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolverWorktreeIncludePatterns(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fileContents string
		want         []string
	}{
		"success: strips comments and blanks": {
			fileContents: "\n# comment\n**/.env.example\n\napps/*/run.sh\n",
			want:         []string{"**/.env.example", "apps/*/run.sh"},
		},
		"success: missing file returns nil": {
			want: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := testutil.Git(t)
			repoDir := filepath.Join(t.TempDir(), "repo")
			testutil.InitRepo(t, g, repoDir)

			if tc.fileContents != "" {
				if err := os.WriteFile(filepath.Join(repoDir, ".worktreeinclude"), []byte(tc.fileContents), 0o644); err != nil {
					t.Fatalf("WriteFile(.worktreeinclude): %v", err)
				}
			}

			r := New(g, repoDir, nil)

			got, err := r.WorktreeIncludePatterns()
			if err != nil {
				t.Fatalf("WorktreeIncludePatterns() error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("patterns mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
