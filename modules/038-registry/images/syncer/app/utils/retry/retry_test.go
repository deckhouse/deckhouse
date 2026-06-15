/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errTest = errors.New("test error")

func TestDo(t *testing.T) {
	tests := []struct {
		name   string
		retry  *Retry
		testFn func(t *testing.T, r *Retry)
	}{
		{
			name:  "success on first attempt",
			retry: Default(),
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				err := r.Do(context.Background(), func() error { calls++; return nil })
				assertNoErr(t, err)
				assertCalls(t, calls, 1)
			},
		},
		{
			name:  "success on second attempt",
			retry: &Retry{Attempts: 3, Interval: time.Millisecond},
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				err := r.Do(context.Background(), func() error {
					calls++
					if calls < 2 {
						return errTest
					}
					return nil
				})
				assertNoErr(t, err)
				assertCalls(t, calls, 2)
			},
		},
		{
			name:  "all attempts fail",
			retry: &Retry{Attempts: 3, Interval: time.Millisecond},
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				err := r.Do(context.Background(), func() error { calls++; return errTest })
				assertErr(t, err, errTest)
				assertCalls(t, calls, 3)
			},
		},
		{
			name:  "nil retry returns error",
			retry: nil,
			testFn: func(t *testing.T, r *Retry) {
				err := r.Do(context.Background(), func() error { return nil })
				assertErr(t, err, nil)
			},
		},
		{
			name:  "break stops retries after first failure",
			retry: &Retry{Attempts: 5, Interval: time.Millisecond},
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				r.WithBreak(func(lastErr error) bool { return errors.Is(lastErr, errTest) })
				err := r.Do(context.Background(), func() error { calls++; return errTest })
				assertErr(t, err, nil)
				assertCalls(t, calls, 1)
			},
		},
		{
			name:  "before called between retries",
			retry: &Retry{Attempts: 3, Interval: time.Millisecond},
			testFn: func(t *testing.T, r *Retry) {
				calls, beforeCalls := 0, 0
				r.WithBefore(func(_ time.Duration, _, _ uint, _ error) { beforeCalls++ })
				_ = r.Do(context.Background(), func() error { calls++; return errTest })
				assertCalls(t, calls, 3)
				if beforeCalls != 2 {
					t.Fatalf("expected 2 before calls, got %d", beforeCalls)
				}
			},
		},
		{
			name:  "context cancelled during wait",
			retry: &Retry{Attempts: 3, Interval: time.Second},
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- r.Do(ctx, func() error { calls++; return errTest }) }()
				time.Sleep(10 * time.Millisecond)
				cancel()
				assertErr(t, <-done, nil)
				assertCalls(t, calls, 1)
			},
		},
		{
			name:  "zero attempts fn never called",
			retry: &Retry{Attempts: 0, Interval: time.Millisecond},
			testFn: func(t *testing.T, r *Retry) {
				calls := 0
				_ = r.Do(context.Background(), func() error { calls++; return nil })
				assertCalls(t, calls, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFn(t, tt.retry)
		})
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func assertErr(t *testing.T, err error, wrapped error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if wrapped != nil && !errors.Is(err, wrapped) {
		t.Fatalf("expected %v in error chain, got %v", wrapped, err)
	}
}

func assertCalls(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %d calls, got %d", want, got)
	}
}
