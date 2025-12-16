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

package hooks

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRun(t *testing.T) {
	t.Parallel()

	echoEnv := "echo $FOO"
	exit7 := "exit 7"
	if runtime.GOOS == "windows" {
		echoEnv = "echo %FOO%"
		exit7 = "exit /B 7"
	}

	tests := map[string]struct {
		hooks     []string
		env       []string
		wantOut   string
		wantErr   error
		wantExit  int
		wantPhase string
		wantIndex int
	}{
		"success: runs hook with env": {
			hooks:   []string{echoEnv},
			env:     []string{"FOO=bar"},
			wantOut: "bar",
		},
		"error: failing hook returns HookError": {
			hooks:     []string{exit7},
			wantErr:   ErrHookFailed,
			wantExit:  7,
			wantPhase: "postCreate",
			wantIndex: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			err := Run(t.Context(), "postCreate", t.TempDir(), tc.hooks, tc.env, Options{
				Stdout: &stdout,
				Stderr: &stderr,
			})
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}

				var he *HookError
				if !errors.As(err, &he) {
					t.Fatalf("expected *HookError, got %T", err)
				}
				if diff := cmp.Diff(tc.wantExit, he.ExitCode); diff != "" {
					t.Fatalf("exit code mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.wantPhase, he.Phase); diff != "" {
					t.Fatalf("phase mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.wantIndex, he.Index); diff != "" {
					t.Fatalf("index mismatch (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error: %v (stderr=%q)", err, stderr.String())
			}

			if tc.wantOut != "" && !strings.Contains(stdout.String(), tc.wantOut) {
				t.Fatalf("stdout mismatch: expected %q in %q", tc.wantOut, stdout.String())
			}
		})
	}
}
