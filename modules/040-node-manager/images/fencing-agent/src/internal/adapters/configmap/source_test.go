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

package configmap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/profile"
)

type fakeSelf struct {
	mu      sync.Mutex
	group   string
	started bool
	changed chan struct{}
}

func newFakeSelf() *fakeSelf {
	return &fakeSelf{changed: make(chan struct{}, 1)}
}

func (f *fakeSelf) Changed() <-chan struct{} { return f.changed }

func (f *fakeSelf) Snapshot() (domain.NodeSignals, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return domain.NodeSignals{NodeGroup: f.group}, f.started
}

func (f *fakeSelf) set(group string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.group, f.started = group, true

	select {
	case f.changed <- struct{}{}:
	default:
	}
}

type fakeMetrics struct {
	mu      sync.Mutex
	invalid map[string]bool
}

func (m *fakeMetrics) Decision(domain.FeedDecision)                  {}
func (m *fakeMetrics) Poll()                                         {}
func (m *fakeMetrics) PollAnswer(domain.Network, domain.AnswerClass) {}

func (m *fakeMetrics) ConfigInvalid(parameter string, invalid bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.invalid[parameter] = invalid
}

func (m *fakeMetrics) flagged() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	flagged := map[string]bool{}

	for p, invalid := range m.invalid {
		if invalid {
			flagged[p] = true
		}
	}

	return flagged
}

// fakeProfiles serves the profiles. A profile has the checksum "<name>-<version>".
type fakeProfiles struct {
	mu       sync.Mutex
	versions map[string]int
	specs    map[string]v1alpha1.FencingSLAProfileSpec
	// stale is the number of the next GETs that return the previous version.
	stale int
	gets  int
}

func newFakeProfiles() *fakeProfiles {
	f := &fakeProfiles{versions: map[string]int{}, specs: map[string]v1alpha1.FencingSLAProfileSpec{}}
	for _, name := range []string{"critical", "normal", "slow"} {
		f.versions[name] = 1
		f.specs[name] = specOf(name)
	}

	return f
}

func (f *fakeProfiles) GetSLAProfile(_ context.Context, name string) (*v1alpha1.FencingSLAProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.gets++

	version, ok := f.versions[name]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "fencingslaprofiles"}, name)
	}

	if f.stale > 0 {
		f.stale--
		version--
	}

	return &v1alpha1.FencingSLAProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{profile.ChecksumAnnotation: checksum(name, version)}},
		Spec:       f.specs[name],
	}, nil
}

// publish makes a new version of the profile, as Helm does with a new checksum.
func (f *fakeProfiles) publish(name string, spec v1alpha1.FencingSLAProfileSpec) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.versions[name]++
	f.specs[name] = spec
}

func checksum(name string, version int) string {
	return fmt.Sprintf("%s-%d", name, version)
}

func dur(d time.Duration) metav1.Duration {
	return metav1.Duration{Duration: d}
}

