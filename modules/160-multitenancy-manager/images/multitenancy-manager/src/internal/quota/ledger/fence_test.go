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

package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// slowLock answers an Update after a delay of the fake clock.
type slowLock struct {
	resourcelock.Interface
	clock *time.Time
	delay time.Duration
	err   error
}

func (l *slowLock) Identity() string { return "me" }

func (l *slowLock) Create(context.Context, resourcelock.LeaderElectionRecord) error {
	*l.clock = l.clock.Add(l.delay)
	return l.err
}

func (l *slowLock) Update(context.Context, resourcelock.LeaderElectionRecord) error {
	*l.clock = l.clock.Add(l.delay)
	return l.err
}

func TestTimingValidate(t *testing.T) {
	require.NoError(t, DefaultTiming.Validate())
	assert.Equal(t, MinFenceWindow, DefaultTiming.Lease-DefaultTiming.Fence-DefaultTiming.WebhookTimeout)

	tooShort := DefaultTiming
	tooShort.Lease = 14 * time.Second
	assert.ErrorContains(t, tooShort.Validate(), "below 9s")

	slowWebhook := DefaultTiming
	slowWebhook.WebhookTimeout = 2 * time.Second
	assert.Error(t, slowWebhook.Validate())
}

func TestTimingFromConfigMap(t *testing.T) {
	got, err := TimingFromConfigMap(&corev1.ConfigMap{Data: map[string]string{"leaseSeconds": "20", "fenceSeconds": "5", "webhookTimeoutSeconds": "1"}})
	require.NoError(t, err)
	assert.Equal(t, 20*time.Second, got.Lease)

	_, err = TimingFromConfigMap(&corev1.ConfigMap{Data: map[string]string{"leaseSeconds": "12"}})
	assert.Error(t, err, "a container does not start with a window below 9 s")
	_, err = TimingFromConfigMap(&corev1.ConfigMap{Data: map[string]string{"fenceSeconds": "x"}})
	assert.Error(t, err)
}

// The fence counts from when the renewal was sent: a renewal answered 6 s late leaves the holder
// 1 s to answer, so a candidate that saw the renewal at its saving still finds 9 s between the
// last approval and the lease.
func TestFenceCountsFromTheSending(t *testing.T) {
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	lock := &FencedLock{Interface: &slowLock{clock: &clock, delay: 6 * time.Second}, Now: func() time.Time { return clock }}
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))

	assert.True(t, lock.Fresh(DefaultTiming.Fence))
	clock = clock.Add(time.Second)
	assert.False(t, lock.Fresh(DefaultTiming.Fence), "7 s after the sending, not after the answer")
}

func TestFenceIgnoresFailuresAndReleases(t *testing.T) {
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	inner := &slowLock{clock: &clock}
	lock := &FencedLock{Interface: inner, Now: func() time.Time { return clock }}
	assert.False(t, lock.Fresh(DefaultTiming.Fence), "never renewed")

	require.NoError(t, lock.Create(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, lock.Fresh(DefaultTiming.Fence))

	clock = clock.Add(3 * time.Second)
	inner.err = errors.New("conflict")
	assert.Error(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	clock = clock.Add(4 * time.Second)
	assert.False(t, lock.Fresh(DefaultTiming.Fence), "a failed renewal does not move the fence")

	inner.err = nil
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))
	assert.True(t, lock.Fresh(DefaultTiming.Fence))
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{}))
	assert.False(t, lock.Fresh(DefaultTiming.Fence), "a release stops the answers")
}

// The wall clock is checked too: after a pause of the machine the monotonic clock may not have moved.
func TestFenceChecksBothClocks(t *testing.T) {
	sent := time.Now()
	clock := sent
	lock := &FencedLock{Interface: &slowLock{clock: &clock}, Now: func() time.Time { return clock }}
	require.NoError(t, lock.Update(context.Background(), resourcelock.LeaderElectionRecord{HolderIdentity: "me"}))

	// the wall clock jumps 10 s while the monotonic reading stays
	wall := sent.Round(0).Add(10 * time.Second)
	clock = wall
	assert.False(t, lock.Fresh(DefaultTiming.Fence))
}
