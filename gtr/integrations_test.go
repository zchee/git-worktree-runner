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
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestOpenEditorCustomCommand(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.editor.default", "/usr/bin/true"); err != nil {
		t.Fatalf("git config gtr.editor.default: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	exitCode, err := m.OpenEditor(t.Context(), "1", "", ExecIO{
		Stdin:  strings.NewReader(""),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("OpenEditor() error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestRunAINoneConfigured(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.ai.default", "none"); err != nil {
		t.Fatalf("git config gtr.ai.default: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	_, err = m.RunAI(t.Context(), "1", "", nil, ExecIO{
		Stdin:  strings.NewReader(""),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrNoAIToolConfigured) {
		t.Fatalf("expected ErrNoAIToolConfigured, got %v", err)
	}
}

func TestRunAICustomCommand(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.ai.default", "/usr/bin/true"); err != nil {
		t.Fatalf("git config gtr.ai.default: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	exitCode, err := m.RunAI(t.Context(), "1", "", []string{"--help"}, ExecIO{
		Stdin:  strings.NewReader(""),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("RunAI() error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}