func specOf(name string) v1alpha1.FencingSLAProfileSpec {
	ms := time.Millisecond

	spec := v1alpha1.FencingSLAProfileSpec{
		ControlPlaneLostAfter: dur(4150 * ms),
		Memberlist: v1alpha1.FencingSLAProfileMemberlist{
			ProbeInterval: dur(200 * ms), ProbeTimeout: dur(80 * ms), SuspicionMult: 3, SuspicionMaxTimeoutMult: 4,
			IndirectChecks: 3, AwarenessMaxMultiplier: 8, GossipInterval: dur(200 * ms), GossipNodes: 3, RetransmitMult: 4,
			DeadNodeReclaimTime: dur(time.Second), GossipToTheDeadTime: dur(30 * time.Second),
		},
		Rejoin:    v1alpha1.FencingSLAProfileRejoin{Interval: dur(5 * time.Second)},
		Poll:      v1alpha1.FencingSLAProfilePoll{WitnessFresherBy: dur(2350 * ms)},
		Watchdog:  v1alpha1.FencingSLAProfileWatchdog{Timeout: dur(3 * time.Second), FeedInterval: dur(250 * ms), Margin: dur(300 * ms)},
		Heartbeat: v1alpha1.FencingSLAProfileHeartbeat{Interval: dur(time.Second), APITimeout: dur(2 * time.Second)},
		Eviction:  v1alpha1.FencingSLAProfileEviction{WriterWaveInterval: dur(2 * time.Second)},
	}

	switch name {
	case "normal":
		spec.ControlPlaneLostAfter = dur(9100 * ms)
		spec.Memberlist.ProbeInterval, spec.Memberlist.ProbeTimeout = dur(500*ms), dur(200*ms)
		spec.Memberlist.SuspicionMult, spec.Memberlist.SuspicionMaxTimeoutMult = 4, 6
		spec.Poll.WitnessFresherBy = dur(2350 * ms)
		spec.Watchdog = v1alpha1.FencingSLAProfileWatchdog{Timeout: dur(15 * time.Second), FeedInterval: dur(time.Second), Margin: dur(750 * ms)}
		spec.Heartbeat = v1alpha1.FencingSLAProfileHeartbeat{Interval: dur(2 * time.Second), APITimeout: dur(5 * time.Second)}
		spec.Eviction.WriterWaveInterval = dur(3 * time.Second)
	case "slow":
		spec.ControlPlaneLostAfter = dur(20200 * ms)
		spec.Memberlist.ProbeInterval, spec.Memberlist.ProbeTimeout = dur(time.Second), dur(400*ms)
		spec.Memberlist.SuspicionMult, spec.Memberlist.SuspicionMaxTimeoutMult = 4, 6
		spec.Poll.WitnessFresherBy = dur(2350 * ms)
		spec.Watchdog = v1alpha1.FencingSLAProfileWatchdog{Timeout: dur(30 * time.Second), FeedInterval: dur(2 * time.Second), Margin: dur(1500 * ms)}
		spec.Heartbeat = v1alpha1.FencingSLAProfileHeartbeat{Interval: dur(5 * time.Second), APITimeout: dur(10 * time.Second)}
		spec.Eviction.WriterWaveInterval = dur(5 * time.Second)
	}

	return spec
}

// configJSON renders config.json for the given modes, the memberlist profile
// and the current versions of the profiles.
func configJSON(t *testing.T, profiles *fakeProfiles, memberlist, minFullPollInterval string, modes map[string]string) string {
	t.Helper()

	groups := map[string]map[string]string{}
	for group, mode := range modes {
		groups[group] = map[string]string{"mode": mode}
	}

	sums := map[string]string{}
	for name, version := range profiles.versions {
		sums[name] = checksum(name, version)
	}

	raw, err := json.Marshal(map[string]any{
		"clusterUUID":             "f49dd1c3-a63a-4565-a06c-625e35587eab",
		"nodeGroups":              groups,
		"memberlistProfile":       memberlist,
		"minFullPollInterval":     minFullPollInterval,
		"peerListRefreshInterval": "5m",
		"profileChecksums":        sums,
		"commonTimings":           map[string]string{"roundTimeout": "150ms", "confirmTimeout": "600ms", "fastReadAfter": "1.2s"},
		"maxConcurrentEvictions":  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	return string(raw)
}

func configMap(data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.ConfigMapName, Namespace: config.ConfigMapNamespace},
		Data:       map[string]string{config.ConfigMapKey: data},
	}
}

type harness struct {
	source   *Source
	self     *fakeSelf
	profiles *fakeProfiles
	metrics  *fakeMetrics
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{self: newFakeSelf(), profiles: newFakeProfiles(), metrics: &fakeMetrics{invalid: map[string]bool{}}}

	var err error

	h.source, err = NewSource(fake.NewSimpleClientset(), h.self, h.profiles, h.metrics, log.NewNop())
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	return h
}

// put stores the ConfigMap in the cache of the informer, as the watch does.
func (h *harness) put(t *testing.T, data string) {
	t.Helper()

	if err := h.source.informer.GetStore().Add(configMap(data)); err != nil {
		t.Fatal(err)
	}
}

