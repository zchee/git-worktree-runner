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

package pathutil

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCanonicalize(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path      string
		wantErr   bool
		wantAbs   bool
		wantEmpty bool
	}{
		"success: existing directory": {
			path:    t.TempDir(),
			wantAbs: true,
		},
		"success: non-existent path returns absolute": {
			path:    filepath.Join(t.TempDir(), "does-not-exist"),
			wantAbs: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := Canonicalize(tc.path)
			if tc.wantErr != (err != nil) {
				t.Fatalf("error presence mismatch: wantErr=%v gotErr=%v err=%v", tc.wantErr, err != nil, err)
			}
			if tc.wantEmpty && got != "" {
				t.Fatalf("expected empty, got %q", got)
			}
			if tc.wantAbs && got != "" && !filepath.IsAbs(got) {
				t.Fatalf("expected absolute path, got %q", got)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input       string
		wantChanged bool
	}{
		"success: tilde": {
			input:       "~",
			wantChanged: true,
		},
		"success: tilde slash": {
			input:       "~/x",
			wantChanged: true,
		},
		"success: not tilde": {
			input: "x/y",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := ExpandTilde(tc.input)
			if err != nil {
				t.Fatalf("ExpandTilde() error: %v", err)
			}
			if tc.wantChanged && got == tc.input {
				t.Fatalf("expected expansion for %q, got unchanged", tc.input)
			}
			if !tc.wantChanged {
				if diff := cmp.Diff(tc.input, got); diff != "" {
					t.Fatalf("result mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
