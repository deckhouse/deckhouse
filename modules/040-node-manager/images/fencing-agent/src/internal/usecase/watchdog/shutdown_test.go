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

package watchdog

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

const healthShutdown = 5 * time.Second

const dueTickAttempts = 32

type shutdownScenario struct {
	h          *harness
	cancel     context.CancelFunc
	runDone    chan struct{}
	runErr     chan error
	healthDone chan struct{}
}

func startShutdownScenario(t *testing.T) *shutdownScenario {
	t.Helper()

	return runShutdownScenario(t, newHarness(t))
}

func runShutdownScenario(t *testing.T, h *harness) *shutdownScenario {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())

	s := &shutdownScenario{
		h:          h,
		cancel:     cancel,
		runDone:    make(chan struct{}),
		runErr:     make(chan error, 1),
		healthDone: make(chan struct{}),
	}

	go func() {
		defer close(s.runDone)

		s.runErr <- h.manager.Run(ctx)
	}()

	go func() {
		defer close(s.healthDone)

		<-ctx.Done()
		time.Sleep(healthShutdown)
	}()

	t.Cleanup(func() {
		cancel()
		<-s.healthDone
	})

	time.Sleep(3 * testFeedInterval)
	synctest.Wait()

	if keepAlives, _, _ := h.device.counters(); keepAlives == 0 || !h.manager.Ready() {
		t.Fatalf("before the shutdown: %d keepalives, ready %t, want an armed and fed watchdog",
			keepAlives, h.manager.Ready())
	}

	return s
}

func (s *shutdownScenario) checkRunReturned(t *testing.T) {
	t.Helper()

	select {
	case <-s.runDone:
		if err := <-s.runErr; err != nil {
			t.Errorf("Run returned an error when its context ended: %v", err)
		}
	default:
		t.Error("Run did not return when its context ended")
	}
}

func TestRunDisarmsAsSoonAsTheContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := startShutdownScenario(t)

		s.cancel()
		synctest.Wait()

		select {
		case <-s.healthDone:
			t.Fatal("the health server stopped before the checks, the disarm order is not pinned")
		default:
		}

		if _, magicCloses, _ := s.h.device.counters(); magicCloses != 1 {
			t.Errorf("magic closes while the health server is still stopping: %d, want exactly one", magicCloses)
		}

		if s.h.manager.Ready() {
			t.Error("Ready() is true while the health server is still stopping, want false after the disarm")
		}

		s.checkRunReturned(t)
	})
}

func TestRunDoesNotReArmAfterTheContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := startShutdownScenario(t)

		s.cancel()
		synctest.Wait()

		opens := s.h.opener.opens()
		keepAlives, _, _ := s.h.device.counters()

		time.Sleep(10 * testFeedInterval)
		synctest.Wait()

		if got := s.h.opener.opens(); got != opens {
			t.Errorf("device opens after the context ended: %d, want %d, the watchdog was armed again", got, opens)
		}

		if got, _, _ := s.h.device.counters(); got != keepAlives {
			t.Errorf("keepalives after the context ended: %d, want %d", got, keepAlives)
		}

		s.checkRunReturned(t)
	})
}

type stallingDevice struct {
	*fakeDevice

	stallNext   atomic.Bool
	stalled     chan struct{}
	unblock     chan struct{}
	releaseOnce sync.Once
}

func (d *stallingDevice) KeepAlive() error {
	err := d.fakeDevice.KeepAlive()

	if d.stallNext.CompareAndSwap(true, false) {
		close(d.stalled)
		<-d.unblock
	}

	return err
}

func (d *stallingDevice) release() {
	d.releaseOnce.Do(func() { close(d.unblock) })
}

func stallKeepAlives(t *testing.T, h *harness) *stallingDevice {
	t.Helper()

	device := &stallingDevice{
		fakeDevice: h.device,
		stalled:    make(chan struct{}),
		unblock:    make(chan struct{}),
	}

	h.manager.deps.Open = func() (Device, error) {
		if _, err := h.opener.open(); err != nil {
			return nil, err
		}

		return device, nil
	}

	t.Cleanup(device.release)

	return device
}

func TestRunDoesNotFeedOnATickDueWhenTheContextEnds(t *testing.T) {
	for attempt := range dueTickAttempts {
		t.Run(fmt.Sprintf("attempt_%02d", attempt), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t)
				device := stallKeepAlives(t, h)
				s := runShutdownScenario(t, h)

				device.stallNext.Store(true)

				select {
				case <-device.stalled:
				case <-time.After(2 * testFeedInterval):
					t.Fatal("no keepalive started within two feed intervals")
				}

				opens := h.opener.opens()
				keepAlives, _, _ := h.device.counters()

				time.Sleep(testFeedInterval + testFeedInterval/2)
				s.cancel()
				device.release()
				synctest.Wait()

				if got, _, _ := h.device.counters(); got != keepAlives {
					t.Errorf("keepalives after the context ended with a tick due: %d, want %d", got, keepAlives)
				}

				if got := h.opener.opens(); got != opens {
					t.Errorf("device opens after the context ended with a tick due: %d, want %d", got, opens)
				}

				if _, magicCloses, _ := h.device.counters(); magicCloses != 1 {
					t.Errorf("magic closes: %d, want exactly one", magicCloses)
				}

				s.checkRunReturned(t)
			})
		})
	}
}
