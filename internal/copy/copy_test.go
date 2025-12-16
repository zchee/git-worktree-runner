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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCopyFiles(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupSrc       func(t *testing.T, srcRoot string)
		include        []string
		exclude        []string
		opts           Options
		wantCopied     []string
		wantFilesExist []string
		wantErr        error
	}{
		"success: copies globbed files preserving paths": {
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
				mustWrite(".env.example", "root\n")
				mustWrite("apps/app1/.env.example", "nested\n")
				mustWrite("apps/app1/secret.env", "secret\n")
			},
			include:        []string{"**/.env.example"},
			exclude:        []string{"**/secret.env"},
			opts:           Options{PreservePaths: true},
			wantCopied:     []string{".env.example", "apps/app1/.env.example"},
			wantFilesExist: []string{".env.example", "apps/app1/.env.example"},
		},
		"success: dry-run does not write files": {
			setupSrc: func(t *testing.T, srcRoot string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(srcRoot, "README.md"), []byte("hi\n"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			},
			include:    []string{"README.md"},
			opts:       Options{DryRun: true, PreservePaths: true},
			wantCopied: []string{"README.md"},
		},
		"success: flatten copies to base name": {
			setupSrc: func(t *testing.T, srcRoot string) {
				t.Helper()
				p := filepath.Join(srcRoot, "docs", "README.md")
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(p, []byte("docs\n"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			},
			include:        []string{"docs/README.md"},
			opts:           Options{PreservePaths: false},
			wantCopied:     []string{"docs/README.md"},
			wantFilesExist: []string{"README.md"},
		},
		"error: unsafe pattern rejected": {
			setupSrc: func(t *testing.T, _ string) {},
			include:  []string{"../*"},
			opts:     Options{PreservePaths: true},
			wantErr:  ErrUnsafePattern,
		},
		"error: no patterns": {
			setupSrc: func(t *testing.T, _ string) {},
			include:  nil,
			opts:     Options{PreservePaths: true},
			wantErr:  ErrNoPatterns,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srcRoot := t.TempDir()
			dstRoot := t.TempDir()

			tc.setupSrc(t, srcRoot)

			got, err := CopyFiles(t.Context(), srcRoot, dstRoot, tc.include, tc.exclude, tc.opts)
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
				t.Fatalf("CopyFiles() error: %v", err)
			}

			if diff := cmp.Diff(tc.wantCopied, got.CopiedFiles); diff != "" {
				t.Fatalf("copied mismatch (-want +got):\n%s", diff)
			}

			for _, rel := range tc.wantFilesExist {
				p := filepath.Join(dstRoot, filepath.FromSlash(rel))
				if _, err := os.Stat(p); err != nil {
					t.Fatalf("expected file %q to exist: %v", p, err)
				}
			}

			if tc.opts.DryRun {
				for _, rel := range tc.wantFilesExist {
					p := filepath.Join(dstRoot, filepath.FromSlash(rel))
					if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("expected dry-run to not create %q, stat err=%v", p, err)
					}
				}
			}
		})
	}
}
