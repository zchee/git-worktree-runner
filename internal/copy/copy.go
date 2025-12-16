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

package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
)

// ErrNoPatterns is returned when no include patterns are provided.
var ErrNoPatterns = errors.New("no patterns specified")

// ErrUnsafePattern is returned when a pattern is unsafe (absolute or contains .. path traversal).
var ErrUnsafePattern = errors.New("unsafe pattern")

// Options configures copy behavior.
type Options struct {
	PreservePaths bool
	DryRun        bool
}

// Result describes what was copied.
type Result struct {
	CopiedFiles []string // relative paths from srcRoot
}

// CopyFiles copies files matching include patterns from srcRoot to dstRoot, excluding exclude patterns.
func CopyFiles(ctx context.Context, srcRoot, dstRoot string, includePatterns, excludePatterns []string, opts Options) (Result, error) {
	if len(includePatterns) == 0 {
		return Result{}, ErrNoPatterns
	}
	if opts.PreservePaths == false {
		// ok; flatten mode is supported.
	} else {
		opts.PreservePaths = true
	}

	srcFS := os.DirFS(srcRoot)

	excludes := normalizePatterns(excludePatterns)

	var copied []string
	for _, rawPattern := range includePatterns {
		rawPattern = strings.TrimSpace(rawPattern)
		if rawPattern == "" {
			continue
		}
		if !isSafePattern(rawPattern) {
			return Result{}, fmt.Errorf("%w: %q", ErrUnsafePattern, rawPattern)
		}

		pattern := filepath.ToSlash(rawPattern)
		matches, err := doublestar.Glob(srcFS, pattern)
		if err != nil {
			return Result{}, err
		}

		for _, match := range matches {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			default:
			}

			rel := filepath.ToSlash(strings.TrimPrefix(match, "./"))
			if rel == "" {
				continue
			}
			if excluded(rel, excludes) {
				continue
			}

			info, err := fs.Stat(srcFS, match)
			if err != nil || info.IsDir() {
				continue
			}

			if opts.DryRun {
				copied = appendUnique(copied, rel)
				continue
			}

			srcPath := filepath.Join(srcRoot, filepath.FromSlash(rel))
			var dstPath string
			if opts.PreservePaths {
				dstPath = filepath.Join(dstRoot, filepath.FromSlash(rel))
			} else {
				dstPath = filepath.Join(dstRoot, filepath.Base(filepath.FromSlash(rel)))
			}

			if err := copyFile(srcPath, dstPath); err != nil {
				return Result{}, err
			}
			copied = appendUnique(copied, rel)
		}
	}

	return Result{CopiedFiles: copied}, nil
}

// DirResult describes directory-copy results.
type DirResult struct {
	CopiedDirs []string // relative directory paths from srcRoot
}

// CopyDirectories copies directories whose base name matches any of includeDirPatterns.
//
// includeDirPatterns are matched against the directory base name (like `find -name`), not the full path.
// excludeDirPatterns are matched against the full relative path from srcRoot (with `/` separators).
func CopyDirectories(ctx context.Context, srcRoot, dstRoot string, includeDirPatterns, excludeDirPatterns []string) (DirResult, error) {
	if len(includeDirPatterns) == 0 {
		return DirResult{}, nil
	}

	includes := normalizePatterns(includeDirPatterns)
	excludes := normalizePatterns(excludeDirPatterns)

	for _, p := range includes {
		if !isSafePattern(p) || strings.Contains(p, "/") {
			return DirResult{}, fmt.Errorf("%w: %q", ErrUnsafePattern, p)
		}
	}

	var copiedDirs []string

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !d.IsDir() {
			return nil
		}
		if path == srcRoot {
			return nil
		}

		base := d.Name()
		matched := false
		for _, p := range includes {
			if !isSafePattern(p) {
				return fmt.Errorf("%w: %q", ErrUnsafePattern, p)
			}
			ok, err := filepath.Match(filepath.FromSlash(p), base)
			if err != nil {
				return err
			}
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}

		relDir, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		relDir = filepath.ToSlash(relDir)
		if excluded(relDir, excludes) {
			return fs.SkipDir
		}

		if err := copyDirTree(ctx, srcRoot, dstRoot, relDir, excludes); err != nil {
			return err
		}
		copiedDirs = appendUnique(copiedDirs, relDir)
		return fs.SkipDir
	}

	if err := filepath.WalkDir(srcRoot, walkFn); err != nil {
		return DirResult{}, err
	}

	return DirResult{CopiedDirs: copiedDirs}, nil
}

func copyDirTree(ctx context.Context, srcRoot, dstRoot, relDir string, excludePatterns []string) error {
	srcDir := filepath.Join(srcRoot, filepath.FromSlash(relDir))

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if excluded(rel, excludePatterns) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		if err := copyFile(path, dstPath); err != nil {
			return err
		}
		return nil
	})
}

func copyFile(srcPath, dstPath string) (err error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode()&0o777) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}

func isSafePattern(pattern string) bool {
	if strings.HasPrefix(pattern, "/") {
		return false
	}
	// Normalize slashes for safety checks.
	p := filepath.ToSlash(pattern)
	if strings.HasPrefix(p, "../") || p == ".." || strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") {
		return false
	}
	return true
}

func normalizePatterns(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, filepath.ToSlash(p))
	}
	return out
}

func excluded(path string, excludePatterns []string) bool {
	for _, p := range excludePatterns {
		if !isSafePattern(p) {
			continue
		}
		ok, err := doublestar.Match(p, path)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func appendUnique(dst []string, value string) []string {
	if slices.Contains(dst, value) {
		return dst
	}
	return append(dst, value)
}
