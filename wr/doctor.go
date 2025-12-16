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

package wr

import (
	"context"
	"io"
	"strings"

	"github.com/zchee/git-worktree-runner/internal/adapters"
	"github.com/zchee/git-worktree-runner/internal/worktrees"
)

// DoctorReport summarizes environment and repository health.
type DoctorReport struct {
	GitVersion string

	MainRoot string

	WorktreesBaseDir string
	WorktreesPrefix  string
	WorktreeCount    int

	Editor      string
	EditorReady bool

	AITool      string
	AIToolReady bool
}

// Doctor inspects environment and repository state.
func (m *Manager) Doctor(ctx context.Context) (DoctorReport, error) {
	var report DoctorReport

	report.MainRoot = m.repoCtx.MainRoot

	if res, err := m.git.Run(ctx, m.repoCtx.MainRoot, "--version"); err == nil {
		report.GitVersion = res.Stdout
	}

	paths, err := worktrees.ResolvePaths(ctx, m.cfg)
	if err != nil {
		return DoctorReport{}, err
	}
	report.WorktreesBaseDir = paths.BaseDir
	report.WorktreesPrefix = paths.Prefix

	entries, err := m.List(ctx)
	if err != nil {
		return DoctorReport{}, err
	}
	for _, e := range entries {
		if e.Target.IsMain {
			continue
		}
		report.WorktreeCount++
	}

	editor, err := m.cfg.Default(ctx, "wr.editor.default", "GTR_EDITOR_DEFAULT", "none", "defaults.editor")
	if err != nil {
		return DoctorReport{}, err
	}
	report.Editor = editor
	if editor == "none" || editor == "" {
		report.EditorReady = true
	} else {
		spec, err := adapters.ResolveEditor(editor, report.MainRoot)
		if err == nil {
			_, err = ensureCommandExists(spec)
		}
		report.EditorReady = err == nil
	}

	ai, err := m.cfg.Default(ctx, "wr.ai.default", "GTR_AI_DEFAULT", "none", "defaults.ai")
	if err != nil {
		return DoctorReport{}, err
	}
	report.AITool = ai
	if ai == "none" || ai == "" {
		report.AIToolReady = false
	} else {
		spec, err := adapters.ResolveAI(ai, report.MainRoot, nil)
		if err == nil {
			_, err = ensureCommandExists(spec)
		}
		report.AIToolReady = err == nil
	}

	return report, nil
}

// WriteDoctorReport renders report to w as human-readable text.
func WriteDoctorReport(w io.Writer, report DoctorReport) {
	writeLine := func(s string) {
		if s == "" {
			return
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		_, _ = io.WriteString(w, s)
	}

	writeLine("Running git wr health check...")
	writeLine("")

	if report.GitVersion != "" {
		writeLine("[OK] Git: " + report.GitVersion)
	}
	writeLine("[OK] Repository: " + report.MainRoot)
	writeLine("[OK] Worktrees directory: " + report.WorktreesBaseDir)
	writeLine("")

	if report.Editor == "none" || report.Editor == "" {
		writeLine("[i] Editor: none configured")
	} else if report.EditorReady {
		writeLine("[OK] Editor: " + report.Editor + " (found)")
	} else {
		writeLine("[!] Editor: " + report.Editor + " (configured but not found)")
	}

	if report.AITool == "none" || report.AITool == "" {
		writeLine("[i] AI tool: none configured")
	} else if report.AIToolReady {
		writeLine("[OK] AI tool: " + report.AITool + " (found)")
	} else {
		writeLine("[!] AI tool: " + report.AITool + " (configured but not found)")
	}
}