// reconcile runs one step. It returns the Settings that the step sent, if any.
func (h *harness) reconcile(t *testing.T) (config.Settings, bool) {
	t.Helper()

	s, ok, _ := h.step(t)

	return s, ok
}

// step is reconcile with the delay of the next call.
func (h *harness) step(t *testing.T) (config.Settings, bool, time.Duration) {
	t.Helper()

	done := make(chan time.Duration)

	go func() { done <- h.source.reconcile(t.Context()) }()

	select {
	case s := <-h.source.Updates():
		return s, true, <-done
	case delay := <-done:
		return config.Settings{}, false, delay
	}
}

func TestFirstSettingsHoldTheGroupModeAndProfiles(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "critical", "20s", map[string]string{"worker": "Normal", "master": "Critical"}))

	s, ok := h.reconcile(t)
	if !ok {
		t.Fatal("no Settings")
	}

	if s.NodeGroup != "worker" || s.Mode != config.ModeNormal || s.ProfileName != "normal" {
		t.Errorf("group %q, mode %q, profile %q", s.NodeGroup, s.Mode, s.ProfileName)
	}

	if s.Profile.ControlPlaneLostAfter.Duration != 9100*time.Millisecond || s.Memberlist.ControlPlaneLostAfter.Duration != 4150*time.Millisecond {
		t.Errorf("controlPlaneLostAfter of the profile %s, of memberlist %s", s.Profile.ControlPlaneLostAfter.Duration, s.Memberlist.ControlPlaneLostAfter.Duration)
	}

	if s.Config.MinFullPollInterval.Duration != 20*time.Second || s.Config.ClusterUUID == "" {
		t.Errorf("config %+v", s.Config)
	}

	if flagged := h.metrics.flagged(); len(flagged) != 0 {
		t.Errorf("flagged %v", flagged)
	}
}

// A Disabled agent takes controlPlaneLostAfter, poll.witnessFresherBy,
// repollInterval, freezeLease, heartbeat.interval and heartbeat.apiTimeout
// from the memberlist profile.
func TestDisabledGroupUsesTheMemberlistProfile(t *testing.T) {
	h := newHarness(t)
	h.self.set("system")
	h.put(t, configJSON(t, h.profiles, "critical", "10s", map[string]string{"system": "Disabled", "master": "Critical"}))

	s, ok := h.reconcile(t)
	if !ok {
		t.Fatal("no Settings")
	}

	if s.Mode != config.ModeDisabled || s.ProfileName != "critical" {
		t.Errorf("mode %q, profile %q", s.Mode, s.ProfileName)
	}

	if s.Profile.ControlPlaneLostAfter.Duration != 4150*time.Millisecond || s.Profile.Poll.WitnessFresherBy.Duration != 2350*time.Millisecond ||
		s.Profile.Heartbeat.APITimeout.Duration != 2*time.Second {
		t.Errorf("profile of a Disabled group is not critical: %+v", s.Profile)
	}
}

func TestNoSettingsBeforeTheNodeAndTheConfigMap(t *testing.T) {
	h := newHarness(t)

	// The own Node is not seen, the ConfigMap is absent.
	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings without the own Node")
	}

	h.self.set("worker")

	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings without the ConfigMap")
	}
}

// A change of the mode in the ConfigMap applies without a restart.
func TestModeChangeApplies(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "slow", "10s", map[string]string{"worker": "Slow"}))

	if s, ok := h.reconcile(t); !ok || s.ProfileName != "slow" {
		t.Fatalf("first Settings %v %v", s.ProfileName, ok)
	}

	// The same ConfigMap gives no new Settings.
	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings without a change")
	}

	data := configJSON(t, h.profiles, "critical", "10s", map[string]string{"worker": "Critical"})
	if err := h.source.informer.GetStore().Update(configMap(data)); err != nil {
		t.Fatal(err)
	}

	s, ok := h.reconcile(t)
	if !ok || s.Mode != config.ModeCritical || s.ProfileName != "critical" || s.Memberlist.ControlPlaneLostAfter.Duration != 4150*time.Millisecond {
		t.Fatalf("Settings after the change: %+v, %v", s, ok)
	}
}

