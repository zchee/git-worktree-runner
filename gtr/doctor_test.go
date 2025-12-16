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
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zchee/git-worktree-runner/internal/testutil"
)

func TestDoctorReport(t *testing.T) {
	testutil.SetGitProcessEnv(t)

	toolDir := t.TempDir()
	createTestExecutable(t, toolDir, "myeditor")
	createTestExecutable(t, toolDir, "myai")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	repoDir := filepath.Join(t.TempDir(), "repo")
	g := testutil.Git(t)
	testutil.InitRepo(t, g, repoDir)

	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.editor.default", "myeditor"); err != nil {
		t.Fatalf("git config gtr.editor.default: %v", err)
	}
	if _, err := g.Run(t.Context(), repoDir, "config", "--local", "gtr.ai.default", "myai"); err != nil {
		t.Fatalf("git config gtr.ai.default: %v", err)
	}

	m, err := NewManager(t.Context(), ManagerOptions{StartDir: repoDir})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	report, err := m.Doctor(t.Context())
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}

	if diff := cmp.Diff("myeditor", report.Editor); diff != "" {
		t.Fatalf("editor mismatch (-want +got):\n%s", diff)
	}
	if !report.EditorReady {
		t.Fatalf("expected editor ready, got report=%+v", report)
	}
	if diff := cmp.Diff("myai", report.AITool); diff != "" {
		t.Fatalf("ai mismatch (-want +got):\n%s", diff)
	}
	if !report.AIToolReady {
		t.Fatalf("expected ai tool ready, got report=%+v", report)
	}

	var buf bytes.Buffer
	WriteDoctorReport(&buf, report)
	if buf.Len() == 0 {
		t.Fatalf("expected non-empty output")
	}
}

func createTestExecutable(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += ".bat"
		if err := os.WriteFile(path, []byte("@echo off\r\nexit /B 0\r\n"), 0o755); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
		return
	}

	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
