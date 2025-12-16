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

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
)

// Git returns a gitcmd.Git configured for tests.
func Git(t *testing.T) gitcmd.Git {
	t.Helper()

	g, err := gitcmd.New()
	if err != nil {
		t.Fatalf("gitcmd.New() error: %v", err)
	}

	cfgDir := t.TempDir()
	globalConfig := filepath.Join(cfgDir, "gitconfig")
	systemConfig := filepath.Join(cfgDir, "gitconfig-system")

	g.Env = []string{
		"GIT_AUTHOR_NAME=git-wr-test",
		"GIT_AUTHOR_EMAIL=git-wr-test@example.invalid",
		"GIT_COMMITTER_NAME=git-wr-test",
		"GIT_COMMITTER_EMAIL=git-wr-test@example.invalid",
		"GIT_CONFIG_GLOBAL=" + globalConfig,
		"GIT_CONFIG_SYSTEM=" + systemConfig,
		"GIT_CONFIG_NOSYSTEM=1",
	}

	return g
}
