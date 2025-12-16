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
	"fmt"
	"time"

	"github.com/gofrs/flock"
)

// ErrAcquireTimeout is returned when a lock cannot be acquired before the timeout.
var ErrAcquireTimeout = errors.New("lock acquire timeout")

// FileLock is an exclusive file lock.
type FileLock struct {
	f *flock.Flock
}

// Acquire acquires an exclusive lock at path, retrying until timeout or ctx is done.
func Acquire(ctx context.Context, path string, timeout time.Duration) (*FileLock, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	f := flock.New(path)
	ok, err := f.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", path, err)
	}
	if !ok {
		return nil, ErrAcquireTimeout
	}

	return &FileLock{f: f}, nil
}

// Release releases the lock.
func (l *FileLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	return l.f.Unlock()
}
