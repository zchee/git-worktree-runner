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

package adapters

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolveEditor(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name     string
		path     string
		wantCmd  string
		wantArgs []string
		wantDir  string
		wantMode Mode
	}{
		"success: vscode": {
			name:     "vscode",
			path:     "/tmp/x",
			wantCmd:  "code",
			wantArgs: []string{"/tmp/x"},
			wantMode: ModeStart,
		},
		"success: vim uses directory and opens dot": {
			name:     "vim",
			path:     "/tmp/x",
			wantCmd:  "vim",
			wantArgs: []string{"."},
			wantDir:  "/tmp/x",
			wantMode: ModeRun,
		},
		"success: custom command parses args and appends path": {
			name:     "code --wait",
			path:     "/tmp/x",
			wantCmd:  "code",
			wantArgs: []string{"--wait", "/tmp/x"},
			wantMode: ModeRun,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveEditor(tc.name, tc.path)
			if err != nil {
				t.Fatalf("ResolveEditor() error: %v", err)
			}
			if diff := cmp.Diff(tc.wantCmd, got.Command); diff != "" {
				t.Fatalf("command mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantArgs, got.Args); diff != "" {
				t.Fatalf("args mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantDir, got.Dir); diff != "" {
				t.Fatalf("dir mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantMode, got.Mode); diff != "" {
				t.Fatalf("mode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveAIClaudePrefersPATH(t *testing.T) {
	// This test mutates PATH via t.Setenv, so it must not run in parallel.
	tmp := t.TempDir()
	createExecutable(t, tmp, "claude")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))

	spec, err := ResolveAI("claude", "/tmp/repo", []string{"--help"})
	if err != nil {
		t.Fatalf("ResolveAI() error: %v", err)
	}
	if diff := cmp.Diff("claude", spec.Command); diff != "" {
		t.Fatalf("command mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveAICursorTriesCursorCLI(t *testing.T) {
	// This test mutates PATH via t.Setenv, so it must not run in parallel.
	tmp := t.TempDir()
	createExecutable(t, tmp, "cursor")
	t.Setenv("PATH", tmp)

	spec, err := ResolveAI("cursor", "/tmp/repo", []string{"--help"})
	if err != nil {
		t.Fatalf("ResolveAI() error: %v", err)
	}
	if diff := cmp.Diff("cursor", spec.Command); diff != "" {
		t.Fatalf("command mismatch (-want +got):\n%s", diff)
	}
	wantArgs := []string{"cli", "--help"}
	if diff := cmp.Diff(wantArgs, spec.Args); diff != "" {
		t.Fatalf("args mismatch (-want +got):\n%s", diff)
	}
}

func TestExecCursorFallbackToPlainCursor(t *testing.T) {
	// This test mutates PATH via t.Setenv, so it must not run in parallel.
	tmp := t.TempDir()
	createCursorFallbackFixture(t, tmp)
	t.Setenv("PATH", tmp)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode, err := Exec(t.Context(), Spec{
		Name:    "cursor",
		Command: "cursor",
		Args:    []string{"cli", "x"},
		Dir:     t.TempDir(),
		Mode:    ModeRun,
	}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Exec() exitCode=%d, want 0", exitCode)
	}
	if diff := cmp.Diff("x\n", stdout.String()); diff != "" {
		t.Fatalf("stdout mismatch (-want +got):\n%s", diff)
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestProbe(t *testing.T) {
	// This test mutates PATH via t.Setenv, so it must not run in parallel.
	tmp := t.TempDir()
	createExecutable(t, tmp, "cursor")
	createExecutable(t, tmp, "code")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))

	editors, err := Probe(t.Context(), KindEditor)
	if err != nil {
		t.Fatalf("Probe() error: %v", err)
	}

	var cursorStatus string
	var vscodeStatus string
	for _, e := range editors {
		if e.Name == "cursor" {
			cursorStatus = e.Status
		}
		if e.Name == "vscode" {
			vscodeStatus = e.Status
		}
	}
	if cursorStatus != "[ready]" {
		t.Fatalf("expected cursor ready, got %q", cursorStatus)
	}
	if vscodeStatus != "[ready]" {
		t.Fatalf("expected vscode ready, got %q", vscodeStatus)
	}
}

func createExecutable(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)

	var content []byte
	if runtime.GOOS == "windows" {
		// Minimal batch file.
		path += ".bat"
		content = []byte("@echo off\r\nexit /B 0\r\n")
	} else {
		content = []byte("#!/bin/sh\nexit 0\n")
	}

	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func createCursorFallbackFixture(t *testing.T, dir string) {
	t.Helper()

	path := filepath.Join(dir, "cursor")

	var content []byte
	if runtime.GOOS == "windows" {
		path += ".bat"
		content = []byte("@echo off\r\nif \"%1\"==\"cli\" (\r\n  echo cli err 1>&2\r\n  exit /B 42\r\n)\r\necho %1\r\nexit /B 0\r\n")
	} else {
		content = []byte("#!/bin/sh\nif [ \"$1\" = \"cli\" ]; then\n  echo \"cli err\" 1>&2\n  exit 42\nfi\necho \"$1\"\nexit 0\n")
	}

	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
