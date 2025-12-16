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

package platform

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOpenCommand(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		goos    string
		path    string
		wantBin string
		wantArg []string
		wantErr bool
	}{
		"success: darwin": {
			goos:    "darwin",
			path:    "/tmp/x",
			wantBin: "open",
			wantArg: []string{"/tmp/x"},
		},
		"success: linux": {
			goos:    "linux",
			path:    "/tmp/x",
			wantBin: "xdg-open",
			wantArg: []string{"/tmp/x"},
		},
		"success: windows": {
			goos:    "windows",
			path:    "C:\\x",
			wantBin: "cmd.exe",
			wantArg: []string{"/C", "start", "", "C:\\x"},
		},
		"error: unknown platform": {
			goos:    "plan9",
			path:    "/tmp/x",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotBin, gotArgs, err := openCommand(tc.goos, tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("openCommand() error: %v", err)
			}
			if diff := cmp.Diff(tc.wantBin, gotBin); diff != "" {
				t.Fatalf("bin mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantArg, gotArgs); diff != "" {
				t.Fatalf("args mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