func TestGroupChangeApplies(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "10s", map[string]string{"worker": "Normal", "edge": "Slow"}))
	h.reconcile(t)

	h.self.set("edge")

	if s, ok := h.reconcile(t); !ok || s.NodeGroup != "edge" || s.ProfileName != "slow" {
		t.Fatalf("Settings after the group change: %+v, %v", s, ok)
	}
}

// Stale GET answer: the ConfigMap has the new checksum, GET returns the old
// profile. The agent keeps the applied profile and asks again.
func TestStaleGetKeepsTheAppliedProfileAndRetries(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "10s", map[string]string{"worker": "Normal"}))
	h.reconcile(t)

	h.profiles.publish("normal", func() v1alpha1.FencingSLAProfileSpec {
		spec := specOf("normal")
		spec.ControlPlaneLostAfter = dur(10 * time.Second)

		return spec
	}())
	h.profiles.stale = 2

	data := configJSON(t, h.profiles, "normal", "10s", map[string]string{"worker": "Normal"})
	if err := h.source.informer.GetStore().Update(configMap(data)); err != nil {
		t.Fatal(err)
	}

	for i := range 2 {
		_, ok, delay := h.step(t)
		if ok || delay != h.source.retryInterval {
			t.Fatalf("try %d: Settings %v with a stale profile, next call in %s", i, ok, delay)
		}
	}

	s, ok := h.reconcile(t)
	if !ok || s.Profile.ControlPlaneLostAfter.Duration != 10*time.Second {
		t.Fatalf("Settings after the answer is fresh: %+v, %v", s.Profile.ControlPlaneLostAfter, ok)
	}
}

// Invalid value in the ConfigMap: minFullPollInterval = 2 s. The agent keeps the previous
// minFullPollInterval and flags the parameter. A valid value clears the flag.
func TestInvalidMinFullPollIntervalKeepsThePreviousValue(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	modes := map[string]string{"worker": "Normal"}
	h.put(t, configJSON(t, h.profiles, "normal", "20s", modes))
	h.reconcile(t)

	if err := h.source.informer.GetStore().Update(configMap(configJSON(t, h.profiles, "normal", "2s", modes))); err != nil {
		t.Fatal(err)
	}

	if s, ok := h.reconcile(t); ok {
		t.Fatalf("new Settings for a value that does not change the applied ones: %+v", s.Config)
	}

	if flagged := h.metrics.flagged(); len(flagged) != 1 || !flagged[config.ParamMinFullPollInterval] {
		t.Errorf("flagged %v, want only minFullPollInterval", flagged)
	}

	// A valid value is applied and the flag goes.
	if err := h.source.informer.GetStore().Update(configMap(configJSON(t, h.profiles, "normal", "30s", modes))); err != nil {
		t.Fatal(err)
	}

	s, ok := h.reconcile(t)
	if !ok || s.Config.MinFullPollInterval.Duration != 30*time.Second {
		t.Fatalf("Settings %+v, %v", s.Config.MinFullPollInterval, ok)
	}

	if flagged := h.metrics.flagged(); len(flagged) != 0 {
		t.Errorf("flagged %v, want none", flagged)
	}
}

// An invalid value is not a reason to stop the other values: they apply, the
// invalid one takes the previous value.
func TestInvalidMinFullPollIntervalDoesNotBlockOtherChanges(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"}))
	h.reconcile(t)

	data := configJSON(t, h.profiles, "normal", "61s", map[string]string{"worker": "Slow"})
	if err := h.source.informer.GetStore().Update(configMap(data)); err != nil {
		t.Fatal(err)
	}

	s, ok := h.reconcile(t)
	if !ok || s.Mode != config.ModeSlow || s.Config.MinFullPollInterval.Duration != 20*time.Second {
		t.Fatalf("Settings %q, minFullPollInterval %s, %v", s.Mode, s.Config.MinFullPollInterval.Duration, ok)
	}
}

