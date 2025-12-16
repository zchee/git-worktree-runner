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

package copy

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCopyDirectories(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupSrc    func(t *testing.T, srcRoot string)
		includes    []string
		excludes    []string
		wantDirs    []string
		wantPresent []string
		wantAbsent  []string
		wantErr     error
	}{
		"success: copies named directories and excludes subpaths": {
			setupSrc: func(t *testing.T, srcRoot string) {
				t.Helper()
				mustWrite := func(rel, contents string) {
					t.Helper()
					p := filepath.Join(srcRoot, filepath.FromSlash(rel))
					if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
						t.Fatalf("MkdirAll: %v", err)
					}
					if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
						t.Fatalf("WriteFile: %v", err)
					}
				}
				mustWrite("node_modules/pkg/index.js", "ok\n")
				mustWrite("node_modules/.cache/secret.txt", "no\n")
				mustWrite("apps/node_modules/pkg2/index.js", "ok2\n")
			},
			includes: []string{"node_modules"},
			excludes: []string{"*/.cache"},
			wantDirs: []string{"node_modules", "apps/node_modules"},
			wantPresent: []string{
				"node_modules/pkg/index.js",
				"apps/node_modules/pkg2/index.js",
			},
			wantAbsent: []string{
				"node_modules/.cache/secret.txt",
			},
		},
		"error: unsafe include pattern rejected": {
			setupSrc: func(t *testing.T, _ string) {},
			includes: []string{"../node_modules"},
			wantErr:  ErrUnsafePattern,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srcRoot := t.TempDir()
			dstRoot := t.TempDir()

			tc.setupSrc(t, srcRoot)

			got, err := CopyDirectories(t.Context(), srcRoot, dstRoot, tc.includes, tc.excludes)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CopyDirectories() error: %v", err)
			}

			wantDirs := append([]string(nil), tc.wantDirs...)
			sort.Strings(wantDirs)
			sort.Strings(got.CopiedDirs)

			if diff := cmp.Diff(wantDirs, got.CopiedDirs); diff != "" {
				t.Fatalf("copied dirs mismatch (-want +got):\n%s", diff)
			}

			for _, rel := range tc.wantPresent {
				p := filepath.Join(dstRoot, filepath.FromSlash(rel))
				if _, err := os.Stat(p); err != nil {
					t.Fatalf("expected %q to exist: %v", p, err)
				}
			}
			for _, rel := range tc.wantAbsent {
				p := filepath.Join(dstRoot, filepath.FromSlash(rel))
				if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("expected %q to be absent: stat err=%v", p, err)
				}
			}
		})
	}
}
