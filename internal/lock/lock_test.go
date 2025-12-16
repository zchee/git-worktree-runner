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

package lock

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "wr.lock")

	l, err := Acquire(t.Context(), lockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}
}

func TestAcquireContextCanceled(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "wr.lock")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Acquire(ctx, lockPath, 2*time.Second)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
