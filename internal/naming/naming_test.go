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

package naming

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  string
	}{
		"success: preserves normal branch": {
			input: "feature",
			want:  "feature",
		},
		"success: replaces slashes": {
			input: "feature/auth",
			want:  "feature-auth",
		},
		"success: replaces spaces and trims hyphens": {
			input: "  feature auth  ",
			want:  "feature-auth",
		},
		"success: trims leading and trailing hyphens": {
			input: "/feature/auth/",
			want:  "feature-auth",
		},
		"success: replaces forbidden characters": {
			input: `a:b*c?d"e<f>g|h`,
			want:  "a-b-c-d-e-f-g-h",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeBranchName(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("sanitize mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
