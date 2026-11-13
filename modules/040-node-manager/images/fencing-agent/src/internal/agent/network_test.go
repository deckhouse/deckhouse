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

package agent

import (
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/peers"
	"fencing-agent/internal/config"
)

// fakeInstances makes an instance in Apply, which waits for release, and drops
// it in Leave. It fails the test when the two run at the same time.
type fakeInstances struct {
	t       *testing.T
	started chan struct{}
	release chan struct{}

	mu       sync.Mutex
	applying bool
	applies  int
	leaves   int
	instance bool
	// recreated are the configurations of Recreate.
	recreated []memberlist.Config
}

func (f *fakeInstances) Apply(memberlist.Config) error {
	f.mu.Lock()
	f.applying = true
	f.applies++
	f.mu.Unlock()

	f.started <- struct{}{}
	<-f.release

	f.mu.Lock()
	defer f.mu.Unlock()

	f.applying, f.instance = false, true

	return nil
}

func (f *fakeInstances) Recreate(cfg memberlist.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.recreated = append(f.recreated, cfg)

	return nil
}

func (f *fakeInstances) Leave() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.applying {
		f.t.Error("Leave runs in parallel with Apply")
	}

	f.leaves++
	f.instance = false
}

// SIGTERM during the creation of memberlist: Leave waits for Apply and leaves
// the instance that Apply made, and a later Apply makes none.
func TestLeaveWaitsForApplyAndStopsLaterApplies(t *testing.T) {
	ml := &fakeInstances{t: t, started: make(chan struct{}), release: make(chan struct{})}
	n := &mainNetwork{peers: peers.New(nil, "worker-1", log.NewNop()), inst: ml}

	applied := make(chan error, 1)

	go func() { applied <- n.Apply(config.Settings{}) }()

	<-ml.started

	left := make(chan struct{})

	go func() { n.Leave(); close(left) }()

	select {
	case <-left:
		t.Fatal("Leave did not wait for Apply")
	case <-time.After(50 * time.Millisecond):
	}

	close(ml.release)

	if err := <-applied; err != nil {
		t.Fatalf("Apply: %v", err)
	}

	<-left

	if err := n.Apply(config.Settings{}); err != nil {
		t.Fatalf("Apply after Leave: %v", err)
	}

	ml.mu.Lock()
	defer ml.mu.Unlock()

	if ml.instance || ml.leaves != 1 || ml.applies != 1 {
		t.Errorf("instance %v, leaves %d, applies %d: want no instance, one Leave and one Apply", ml.instance, ml.leaves, ml.applies)
	}
}

// Recreate uses the configuration of the last Apply. It does nothing before
// the first Apply and after Leave.
func TestRecreateUsesTheLastConfiguration(t *testing.T) {
	ml := &fakeInstances{t: t, started: make(chan struct{}, 1), release: make(chan struct{})}
	close(ml.release)
	n := &mainNetwork{nodeName: "worker-1", peers: peers.New(nil, "worker-1", log.NewNop()), inst: ml, logger: log.NewNop()}

	n.Recreate(t.Context())

	var s config.Settings
	s.Config.ClusterUUID = "uuid-1"

	if err := n.Apply(s); err != nil {
		t.Fatal(err)
	}

	n.Recreate(t.Context())
	n.Leave()
	n.Recreate(t.Context())

	ml.mu.Lock()
	defer ml.mu.Unlock()

	if len(ml.recreated) != 1 || ml.recreated[0].NodeName != "worker-1" || ml.recreated[0].Label != "uuid-1" {
		t.Fatalf("recreations = %+v, want one with the configuration of Apply", ml.recreated)
	}
}