func TestInvalidMinFullPollIntervalBeforeTheFirstValueUsesTheDefault(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "1s", map[string]string{"worker": "Normal"}))

	s, ok := h.reconcile(t)
	if !ok || s.Config.MinFullPollInterval.Duration != config.DefaultMinFullPollInterval {
		t.Fatalf("minFullPollInterval %s, %v", s.Config.MinFullPollInterval.Duration, ok)
	}

	if !h.metrics.flagged()[config.ParamMinFullPollInterval] {
		t.Error("minFullPollInterval is not flagged")
	}
}

func TestBrokenConfigKeepsThePreviousSettings(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"}))
	h.reconcile(t)

	if err := h.source.informer.GetStore().Update(configMap(`{"minFullPollInterval": "soon"`)); err != nil {
		t.Fatal(err)
	}

	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings from a broken config.json")
	}

	if !h.metrics.flagged()[config.ParamConfig] {
		t.Errorf("flagged %v", h.metrics.flagged())
	}
}

// Helm can delete the ConfigMap before the agent gets SIGTERM. The agent keeps
// the applied Settings and does not apply them again when the ConfigMap comes back.
func TestDeletedConfigMapKeepsTheSettings(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	data := configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"})
	h.put(t, data)

	if _, ok := h.reconcile(t); !ok {
		t.Fatal("no Settings")
	}

	if err := h.source.informer.GetStore().Delete(configMap(data)); err != nil {
		t.Fatal(err)
	}

	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings without the ConfigMap")
	}

	h.put(t, data)

	if s, ok := h.reconcile(t); ok {
		t.Fatalf("Settings after the same ConfigMap came back: %+v", s)
	}
}

