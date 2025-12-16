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

package entrypoint

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/zchee/git-worktree-runner/internal/cli"
	"github.com/zchee/git-worktree-runner/internal/version"
)

// Main runs the CLI with a signal-aware context and returns the desired process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return Run(ctx, args, stdin, stdout, stderr)
}

// Run runs the CLI using the provided context and returns the desired process exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	runner := cli.Runner{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Version: cli.VersionInfo{
			Version: version.Version,
		},
	}

	return runner.Run(ctx, args)
}
