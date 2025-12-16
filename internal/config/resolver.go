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

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zchee/git-worktree-runner/internal/gitcmd"
)

// Resolver resolves configuration from git config scopes, .gtrconfig, and environment variables.
type Resolver struct {
	Git      gitcmd.Git
	MainRoot string

	// Env overrides os.LookupEnv when non-nil (tests).
	Env map[string]string
}

// New returns a config Resolver for a repository rooted at mainRoot.
func New(g gitcmd.Git, mainRoot string, env map[string]string) Resolver {
	return Resolver{
		Git:      g,
		MainRoot: mainRoot,
		Env:      env,
	}
}

func (r Resolver) gtrconfigPath() string {
	return filepath.Join(r.MainRoot, ".gtrconfig")
}

func (r Resolver) worktreeIncludePath() string {
	return filepath.Join(r.MainRoot, ".worktreeinclude")
}

func (r Resolver) lookupEnv(key string) (string, bool) {
	if r.Env != nil {
		v, ok := r.Env[key]
		return v, ok
	}
	return os.LookupEnv(key)
}

func ignoreMissingKey(err error) error {
	var ee *gitcmd.ExitError
	if errors.As(err, &ee) {
		// `git config --get` exits 1 when the key is missing.
		if ee.ExitCode == 1 {
			return nil
		}
	}
	return err
}

func (r Resolver) get(ctx context.Context, args ...string) (string, error) {
	res, err := r.Git.Run(ctx, r.MainRoot, append([]string{"config"}, args...)...)
	if err != nil {
		return "", ignoreMissingKey(err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (r Resolver) getAll(ctx context.Context, args ...string) ([]string, error) {
	res, err := r.Git.Run(ctx, r.MainRoot, append([]string{"config"}, args...)...)
	if err != nil {
		return nil, ignoreMissingKey(err)
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(res.Stdout, "\n"), "\n"), nil
}

func (r Resolver) getFile(ctx context.Context, file, key string) (string, error) {
	if _, err := os.Stat(file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	res, err := r.Git.Run(ctx, r.MainRoot, "config", "-f", file, "--get", key)
	if err != nil {
		return "", ignoreMissingKey(err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (r Resolver) getAllFile(ctx context.Context, file, key string) ([]string, error) {
	if _, err := os.Stat(file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	res, err := r.Git.Run(ctx, r.MainRoot, "config", "-f", file, "--get-all", key)
	if err != nil {
		return nil, ignoreMissingKey(err)
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(res.Stdout, "\n"), "\n"), nil
}

// Default resolves a single-value key using precedence:
// local git config > .gtrconfig > global git config > system git config > env > fallback.
//
// fileKey is the key name used in .gtrconfig (for example "defaults.editor" for "gtr.editor.default").
func (r Resolver) Default(ctx context.Context, key, envName, fallback, fileKey string) (string, error) {
	v, err := r.get(ctx, "--local", "--get", key)
	if err != nil {
		return "", fmt.Errorf("git config --local --get %s: %w", key, err)
	}
	if v != "" {
		return v, nil
	}

	if fileKey != "" {
		fv, err := r.getFile(ctx, r.gtrconfigPath(), fileKey)
		if err != nil {
			return "", fmt.Errorf("read .gtrconfig %s: %w", fileKey, err)
		}
		if fv != "" {
			return fv, nil
		}
	}

	v, err = r.get(ctx, "--global", "--get", key)
	if err != nil {
		return "", fmt.Errorf("git config --global --get %s: %w", key, err)
	}
	if v != "" {
		return v, nil
	}

	v, err = r.get(ctx, "--system", "--get", key)
	if err != nil {
		return "", fmt.Errorf("git config --system --get %s: %w", key, err)
	}
	if v != "" {
		return v, nil
	}

	if envName != "" {
		if ev, ok := r.lookupEnv(envName); ok && ev != "" {
			return ev, nil
		}
	}

	return fallback, nil
}

// All resolves a multi-valued key and merges values with precedence:
// local git config > .gtrconfig > global git config > system git config.
//
// fileKey is the key name used in .gtrconfig (for example "copy.include" for "gtr.copy.include").
func (r Resolver) All(ctx context.Context, key, fileKey string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string

	appendUnique := func(values []string) {
		for _, v := range values {
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}

	localVals, err := r.getAll(ctx, "--local", "--get-all", key)
	if err != nil {
		return nil, fmt.Errorf("git config --local --get-all %s: %w", key, err)
	}
	appendUnique(localVals)

	if fileKey != "" {
		fileVals, err := r.getAllFile(ctx, r.gtrconfigPath(), fileKey)
		if err != nil {
			return nil, fmt.Errorf("read .gtrconfig %s: %w", fileKey, err)
		}
		appendUnique(fileVals)
	}

	globalVals, err := r.getAll(ctx, "--global", "--get-all", key)
	if err != nil {
		return nil, fmt.Errorf("git config --global --get-all %s: %w", key, err)
	}
	appendUnique(globalVals)

	systemVals, err := r.getAll(ctx, "--system", "--get-all", key)
	if err != nil {
		return nil, fmt.Errorf("git config --system --get-all %s: %w", key, err)
	}
	appendUnique(systemVals)

	return out, nil
}

// WorktreeIncludePatterns reads .worktreeinclude from the repository root and returns non-empty, non-comment lines.
func (r Resolver) WorktreeIncludePatterns() ([]string, error) {
	b, err := os.ReadFile(r.worktreeIncludePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []string
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}

	return out, nil
}

// Set sets a config key in the given scope.
func (r Resolver) Set(ctx context.Context, key, value string, global bool) error {
	args := []string{"config", "--local", key, value}
	if global {
		args = []string{"config", "--global", key, value}
	}
	_, err := r.Git.Run(ctx, r.MainRoot, args...)
	return err
}

// Add adds a value to a multi-valued config key in the given scope.
func (r Resolver) Add(ctx context.Context, key, value string, global bool) error {
	args := []string{"config", "--local", "--add", key, value}
	if global {
		args = []string{"config", "--global", "--add", key, value}
	}
	_, err := r.Git.Run(ctx, r.MainRoot, args...)
	return err
}

// Unset removes all values for a config key in the given scope.
func (r Resolver) Unset(ctx context.Context, key string, global bool) error {
	args := []string{"config", "--local", "--unset-all", key}
	if global {
		args = []string{"config", "--global", "--unset-all", key}
	}
	_, err := r.Git.Run(ctx, r.MainRoot, args...)
	if ignoreMissingKey(err) == nil {
		return nil
	}
	return err
}
