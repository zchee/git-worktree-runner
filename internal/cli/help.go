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

package cli

import (
	"fmt"
	"io"
)

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `git gtr - Git worktree runner

PHILOSOPHY: Configuration over flags. Set defaults once, then use simple commands.

USAGE:
  git gtr <command> [args...]

CORE COMMANDS:
  new <branch> [options]      Create a new worktree
  rm <id|name>... [options]   Remove worktree(s)
  go <id|name>                Print worktree path for shell navigation
  run <id|name> <cmd...>      Run a command in a worktree
  list [--porcelain]          List worktrees

INTEGRATIONS:
  editor <id|name> [--editor <name>]     Open worktree in editor
  ai <id|name> [--ai <name>] [-- args]   Start AI tool in worktree

SETUP & MAINTENANCE:
  copy <target>... [-- <pattern>...]     Copy files between worktrees
  clean                                 Remove stale/prunable worktrees
  doctor                                Health check
  adapter                               List adapters
  config {get|set|add|unset} <key> ...   Manage configuration
  version                               Show version
  help                                  Show this help
`)
}
