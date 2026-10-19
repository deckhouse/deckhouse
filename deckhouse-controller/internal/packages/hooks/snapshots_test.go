// Copyright 2026 Flant JSC
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

package hooks_test

import (
	"context"
	"testing"

	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
)

// monitor records whether its snapshot was read: reading a locked monitor's snapshot is the
// hazard (it discards the events it buffers).
type monitor struct{ read bool }

func (m *monitor) CreateInformers() error                      { return nil }
func (m *monitor) Start(context.Context)                       {}
func (m *monitor) Stop()                                       {}
func (m *monitor) Wait()                                       {}
func (m *monitor) EnableKubeEventCb()                          {}
func (m *monitor) GetConfig() *kubeeventsmanager.MonitorConfig { return nil }

func (m *monitor) Snapshot() []kemtypes.ObjectAndFilterResult {
	m.read = true
	return []kemtypes.ObjectAndFilterResult{}
}

func (m *monitor) SnapshotOperations() (*kubeeventsmanager.CachedObjectsInfo, *kubeeventsmanager.CachedObjectsInfo) {
	return &kubeeventsmanager.CachedObjectsInfo{}, &kubeeventsmanager.CachedObjectsInfo{}
}

// monitors is the kube events manager's monitor map; a missing id returns nil, as it does.
type monitors map[string]kubeeventsmanager.Monitor

func (m monitors) GetMonitor(id string) kubeeventsmanager.Monitor { return m[id] }

func binding(name, monitorID string) shtypes.OnKubernetesEventConfig {
	cfg := &kubeeventsmanager.MonitorConfig{}
	cfg.Metadata.MonitorId = monitorID

	return shtypes.OnKubernetesEventConfig{CommonBindingConfig: shtypes.CommonBindingConfig{BindingName: name}, Monitor: cfg}
}

type snapshotsSuite struct {
	suite.Suite
}

func TestSnapshots(t *testing.T) {
	suite.Run(t, new(snapshotsSuite))
}

func (s *snapshotsSuite) TestOnlyUnlockedMonitorsAreRead() {
	locked, open := &monitor{}, &monitor{}
	bindings := []shtypes.OnKubernetesEventConfig{binding("pending", "m1"), binding("synced", "m2")}

	dump := hooks.SnapshotsDump(bindings, monitors{"m1": locked, "m2": open}, hooks.Unlocked{"m2": {}})

	s.False(locked.read, "a locked monitor's snapshot is never read: it would discard its buffered events")
	s.True(open.read)
	s.Contains(dump, "synced")
	s.NotContains(dump, "pending")
}

// A monitor replaced by UpdateMonitor meanwhile (stopped, not yet added) is skipped, not dereferenced.
func (s *snapshotsSuite) TestAMissingMonitorIsSkipped() {
	dump := hooks.SnapshotsDump([]shtypes.OnKubernetesEventConfig{binding("b", "gone")}, monitors{}, hooks.Unlocked{"gone": {}})

	s.Empty(dump)
}

func (s *snapshotsSuite) TestABindingWithoutMonitorIsSkipped() {
	dump := hooks.SnapshotsDump([]shtypes.OnKubernetesEventConfig{{CommonBindingConfig: shtypes.CommonBindingConfig{BindingName: "b"}}},
		monitors{}, hooks.Unlocked{})

	s.Empty(dump)
}
