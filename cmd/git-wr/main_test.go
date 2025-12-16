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

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunnerRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args           []string
		version        string
		wantExitCode   int
		wantStdoutSub  string
		wantStderrSub  string
		wantStdoutZero bool
		wantStderrZero bool
	}{
		"success: no args prints help": {
			args:           nil,
			version:        "0.0.0-test",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  "git wr - Git worktree runner",
			wantStderrZero: true,
		},
		"success: help prints help": {
			args:           []string{"help"},
			version:        "0.0.0-test",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  "git wr - Git worktree runner",
			wantStderrZero: true,
		},
		"success: version prints version": {
			args:           []string{"version"},
			version:        "1.2.3",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  versionPrefix + " 1.2.3\n",
			wantStderrZero: true,
		},
		"success: --version prints version": {
			args:           []string{"--version"},
			version:        "1.2.3",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  versionPrefix + " 1.2.3\n",
			wantStderrZero: true,
		},
		"success: -v prints version": {
			args:           []string{"-v"},
			version:        "1.2.3",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  versionPrefix + " 1.2.3\n",
			wantStderrZero: true,
		},
		"success: --help prints help": {
			args:           []string{"--help"},
			version:        "0.0.0-test",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  "git wr - Git worktree runner",
			wantStderrZero: true,
		},
		"success: -h prints help": {
			args:           []string{"-h"},
			version:        "0.0.0-test",
			wantExitCode:   exitSuccess,
			wantStdoutSub:  "git wr - Git worktree runner",
			wantStderrZero: true,
		},
		"error: unknown command returns usage error": {
			args:           []string{"nope"},
			version:        "0.0.0-test",
			wantExitCode:   exitUsage,
			wantStdoutZero: true,
			wantStderrSub:  "Unknown command: nope",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			r := Runner{
				Stdin:  strings.NewReader(""),
				Stdout: &stdout,
				Stderr: &stderr,
				Version: VersionInfo{
					Version: tc.version,
				},
			}

			gotExitCode := r.Run(t.Context(), tc.args)
			if diff := cmp.Diff(tc.wantExitCode, gotExitCode); diff != "" {
				t.Fatalf("exit code mismatch (-want +got):\n%s", diff)
			}

			if tc.wantStdoutZero && stdout.Len() != 0 {
				t.Fatalf("stdout mismatch: expected empty, got %q", stdout.String())
			}
			if tc.wantStderrZero && stderr.Len() != 0 {
				t.Fatalf("stderr mismatch: expected empty, got %q", stderr.String())
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

func TestRun_Version(t *testing.T) {
	ctx := context.Background()
	stdin := strings.NewReader("")
	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := Run(ctx, []string{"version"}, stdin, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.HasPrefix(got, "git wr version ") {
		t.Fatalf("stdout = %q, want prefix %q", got, "git wr version ")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestMain_Help_DefaultsToHelp(t *testing.T) {
	stdin := strings.NewReader("")
	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := Run(t.Context(), nil, stdin, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", exitCode, stderr.String())
	}
	if got := stdout.String(); got == "" {
		t.Fatalf("stdout is empty; want help text")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}