func TestInvalidModeKeepsThePreviousSettings(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"}))
	h.reconcile(t)

	if err := h.source.informer.GetStore().Update(configMap(configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Watchdog"}))); err != nil {
		t.Fatal(err)
	}

	if _, ok := h.reconcile(t); ok {
		t.Fatal("Settings for an unknown mode")
	}

	if !h.metrics.flagged()[config.ParamMode] {
		t.Errorf("flagged %v", h.metrics.flagged())
	}
}

// A profile that fails the inequalities is not applied, and the agent does not
// ask for it again until the ConfigMap changes.
func TestInvalidProfileIsNotApplied(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"}))
	h.reconcile(t)

	broken := specOf("normal")
	broken.Poll.WitnessFresherBy = dur(8 * time.Second)
	h.profiles.publish("normal", broken)

	data := configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"})
	if err := h.source.informer.GetStore().Update(configMap(data)); err != nil {
		t.Fatal(err)
	}

	if _, ok := h.reconcile(t); ok {
		t.Fatal("an invalid profile is applied")
	}

	if !h.metrics.flagged()[config.ParamProfile] {
		t.Errorf("flagged %v", h.metrics.flagged())
	}

	gets := h.profiles.gets

	if d := h.source.reconcile(t.Context()); d != 0 || h.profiles.gets != gets {
		t.Errorf("the agent asks for an invalid profile again: next call in %s, %d more GETs", d, h.profiles.gets-gets)
	}
}

func TestProfileNotFoundRetries(t *testing.T) {
	h := newHarness(t)
	h.self.set("worker")
	h.put(t, configJSON(t, h.profiles, "normal", "20s", map[string]string{"worker": "Normal"}))
	delete(h.profiles.versions, "normal")

	if _, _, delay := h.step(t); delay != h.source.retryInterval {
		t.Errorf("next call in %s, want %s", delay, h.source.retryInterval)
	}
}

// Run: the Source reads the ConfigMap by name with an informer, applies a change
// of the mode without a restart, and watches nothing but the ConfigMap.
func TestRunAppliesTheConfigMapFromTheInformer(t *testing.T) {
	profiles := newFakeProfiles()
	modes := map[string]string{"worker": "Normal"}
	other := configMap("{}")
	other.Name = "other"

	client := fake.NewSimpleClientset(configMap(configJSON(t, profiles, "normal", "20s", modes)), other)

	self := newFakeSelf()
	self.set("worker")

	source, err := NewSource(client, self, profiles, &fakeMetrics{invalid: map[string]bool{}}, log.NewNop())
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	source.retryInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() { _ = source.Run(ctx) }()

	next := func() config.Settings {
		t.Helper()

		select {
		case s := <-source.Updates():
			return s
		case <-time.After(5 * time.Second):
			t.Fatal("no Settings")

			return config.Settings{}
		}
	}

	if s := next(); s.Mode != config.ModeNormal {
		t.Fatalf("first mode %q", s.Mode)
	}

	updated := configMap(configJSON(t, profiles, "critical", "20s", map[string]string{"worker": "Critical"}))
	if _, err := client.CoreV1().ConfigMaps(config.ConfigMapNamespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	if s := next(); s.Mode != config.ModeCritical || s.ProfileName != "critical" {
		t.Fatalf("mode after the change %q, profile %q", s.Mode, s.ProfileName)
	}

	// Every request of the informer is for the ConfigMap with its name.
	var lists, watches int

	for _, action := range client.Actions() {
		if action.GetResource().Resource != "configmaps" {
			t.Errorf("request for %s", action.GetResource().Resource)
		}

		var selector string

		switch a := action.(type) {
		case ktesting.ListActionImpl:
			selector = a.ListRestrictions.Fields.String()
			lists++
		case ktesting.WatchActionImpl:
			selector = a.WatchRestrictions.Fields.String()
			watches++
		default:
			continue
		}

		if selector != "metadata.name="+config.ConfigMapName {
			t.Errorf("%s of configmaps with the field selector %q", action.GetVerb(), selector)
		}
	}

	if lists == 0 || watches == 0 {
		t.Errorf("the informer made %d LIST and %d WATCH requests", lists, watches)
	}
}

func TestSettingsCarryTheEngineFlagOfTheGroup(t *testing.T) {
	h := newHarness(t)
	h.self.set("engine")
	data := configJSON(t, h.profiles, "normal", "10s", map[string]string{"engine": "Normal", "worker": "Normal"})
	data = strings.Replace(data, `"engine":{"mode":"Normal"}`, `"engine":{"mode":"Normal","engine":true}`, 1)
	h.put(t, data)

	if s, ok := h.reconcile(t); !ok || !s.Engine {
		t.Fatalf("Settings of an Engine group: %+v, %v", s, ok)
	}

	h.self.set("worker")

	if s, ok := h.reconcile(t); !ok || s.Engine {
		t.Fatalf("Settings after the move to a group without Engine: %+v, %v", s, ok)
	}
}

// Run does not poll the node group: a change that SelfWatcher signals must be enough.
func TestRunAppliesTheGroupChangeSignaledByTheNode(t *testing.T) {
	profiles := newFakeProfiles()
	data := configJSON(t, profiles, "normal", "10s", map[string]string{"worker": "Normal", "edge": "Slow"})

	self := newFakeSelf()
	self.set("worker")

	source, err := NewSource(fake.NewSimpleClientset(configMap(data)), self, profiles, &fakeMetrics{invalid: map[string]bool{}}, log.NewNop())
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() { _ = source.Run(ctx) }()

	next := func() config.Settings {
		t.Helper()

		select {
		case s := <-source.Updates():
			return s
		case <-time.After(5 * time.Second):
			t.Fatal("no Settings")

			return config.Settings{}
		}
	}

	if s := next(); s.NodeGroup != "worker" {
		t.Fatalf("first group %q", s.NodeGroup)
	}

	self.set("edge")

	if s := next(); s.NodeGroup != "edge" || s.Mode != config.ModeSlow {
		t.Fatalf("group %q, mode %q after the change", s.NodeGroup, s.Mode)
	}
}
