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

	"github.com/zchee/git-worktree-runner/internal/copy"
)

// CopyOptions configures `git gtr copy`.
type CopyOptions struct {
	From string
	All  bool

	DryRun bool

	Patterns      []string
	PreservePaths bool
}

// CopyResult is a per-target copy outcome.
type CopyResult struct {
	Target      Target
	CopiedFiles []string
}

// Copy copies files from a source target into one or more destination targets.
func (m *Manager) Copy(ctx context.Context, targets []string, opts CopyOptions) ([]CopyResult, error) {
	sourceID := opts.From
	if sourceID == "" {
		sourceID = "1"
	}
	src, err := m.ResolveTarget(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	includes := opts.Patterns
	if len(includes) == 0 {
		cfgIncludes, err := m.cfg.All(ctx, "gtr.copy.include", "copy.include")
		if err != nil {
			return nil, err
		}
		fileIncludes, err := m.cfg.WorktreeIncludePatterns()
		if err != nil {
			return nil, err
		}
		includes = append(cfgIncludes, fileIncludes...)
	}
	if len(includes) == 0 {
		return nil, copy.ErrNoPatterns
	}

	excludes, err := m.cfg.All(ctx, "gtr.copy.exclude", "copy.exclude")
	if err != nil {
		return nil, err
	}

	var destTargets []Target
	if opts.All {
		entries, err := m.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Target.IsMain {
				continue
			}
			if e.Status == WorktreeStatusMissing || e.Status == WorktreeStatusPrunable {
				continue
			}
			if e.Target.Path == src.Path {
				continue
			}
			destTargets = append(destTargets, e.Target)
		}
	} else {
		if len(targets) == 0 {
			return nil, errors.New("no targets specified")
		}
		for _, id := range targets {
			tgt, err := m.ResolveTarget(ctx, id)
			if err != nil {
				return nil, err
			}
			if tgt.Path == src.Path {
				continue
			}
			destTargets = append(destTargets, tgt)
		}
	}

	if opts.PreservePaths == false {
		opts.PreservePaths = false
	} else {
		opts.PreservePaths = true
	}

	var results []CopyResult
	for _, dst := range destTargets {
		res, err := copy.CopyFiles(ctx, src.Path, dst.Path, includes, excludes, copy.Options{
			PreservePaths: opts.PreservePaths,
			DryRun:        opts.DryRun,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, CopyResult{
			Target:      dst,
			CopiedFiles: res.CopiedFiles,
		})
	}

	return results, nil
}
