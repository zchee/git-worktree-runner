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

package testutil

import (
	"path/filepath"
	"testing"
)

// SetGitProcessEnv configures process-level environment variables so that `git` is deterministic in tests.
//
// Use this when the code under test shells out to `git` without a custom environment (for example, via exec.Command).
func SetGitProcessEnv(t *testing.T) {
	t.Helper()

	cfgDir := t.TempDir()
	globalConfig := filepath.Join(cfgDir, "gitconfig")
	systemConfig := filepath.Join(cfgDir, "gitconfig-system")

	t.Setenv("GIT_AUTHOR_NAME", "git-gtr-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "git-gtr-test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "git-gtr-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "git-gtr-test@example.invalid")

	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", systemConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}
