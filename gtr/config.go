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

package gtr

import (
	"context"
	"errors"
	"strings"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
)

// ConfigGet returns config values for key in local or global scope.
func (m *Manager) ConfigGet(ctx context.Context, key string, global bool) ([]string, error) {
	args := []string{"config", "--local", "--get-all", key}
	if global {
		args = []string{"config", "--global", "--get-all", key}
	}

	res, err := m.git.Run(ctx, m.repoCtx.MainRoot, args...)
	if err == nil {
		if res.Stdout == "" {
			return nil, nil
		}
		return strings.Split(res.Stdout, "\n"), nil
	}

	var ee *gitcmd.ExitError
	if errors.As(err, &ee) && ee.ExitCode == 1 {
		return nil, nil
	}
	return nil, err
}

// ConfigSet sets a config key in local or global scope.
func (m *Manager) ConfigSet(ctx context.Context, key, value string, global bool) error {
	return m.cfg.Set(ctx, key, value, global)
}

// ConfigAdd adds a value to a multi-valued config key in local or global scope.
func (m *Manager) ConfigAdd(ctx context.Context, key, value string, global bool) error {
	return m.cfg.Add(ctx, key, value, global)
}

// ConfigUnset unsets all values for a key in local or global scope.
func (m *Manager) ConfigUnset(ctx context.Context, key string, global bool) error {
	return m.cfg.Unset(ctx, key, global)
}
