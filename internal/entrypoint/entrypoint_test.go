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

package entrypoint

import (
	"context"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	ctx := context.Background()
	stdin := strings.NewReader("")
	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := Run(ctx, []string{"version"}, stdin, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.HasPrefix(got, "git gtr version ") {
		t.Fatalf("stdout = %q, want prefix %q", got, "git gtr version ")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestMain_Help_DefaultsToHelp(t *testing.T) {
	stdin := strings.NewReader("")
	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := Main(nil, stdin, &stdout, &stderr)
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
