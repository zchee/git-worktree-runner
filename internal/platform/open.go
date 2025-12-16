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

package platform

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

func openCommand(goos, path string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{path}, nil
	case "linux":
		return "xdg-open", []string{path}, nil
	case "windows":
		// start "" "<path>"
		return "cmd.exe", []string{"/C", "start", "", path}, nil
	default:
		return "", nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

// OpenInGUI opens path in the system file browser (best-effort).
func OpenInGUI(ctx context.Context, path string) error {
	name, args, err := openCommand(runtime.GOOS, path)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, name, args...)

	if err := cmd.Start(); err != nil {
		var ee *exec.Error
		if errors.As(err, &ee) {
			return fmt.Errorf("open gui: %w", err)
		}
		return err
	}

	return nil
}
