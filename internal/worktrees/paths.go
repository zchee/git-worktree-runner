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

package worktrees

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/zchee/git-worktree-runner/internal/config"
	"github.com/zchee/git-worktree-runner/internal/pathutil"
)

// Paths describes the configured worktrees directory layout.
type Paths struct {
	BaseDir string
	Prefix  string
}

// ResolvePaths resolves the base directory and prefix for worktrees.
//
// Precedence for BaseDir: git config gtr.worktrees.dir > env GTR_WORKTREES_DIR > default (<parent>/<repo>-worktrees).
// Precedence for Prefix: git config gtr.worktrees.prefix > env GTR_WORKTREES_PREFIX > default ("").
func ResolvePaths(ctx context.Context, cfg config.Resolver) (Paths, error) {
	prefix, err := cfg.Default(ctx, "gtr.worktrees.prefix", "GTR_WORKTREES_PREFIX", "", "")
	if err != nil {
		return Paths{}, fmt.Errorf("resolve gtr.worktrees.prefix: %w", err)
	}

	baseDir, err := cfg.Default(ctx, "gtr.worktrees.dir", "GTR_WORKTREES_DIR", "", "")
	if err != nil {
		return Paths{}, fmt.Errorf("resolve gtr.worktrees.dir: %w", err)
	}

	if baseDir == "" {
		repoName := filepath.Base(cfg.MainRoot)
		baseDir = filepath.Join(filepath.Dir(cfg.MainRoot), repoName+"-worktrees")
	} else {
		baseDir, err = pathutil.ExpandTilde(baseDir)
		if err != nil {
			return Paths{}, fmt.Errorf("expand tilde for %q: %w", baseDir, err)
		}
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(cfg.MainRoot, baseDir)
		}
	}

	baseDir, err = pathutil.Canonicalize(baseDir)
	if err != nil {
		return Paths{}, fmt.Errorf("canonicalize base dir: %w", err)
	}

	return Paths{
		BaseDir: baseDir,
		Prefix:  prefix,
	}, nil
}
