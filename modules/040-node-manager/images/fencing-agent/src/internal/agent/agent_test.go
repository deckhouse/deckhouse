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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/metrics"
	"fencing-agent/internal/adapters/nodestatus"
	"fencing-agent/internal/adapters/proxyclient"
	"fencing-agent/internal/config"
	"fencing-agent/internal/controllers/localapi"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/decision"
	"fencing-agent/internal/usecase/watchdog"
)

const nodeName = "worker-1"

type journal struct {
	mu      sync.Mutex
	entries []string
}

func (j *journal) add(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = append(j.entries, fmt.Sprintf(format, args...))
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	return slices.Clone(j.entries)
}

func (j *journal) index(entry string) int {
	return slices.Index(j.all(), entry)
}

func (j *journal) count(entry string) int {
	n := 0

	for _, e := range j.all() {
		if e == entry {
			n++
		}
	}

	return n
}

type fakeConfig struct {
	updates chan config.Settings
	err     error
}

func (c *fakeConfig) Run(ctx context.Context) error {
	if c.err != nil {
		return c.err
	}

	<-ctx.Done()

	return nil
}

func (c *fakeConfig) Updates() <-chan config.Settings { return c.updates }

type fakeNetwork struct {
	j       *journal
	peers   []domain.Peer
	members int
	// sendErr is the error of Send to the peer failTo.
	failTo  string
	sendErr error
	metaErr error
	// release, when it is set, blocks Leave until it is closed.
	release chan struct{}
}

func (n *fakeNetwork) Peers() []domain.Peer { return n.peers }

func (n *fakeNetwork) Peer(name string) (domain.Peer, bool) {
	for _, p := range n.peers {
		if p.Name == name {
			return p, true
		}
	}

	return domain.Peer{}, false
}

func (n *fakeNetwork) Send(peer domain.Peer, msg domain.Message) error {
	if _, ok := msg.(domain.Leaving); ok {
		n.j.add("leaving to %s", peer.Name)
	}

	if peer.Name == n.failTo {
		return n.sendErr
	}

	return nil
}

func (*fakeNetwork) Received() <-chan domain.Message { return nil }

func (n *fakeNetwork) Run(ctx context.Context) error {
	<-ctx.Done()

	return nil
}

func (n *fakeNetwork) Start(context.Context, config.Settings) error {
	n.j.add("memberlist created")

	return nil
}

func (n *fakeNetwork) Apply(config.Settings) error { return nil }

func (n *fakeNetwork) Members() int { return n.members }

func (n *fakeNetwork) UpdateMeta(update func(*domain.NodeMeta)) {
	var m domain.NodeMeta
	update(&m)

	if m.Leaving {
		n.j.add("meta leaving")
	}

	if m.NodeGroup != "" || m.Profile != "" {
		n.j.add("meta group %s profile %s", m.NodeGroup, m.Profile)
	}
}

func (n *fakeNetwork) PublishMeta(update func(*domain.NodeMeta)) error {
	if n.metaErr != nil {
		return n.metaErr
	}

	n.UpdateMeta(update)

	return nil
}

func (n *fakeNetwork) Remove(node string) { n.j.add("remove %s", node) }

func (n *fakeNetwork) Leave() {
	n.j.add("leave")

	if n.release != nil {
		<-n.release
	}
}

type fakeStatus struct {
	j *journal
	// legacyErr is the error of RemoveLegacyLabels.
	legacyErr error
	// writeErr is the error of WriteCondition.
	writeErr error
}

func (s fakeStatus) SetCondition(reason domain.Reason, _ string) { s.j.add("condition %s", reason) }
func (fakeStatus) SetCPSource(domain.CPSource)                   {}
func (s fakeStatus) SetProfile(name string)                      { s.j.add("profile %s", name) }
func (fakeStatus) Run(ctx context.Context)                       { <-ctx.Done() }

func (s fakeStatus) Flush(context.Context) error {
	s.j.add("flush")

	return nil
}

func (s fakeStatus) WriteCondition(_ context.Context, reason domain.Reason, _ string) error {
	if s.writeErr != nil {
		return s.writeErr
	}

	s.j.add("condition %s", reason)

	return nil
}

func (s fakeStatus) RemoveLegacyLabels(context.Context) error {
	if s.legacyErr != nil {
		return s.legacyErr
	}

	s.j.add("1.0 labels removed")

	return nil
}

type fakeCP struct {
	updates chan domain.CPState

	mu                                   sync.Mutex
	controlPlaneLostAfter, fastReadAfter time.Duration
}

func (c *fakeCP) Updates() <-chan domain.CPState { return c.updates }

func (c *fakeCP) Run(ctx context.Context) error {
	<-ctx.Done()

	return nil
}

func (c *fakeCP) SetTimings(controlPlaneLostAfter, fastReadAfter time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.controlPlaneLostAfter, c.fastReadAfter = controlPlaneLostAfter, fastReadAfter
}

func (c *fakeCP) fastReadAfterOfCP() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.fastReadAfter
}

func (c *fakeCP) controlPlaneLostAfterOfCP() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.controlPlaneLostAfter
}

type fakeEngine struct {
	feed    atomic.Bool
	decided chan struct{}
	deps    decision.Deps

	mu     sync.Mutex
	params decision.Params
}

func (e *fakeEngine) SetParams(p decision.Params) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.params = p
}

func (e *fakeEngine) paramsOfEngine() decision.Params {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.params
}

func (e *fakeEngine) controlPlaneLostAfterOfEngine() time.Duration {
	return e.paramsOfEngine().ControlPlaneLostAfter
}

func (e *fakeEngine) Run(ctx context.Context) error {
	<-ctx.Done()

	return nil
}

func (e *fakeEngine) ShouldFeed() bool { return e.feed.Load() }

// stop stops the feed as the engine does.
func (e *fakeEngine) stop() {
	e.feed.Store(false)
	e.deps.StopFeed()
}

func (e *fakeEngine) Decided() <-chan struct{} { return e.decided }

type fakeDevice struct {
	j       *journal
	timeout time.Duration
	feeds   atomic.Int32
}

func (*fakeDevice) Identity() string          { return "Software Watchdog" }
func (*fakeDevice) SetTimeoutSupported() bool { return true }
func (*fakeDevice) MagicCloseSupported() bool { return true }

func (d *fakeDevice) KeepAlive() error {
	d.feeds.Add(1)

	return nil
}

func (d *fakeDevice) SetTimeout(timeout time.Duration) error {
	d.timeout = timeout

	return nil
}

func (d *fakeDevice) GetTimeout() (time.Duration, error) { return d.timeout, nil }

func (d *fakeDevice) MagicClose() error {
	d.j.add("magic close")

	return nil
}

type testAgent struct {
	*Agent

	j       *journal
	config  *fakeConfig
	network *fakeNetwork
	engine  *fakeEngine
	cp      *fakeCP
	opens   atomic.Int32
}

func newTestAgent(t *testing.T) *testAgent {
	t.Helper()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: nodeName, UID: "uid-1", Labels: map[string]string{domain.NodeGroupLabel: "worker"},
	}}

	self, err := kubeclient.NewSelfWatcher(fake.NewClientset(node), nodeName, log.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sys/kernel"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "sys/kernel/panic"), []byte("0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ta := &testAgent{
		j:      &journal{},
		config: &fakeConfig{updates: make(chan config.Settings, 1)},
		engine: &fakeEngine{decided: make(chan struct{})},
		cp:     &fakeCP{updates: make(chan domain.CPState, 1)},
	}
	ta.network = &fakeNetwork{
		j:       ta.j,
		peers:   []domain.Peer{{Name: "worker-2", Addr: netip.MustParseAddr("10.0.0.2"), Compatible: true}},
		members: 1,
	}
	ta.Agent = &Agent{
		env:      config.Env{NodeName: nodeName, HealthProbeBindAddress: "127.0.0.1:0"},
		logger:   log.NewNop(),
		self:     self,
		config:   ta.config,
		network:  ta.network,
		newCP:    func(config.Settings) ControlPlane { return ta.cp },
		status:   fakeStatus{j: ta.j},
		events:   fakeEvents{},
		metrics:  nopMetrics{},
		nowayout: func() (bool, error) { return false, nil },
		active:   func() (bool, error) { return false, nil },
		readyz:   func(context.Context) error { return errors.New("not ready") },
		decide: func(d decision.Deps) Engine {
			ta.engine.deps = d

			return ta.engine
		},
		procDir: dir,
		failed:  make(chan error, 1),
		open: func() (watchdog.Device, error) {
			ta.opens.Add(1)
			ta.j.add("watchdog armed")

			return &fakeDevice{j: ta.j}, nil
		},
	}

	ta.metricsHandler = http.NotFoundHandler()

	ta.engine.feed.Store(true)
	close(ta.engine.decided)
	ta.cp.updates <- domain.CPState{Visible: true, Seen: true, ReadAt: time.Now()}

	return ta
}

type fakeEvents struct{}

func (fakeEvents) Normal(string, string)  {}
func (fakeEvents) Warning(string, string) {}

type nopMetrics struct{}

func (nopMetrics) Decision(domain.FeedDecision)                  {}
func (nopMetrics) Poll()                                         {}
func (nopMetrics) PollAnswer(domain.Network, domain.AnswerClass) {}
func (nopMetrics) ConfigInvalid(string, bool)                    {}

func settings(mode string, timeout, feed time.Duration) config.Settings {
	s := config.Settings{NodeGroup: "worker", Mode: mode, ProfileName: "critical"}
	s.Profile.Watchdog = v1alpha1.FencingSLAProfileWatchdog{
		Timeout:      metav1.Duration{Duration: timeout},
		FeedInterval: metav1.Duration{Duration: feed},
	}

	return s
}

// run starts the agent. Call sigterm to cancel the context of Run, and wait
// to get the result of Run.
func (ta *testAgent) run(t *testing.T) (sigterm context.CancelFunc, wait func() error) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	var err error

	go func() {
		defer close(done)

		err = ta.Run(ctx)
	}()

	wait = func() error {
		t.Helper()

		select {
		case <-done:
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not return")

			return nil
		}
	}

	t.Cleanup(func() {
		cancel()
		<-done
	})

	return cancel, wait
}

func eventually(t *testing.T, msg string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartOrder(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	// The first decision comes after the first read of the control plane, so
	// the watchdog can arm at the start step or at a later step. The 1.0 labels
	// and memberlist come first, the condition comes after the profile.
	order := []string{"1.0 labels removed", "memberlist created", "watchdog armed", "condition Armed"}
	if at := ta.j.index("profile critical"); at < 0 || at > ta.j.index("condition Armed") {
		t.Fatalf("journal = %v, want the profile before the condition", ta.j.all())
	}
	last := -1

	for _, entry := range order {
		at := ta.j.index(entry)
		if at <= last {
			t.Fatalf("journal = %v, want the order %v", ta.j.all(), order)
		}

		last = at
	}
}

// The peers get the group and the profile of the node in the NodeMeta: before
// the start of memberlist and after each new configuration.
func TestNodeMetaHasTheGroupAndTheProfile(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	if at := ta.j.index("meta group worker profile critical"); at < 0 || at > ta.j.index("memberlist created") {
		t.Fatalf("journal = %v, want the group and the profile in the NodeMeta before memberlist", ta.j.all())
	}

	next := settings("Normal", 15*time.Second, time.Second)
	next.ProfileName = "normal"
	ta.config.updates <- next

	eventually(t, "the NodeMeta must get the new profile", func() bool {
		return ta.j.index("meta group worker profile normal") >= 0
	})
}

// The first request to the API server fails when the pod network is not ready yet.
// The agent must continue the start and not wait for this request.
func TestStartContinuesAfterAConnectError(t *testing.T) {
	ta := newTestAgent(t)
	ta.status = fakeStatus{j: ta.j, legacyErr: errors.New("dial tcp 10.0.0.1:443: i/o timeout")}
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })
}

func TestPlannedStopOrder(t *testing.T) {
	ta := newTestAgent(t)
	ta.network.peers = append(ta.network.peers, domain.Peer{Name: "master-0", Addr: netip.MustParseAddr("10.0.0.3")})
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)
	ta.stoppers = []func(){func() { ta.j.add("writer stopped") }}

	sigterm, wait := ta.run(t)

	// The watchdog arms in Start, and Run publishes the condition later.
	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	from := len(ta.j.all())

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	got := ta.j.all()[from:]
	// The writer stops first. Magic Close comes only after AgentStopping is on
	// the Node, «leaving» comes after Magic Close.
	want := []string{"writer stopped", "condition AgentStopping", "magic close"}

	for range 3 {
		want = append(want, "leaving to worker-2", "leaving to master-0")
	}

	// leaving in the NodeMeta is published before Leave.
	want = append(want, "meta leaving", "leave", "flush")

	if !slices.Equal(got, want) {
		t.Fatalf("stop = %v, want %v", got, want)
	}
}

// Leave can take many seconds when a peer does not answer, and on a worker
// shutdown kubelet kills the agent 5s after SIGTERM. Thus the agent writes
// AgentStopping to the API before Leave returns.
func TestPlannedStopWritesAgentStoppingBeforeLeave(t *testing.T) {
	ta := newTestAgent(t)

	client := fake.NewClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}})
	ta.status = nodestatus.New(client, nodeName, fakeEvents{}, metrics.New(nodeName), log.NewNop())

	release := make(chan struct{})
	ta.network.release = release
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	sigterm, wait := ta.run(t)

	// This cleanup runs before the cleanup of run, so Run can return.
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)

	eventually(t, "the agent must write Armed", func() bool { return fencingReason(t, client) == domain.ReasonArmed })

	sigterm()

	eventually(t, "the agent must call Leave", func() bool { return ta.j.index("leave") >= 0 })
	eventually(t, "the agent must write AgentStopping before Leave returns", func() bool {
		return fencingReason(t, client) == domain.ReasonAgentStopping
	})

	unblock()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

// AgentStopping is not on the Node: the watchdog stays armed, and the agent
// does not send «leaving». The peers see the node Dead and create its FFNS.
func TestPlannedStopWithoutAgentStopping(t *testing.T) {
	ta := newTestAgent(t)
	ta.status = fakeStatus{j: ta.j, writeErr: errors.New("api is down")}
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	from := len(ta.j.all())

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	if got := ta.j.all()[from:]; !slices.Equal(got, []string{"flush"}) {
		t.Fatalf("stop = %v, want only the flush", got)
	}
}

// The feed of the watchdog is stopped before SIGTERM. That stop is not
// reversible: the agent does not write AgentStopping, does not disarm and
// does not send «leaving».
func TestPlannedStopAfterTheStopOfTheFeed(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	ta.engine.stop()

	if !ta.wd.Stopped() {
		t.Fatal("the watchdog must stop the feed")
	}

	from := len(ta.j.all())

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	if got := ta.j.all()[from:]; !slices.Equal(got, []string{"flush"}) {
		t.Fatalf("stop = %v, want only the flush", got)
	}
}

// An agent without the armed watchdog has nothing to disarm. It sends
// «leaving» also when AgentStopping is not written: else the peers create an
// FFNS for a node that does not stop.
func TestPlannedStopOfAnUnarmedAgentWithoutAgentStopping(t *testing.T) {
	var out bytes.Buffer

	ta := newTestAgent(t)
	ta.logger = log.NewLogger(log.WithOutput(&out), log.WithHandlerType(log.JSONHandlerType))
	ta.status = fakeStatus{j: ta.j, writeErr: errors.New("api is down")}
	ta.config.updates <- settings(config.ModeDisabled, 0, time.Second)

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must publish GroupDisabled", func() bool { return ta.j.index("condition GroupDisabled") >= 0 })

	from := len(ta.j.all())

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	want := []string{"leaving to worker-2", "leaving to worker-2", "leaving to worker-2", "meta leaving", "leave", "flush"}
	if got := ta.j.all()[from:]; !slices.Equal(got, want) {
		t.Fatalf("stop = %v, want %v", got, want)
	}

	if strings.Contains(out.String(), "the watchdog stays armed") {
		t.Fatalf("log of an unarmed agent tells that the watchdog stays armed: %s", out.String())
	}
}

// fencingReason returns the reason of the Fencing condition of the Node.
func fencingReason(t *testing.T, client *fake.Clientset) domain.Reason {
	t.Helper()

	node, err := client.CoreV1().Nodes().Get(t.Context(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range node.Status.Conditions {
		if c.Type == "Fencing" {
			return domain.Reason(c.Reason)
		}
	}

	return ""
}

// The agent logs one info record after it sends «leaving».
func TestPlannedStopLogsLeavingSent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		metaErr error
		want    string
	}{
		{name: "meta published", want: `"meta_published":true`},
		{name: "meta failed", metaErr: errors.New("boom"), want: `"meta_published":false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			ta := newTestAgent(t)
			ta.logger = log.NewLogger(log.WithOutput(&out), log.WithHandlerType(log.JSONHandlerType))
			ta.network.peers = append(ta.network.peers, domain.Peer{Name: "master-0", Addr: netip.MustParseAddr("10.0.0.3")})
			ta.network.failTo = "master-0"
			ta.network.sendErr = errors.New("connection refused")
			ta.network.metaErr = tc.metaErr
			ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

			sigterm, wait := ta.run(t)

			eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

			sigterm()

			if err := wait(); err != nil {
				t.Fatalf("Run returned %v", err)
			}

			var line string

			for l := range strings.Lines(out.String()) {
				if strings.Contains(l, `"msg":"leaving sent"`) {
					line = l
				}
			}

			for _, want := range []string{`"level":"info"`, `"peers":2`, `"copies":3`, `"send_errors":3`, tc.want} {
				if !strings.Contains(line, want) {
					t.Fatalf("missing %s in the record %q, log %s", want, line, out.String())
				}
			}
		})
	}
}

func TestFailureDoesNotDisarm(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)
	ta.config.err = errors.New("config source failed")

	_, wait := ta.run(t)

	if err := wait(); err == nil {
		t.Fatal("Run returned nil after a failure")
	}

	if ta.j.count("magic close") != 0 {
		t.Fatalf("journal = %v, a failure must not disarm", ta.j.all())
	}
}

func TestNoLocalSocketDoesNotStopFencing(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)
	// A unix socket path is limited to about 100 bytes, t.TempDir is longer.
	dir, err := os.MkdirTemp("", "fa")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// The directory of the socket is missing, so the listen fails.
	ta.runners = []func(ctx context.Context) error{
		localapi.New(filepath.Join(dir, "missing", "a.sock"), nodeName, log.NewNop()).Run,
	}

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must arm without the socket", func() bool { return ta.j.index("condition Armed") >= 0 })

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestDisabledGroupTakesPartWithoutTheDevice(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings(config.ModeDisabled, 0, time.Second)

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must publish GroupDisabled", func() bool { return ta.j.index("condition GroupDisabled") >= 0 })

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	if ta.opens.Load() != 0 {
		t.Fatal("the agent of a Disabled group opened the device")
	}

	for _, entry := range []string{"memberlist created", "leaving to worker-2", "condition AgentStopping"} {
		if ta.j.index(entry) < 0 {
			t.Fatalf("journal = %v, want %q", ta.j.all(), entry)
		}
	}
}

func TestArmsWithAPeerOfADisabledGroup(t *testing.T) {
	// The peer list has nodes of all groups; the group of the peer is not known here.
	ta := newTestAgent(t)
	ta.network.peers = []domain.Peer{{Name: "master-0", Addr: netip.MustParseAddr("10.0.0.3"), Compatible: true}}
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must arm", func() bool { return ta.j.index("condition Armed") >= 0 })
}

func TestSingleNodeArmsWhenASecondNodeComes(t *testing.T) {
	ta := newTestAgent(t)
	ta.network.peers = nil
	ta.network.members = 0
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must publish NoEvacuationTarget", func() bool {
		return ta.j.index("condition NoEvacuationTarget") >= 0
	})
}

// messageStatus also keeps the messages of the condition.
type messageStatus struct {
	fakeStatus
	messages *journal
}

func (s messageStatus) SetCondition(reason domain.Reason, message string) {
	s.fakeStatus.SetCondition(reason, message)
	s.messages.add("%s: %s", reason, message)
}

// The proxy reports a ready probe from before the agent start, older than
// controlPlaneLostAfter. The engine gives no decision to feed, so the agent
// does not arm until the control plane is visible by the control plane
// visibility rule.
func TestArmsOnlyAfterTheControlPlaneIsVisibleByTheRuleW(t *testing.T) {
	ta := newTestAgent(t)
	messages := &journal{}
	ta.status = messageStatus{fakeStatus: fakeStatus{j: ta.j}, messages: messages}
	ta.decide = func(d decision.Deps) Engine { return decision.New(d, log.NewNop()) }

	<-ta.cp.updates
	s := settings("Critical", 3*time.Second, 250*time.Millisecond)
	s.Profile.ControlPlaneLostAfter = metav1.Duration{Duration: 4150 * time.Millisecond}
	s.Memberlist = s.Profile
	ta.config.updates <- s

	ta.run(t)

	waiting := "WaitingForControlPlane: no fresh decision to feed the watchdog"
	eventually(t, "the agent must wait for the control plane", func() bool {
		select {
		case ta.cp.updates <- domain.CPState{Seen: true, LastReadyAgo: 5 * time.Second, ReadAt: time.Now()}:
		default:
		}

		return messages.index(waiting) >= 0
	})

	if ta.opens.Load() != 0 {
		t.Fatalf("journal = %v, the agent armed before the control plane was visible", ta.j.all())
	}

	eventually(t, "the agent must arm", func() bool {
		select {
		case ta.cp.updates <- domain.CPState{Seen: true, Visible: true, LastReadyAgo: 100 * time.Millisecond, ReadAt: time.Now()}:
		default:
		}

		return ta.j.index("condition Armed") >= 0
	})
}

func TestRequiresAKubernetesClient(t *testing.T) {
	if _, err := New(config.Env{}, Deps{}, log.NewNop()); err == nil {
		t.Error("New without a Kubernetes client returned nil")
	}
}

// The unarmed agent of an armable group decides with the memberlist profile,
// the armed agent with the profile of its group.
func TestDecisionProfileFollowsArmed(t *testing.T) {
	ta := newTestAgent(t)
	ta.engine.feed.Store(false)

	s := settings("Normal", 15*time.Second, time.Second)
	s.Profile.ControlPlaneLostAfter = metav1.Duration{Duration: 9100 * time.Millisecond}
	s.Memberlist.ControlPlaneLostAfter = metav1.Duration{Duration: 4150 * time.Millisecond}
	ta.config.updates <- s

	ta.run(t)

	eventually(t, "the unarmed agent must use the memberlist profile", func() bool {
		return ta.engine.controlPlaneLostAfterOfEngine() == 4150*time.Millisecond && ta.cp.controlPlaneLostAfterOfCP() == 4150*time.Millisecond
	})

	ta.engine.feed.Store(true)

	eventually(t, "the armed agent must use the profile of its group", func() bool {
		return ta.opens.Load() == 1 && ta.engine.controlPlaneLostAfterOfEngine() == 9100*time.Millisecond && ta.cp.controlPlaneLostAfterOfCP() == 9100*time.Millisecond
	})
}

// The poll timeouts and fastReadAfter come from the common timings of the
// ConfigMap.
func TestCommonTimingsComeFromTheConfigMap(t *testing.T) {
	ta := newTestAgent(t)

	s := settings("Normal", 15*time.Second, time.Second)
	s.Config.CommonTimings = config.CommonTimings{
		RoundTimeout:   metav1.Duration{Duration: 200 * time.Millisecond},
		ConfirmTimeout: metav1.Duration{Duration: 700 * time.Millisecond},
		FastReadAfter:  metav1.Duration{Duration: 1500 * time.Millisecond},
	}
	ta.config.updates <- s

	ta.run(t)

	eventually(t, "the engine and the control plane view must get the common timings", func() bool {
		p := ta.engine.paramsOfEngine()

		return p.RoundTimeout == 200*time.Millisecond && p.ConfirmTimeout == 700*time.Millisecond && ta.cp.fastReadAfterOfCP() == 1500*time.Millisecond
	})
}

// A part of the agent asks to stop it: Run returns the error, so the process
// exits with a non-zero code and the container restarts.
func TestFailStopsRun(t *testing.T) {
	ta := newTestAgent(t)
	ta.config.updates <- settings("Critical", 3*time.Second, 250*time.Millisecond)

	_, wait := ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	stale := errors.New("the watchdog device in the pod is stale")
	ta.fail(stale)
	ta.fail(stale)

	if err := wait(); !errors.Is(err, stale) {
		t.Fatalf("Run = %v, want %v", err, stale)
	}
}

func TestReady(t *testing.T) {
	compatible := []domain.Peer{{Name: "worker-2", Compatible: true}}
	agent10 := []domain.Peer{{Name: "worker-2", Compatible: false}}

	tests := []struct {
		name      string
		reason    domain.Reason
		cpWorking bool
		peers     []domain.Peer
		members   int
		want      bool
	}{
		{"armed with a member", domain.ReasonArmed, true, compatible, 1, true},
		{"the check of the control plane does not work", domain.ReasonArmed, false, compatible, 1, false},
		{"not started", "", true, compatible, 1, false},
		{"timeout mismatch", domain.ReasonWatchdogTimeoutMismatch, true, compatible, 1, false},
		{"waiting for the control plane", domain.ReasonWaitingForControlPlane, true, compatible, 1, false},
		{"invalid key", domain.ReasonInvalidKey, true, compatible, 1, false},
		{"kernel.panic mismatch", domain.ReasonKernelPanicMismatch, true, compatible, 1, true},
		{"no watchdog device", domain.ReasonWatchdogDeviceMissing, true, compatible, 1, true},
		{"maintenance", domain.ReasonMaintenance, true, compatible, 1, true},
		{"single node", domain.ReasonNoEvacuationTarget, true, nil, 0, true},
		{"disabled group", domain.ReasonGroupDisabled, true, compatible, 1, true},
		{"first install: only 1.0 agents", domain.ReasonArmed, true, agent10, 0, true},
		{"lost key on the same image", domain.ReasonArmed, true, compatible, 0, false},
		{"join is not done, a member is known", domain.ReasonArmed, true, compatible, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ready(tt.reason, tt.cpWorking, tt.peers, tt.members); got != tt.want {
				t.Fatalf("ready = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCPViewPassesEachStateAndKeepsTheLast(t *testing.T) {
	src := &fakeCP{updates: make(chan domain.CPState)}
	var observed []uint64

	v := newCPView(func(s domain.CPState) { observed = append(observed, s.ProbeSeq) })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go v.run(ctx, src)

	if v.working() {
		t.Fatal("working before a read")
	}

	pass := func(state domain.CPState) {
		t.Helper()

		src.updates <- state

		if got := <-v.Updates(); got.ProbeSeq != state.ProbeSeq {
			t.Fatalf("got ProbeSeq %d, want %d", got.ProbeSeq, state.ProbeSeq)
		}
	}

	pass(domain.CPState{ProbeSeq: 1, ReadAt: time.Now()})
	pass(domain.CPState{Seen: true, ProbeSeq: 2, ReadAt: time.Now()})

	if !v.working() {
		t.Fatal("want working after the reads")
	}

	pass(domain.CPState{Seen: true, Visible: true, ProbeSeq: 3, ReadAt: time.Now()})

	if !slices.Equal(observed, []uint64{1, 2, 3}) {
		t.Fatalf("observed = %v, want [1 2 3]", observed)
	}

	// A read older than cpStale: the check of the control plane does not work.
	pass(domain.CPState{Seen: true, ProbeSeq: 4, ReadAt: time.Now().Add(-cpStale)})

	if v.working() {
		t.Fatal("want not working after a stale read")
	}
}

// The previous process stopped without Magic Close. On a visible control
// plane the agent disarms the active device before the Settings, without a
// feed, and writes a condition that is not Armed. The SIGTERM does not open
// the inactive device again.
func TestDisarmsTheActiveDeviceBeforeTheSettings(t *testing.T) {
	ta := newTestAgent(t)
	ta.engine.decided = make(chan struct{})
	ta.engine.feed.Store(false)

	device := &fakeDevice{j: ta.j, timeout: 15 * time.Second}
	// Magic Close stops the timer.
	ta.active = func() (bool, error) { return ta.j.count("magic close") == 0, nil }
	ta.open = func() (watchdog.Device, error) {
		ta.opens.Add(1)

		return device, nil
	}
	ta.upstreams = func(context.Context) (proxyclient.Upstreams, error) {
		return proxyclient.Upstreams{HasProbe: true, LastReadyAgo: []time.Duration{2 * time.Second, 300 * time.Millisecond}}, nil
	}

	sigterm, wait := ta.run(t)

	eventually(t, "the agent must disarm the active device", func() bool {
		closed := ta.j.index("magic close")

		return closed >= 0 && ta.j.index("condition WaitingForControlPlane") > closed
	})

	if device.feeds.Load() != 0 || ta.opens.Load() != 1 || ta.j.index("profile critical") >= 0 {
		t.Fatalf("feeds = %d, opens = %d, journal = %v; want one open and no feed before the Settings",
			device.feeds.Load(), ta.opens.Load(), ta.j.all())
	}

	sigterm()

	if err := wait(); err != nil {
		t.Fatalf("Run returned %v", err)
	}

	if ta.j.count("magic close") != 1 || ta.opens.Load() != 1 {
		t.Fatalf("journal = %v, want no second open at the stop", ta.j.all())
	}
}

// The previous process stopped without Magic Close, and the own
// FencingFailedNodeState has no heartbeat after the start. Without the
// heartbeat the engine does not decide, so the agent cannot arm again, and
// Magic Close would leave a live node without the watchdog. The agent keeps
// the device of the previous process active on a visible control plane and
// disarms it only after the heartbeat.
func TestResumeWaitsForTheHeartbeatOfTheOwnFFNS(t *testing.T) {
	ta := newTestAgent(t)
	ta.engine.decided = make(chan struct{})
	ta.engine.feed.Store(false)

	var heartbeat atomic.Bool
	ta.ownReady = heartbeat.Load

	// Magic Close stops the timer.
	ta.active = func() (bool, error) { return ta.j.count("magic close") == 0, nil }
	ta.open = func() (watchdog.Device, error) {
		ta.opens.Add(1)

		return &fakeDevice{j: ta.j, timeout: 15 * time.Second}, nil
	}

	var reads atomic.Int32
	ta.upstreams = func(context.Context) (proxyclient.Upstreams, error) {
		reads.Add(1)

		return proxyclient.Upstreams{HasProbe: true, LastReadyAgo: []time.Duration{300 * time.Millisecond}}, nil
	}

	ta.run(t)

	eventually(t, "the agent must read the control plane", func() bool {
		return reads.Load() >= 5 || ta.j.count("magic close") > 0
	})

	if ta.j.count("magic close") != 0 || ta.opens.Load() != 0 {
		t.Fatalf("opens = %d, journal = %v; want the active device untouched before the heartbeat", ta.opens.Load(), ta.j.all())
	}

	heartbeat.Store(true)

	eventually(t, "the agent must disarm the active device after the heartbeat", func() bool { return ta.j.count("magic close") == 1 })
}

func TestResumeVisible(t *testing.T) {
	ready := func(ago ...time.Duration) proxyclient.Upstreams {
		return proxyclient.Upstreams{HasProbe: true, LastReadyAgo: ago}
	}

	old := proxyclient.Upstreams{LastReadyAgo: []time.Duration{0}}
	refused := errors.New("connection refused")
	// noConnect is the error of a probe that does not connect: the local proxy is stopped.
	noConnect := &url.Error{Op: "Get", URL: "https://127.0.0.1:6445/readyz", Err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}}
	// closed is the error of a probe that connects, and the proxy closes the connection: the proxy of a node that is
	// cut off from the control plane.
	closed := &url.Error{Op: "Get", URL: "https://127.0.0.1:6445/readyz", Err: &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}}
	// notReady is the error of a probe that connects: the proxy or the apiserver answers, but not with 200.
	notReady := errors.New("the server is currently unable to handle the request")

	// agent and cluster are the results of /readyz through the agent client and through the ClusterIP of the
	// service kubernetes. noCluster: the agent client goes to the ClusterIP already, as on a Deckhouse Engine node.
	// probes lists the probes in their order.
	for _, tt := range []struct {
		name      string
		u         proxyclient.Upstreams
		err       error
		delay     time.Duration
		agent     error
		cluster   error
		noCluster bool
		probes    string
		visible   bool
	}{
		{name: "an upstream is ready not more than maxProbeAge ago", u: ready(5*time.Second, 1700*time.Millisecond), visible: true},
		{name: "all upstreams are ready more than maxProbeAge ago", u: ready(1800 * time.Millisecond)},
		{name: "no upstream was ready", u: ready()},
		{name: "an old proxy, the API is ready", u: old, probes: "agent", visible: true},
		{name: "an old proxy, the API is not ready", u: old, agent: notReady, probes: "agent"},
		{name: "the proxy does not answer, the API is ready", err: refused, probes: "agent", visible: true},
		{name: "the proxy answers too late, the API is ready", u: ready(0), delay: time.Second, probes: "agent", visible: true},
		{name: "the proxy is stopped, the ClusterIP is ready", err: refused, agent: noConnect, probes: "agent cluster", visible: true},
		{name: "the proxy is stopped, the ClusterIP does not answer", err: refused, agent: noConnect, cluster: noConnect, probes: "agent cluster"},
		{name: "the proxy answers without the API", err: refused, agent: notReady, probes: "agent"},
		{name: "the proxy closes the connection", err: refused, agent: closed, probes: "agent"},
		{name: "the agent client goes to the ClusterIP", err: refused, agent: noConnect, noCluster: true, probes: "agent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var probes []string

			probe := func(name string, result error) func(ctx context.Context) error {
				return func(ctx context.Context) error {
					probes = append(probes, name)

					// Each probe has its own timeout.
					if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > proxyclient.ProbeTimeout || time.Until(deadline) < proxyclient.ProbeTimeout/2 {
						t.Errorf("the probe %s must have the timeout %s", name, proxyclient.ProbeTimeout)
					}

					return result
				}
			}

			a := &Agent{
				upstreams: func(ctx context.Context) (proxyclient.Upstreams, error) {
					select {
					case <-ctx.Done():
						return proxyclient.Upstreams{}, ctx.Err()
					case <-time.After(tt.delay):
						return tt.u, tt.err
					}
				},
				readyz:        probe("agent", tt.agent),
				readyzCluster: probe("cluster", tt.cluster),
			}

			if tt.noCluster {
				a.readyzCluster = nil
			}

			if got := a.resumeVisible(t.Context()); got != tt.visible {
				t.Fatalf("resumeVisible = %t, want %t", got, tt.visible)
			}

			if got := strings.Join(probes, " "); got != tt.probes {
				t.Fatalf("probes of /readyz = %q, want %q", got, tt.probes)
			}
		})
	}
}

// The probe goes to /readyz of the API address of the client. Only a failed
// connect is a probe that did not connect.
func TestAPIReady(t *testing.T) {
	closed := httptest.NewTLSServer(http.NotFoundHandler())
	closed.Close()

	for _, tt := range []struct {
		name      string
		code      int
		closed    bool
		ready     bool
		connected bool
	}{
		{name: "ready", code: http.StatusOK, ready: true, connected: true},
		{name: "not ready", code: http.StatusInternalServerError, connected: true},
		{name: "no server on the port", closed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var path atomic.Value

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path.Store(r.URL.Path)
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte("ok"))
			}))
			t.Cleanup(srv.Close)

			host := srv.URL
			if tt.closed {
				host = closed.URL
			}

			client, err := kubeclient.New(&rest.Config{Host: host, TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
			if err != nil {
				t.Fatal(err)
			}

			err = apiReady(t.Context(), client)
			if (err == nil) != tt.ready || notConnected(err) == tt.connected {
				t.Fatalf("apiReady = %v, notConnected = %t; want ready %t, connected %t", err, notConnected(err), tt.ready, tt.connected)
			}

			if !tt.closed && path.Load() != "/readyz" {
				t.Fatalf("path = %v, want /readyz", path.Load())
			}
		})
	}
}

// The agent probes the ClusterIP of KUBERNETES_PORT with the TLS settings and
// the credentials of its API client.
func TestClusterReadyz(t *testing.T) {
	var request atomic.Value

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request.Store(r.URL.Path + " " + r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	port := "tcp://" + srv.Listener.Addr().String()
	cfg := &rest.Config{Host: "https://127.0.0.1:6445", BearerToken: "dh-test-secret-0123456789abcdef", TLSClientConfig: rest.TLSClientConfig{Insecure: true}}

	readyz, err := clusterReadyz(cfg, port)
	if err != nil || readyz == nil {
		t.Fatalf("clusterReadyz = %v, probe set: %t; want a probe", err, readyz != nil)
	}

	if err := readyz(t.Context()); err != nil || request.Load() != "/readyz Bearer dh-test-secret-0123456789abcdef" {
		t.Fatalf("probe = %v, request = %v; want /readyz with the token of the agent", err, request.Load())
	}

	if cfg.Host != "https://127.0.0.1:6445" {
		t.Fatalf("the config of the agent client changed: %s", cfg.Host)
	}

	// The agent client goes to the ClusterIP already, as on a Deckhouse Engine node.
	cfg.Host = "https://" + srv.Listener.Addr().String()

	if readyz, err := clusterReadyz(cfg, port); err != nil || readyz != nil {
		t.Fatalf("clusterReadyz = %v, probe set: %t; want no probe", err, readyz != nil)
	}
}

// The ClusterIP comes from KUBERNETES_PORT, which kubelet sets for the service
// kubernetes. The DaemonSet does not change it.
func TestClusterHost(t *testing.T) {
	for _, tt := range []struct {
		port, host, want string
	}{
		{port: "tcp://10.222.0.1:443", host: "https://127.0.0.1:6445", want: "https://10.222.0.1:443"},
		{port: "tcp://[fd00::1]:443", host: "https://127.0.0.1:6445", want: "https://[fd00::1]:443"},
		// A Deckhouse Engine node: the client goes to the ClusterIP already.
		{port: "tcp://10.222.0.1:443", host: "https://10.222.0.1:443"},
		{port: "", host: "https://127.0.0.1:6445"},
		{port: "10.222.0.1:443", host: "https://127.0.0.1:6445"},
		{port: "tcp://10.222.0.1", host: "https://127.0.0.1:6445"},
	} {
		if got := clusterHost(tt.port, tt.host); got != tt.want {
			t.Errorf("clusterHost(%q, %q) = %q, want %q", tt.port, tt.host, got, tt.want)
		}
	}
}

// The profile annotation shows "disabled" for a Disabled group. In the other
// modes it shows the profile of the group, also when the watchdog is not armed.
// The config gives a Disabled group the memberlist profile, here "critical".
// maintenance puts a maintenance annotation on the own Node of the agent.
func maintenance(t *testing.T, ta *testAgent) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: nodeName, UID: "uid-1",
		Labels:      map[string]string{domain.NodeGroupLabel: "worker"},
		Annotations: map[string]string{domain.FencingDisableAnnotation: ""},
	}}

	self, err := kubeclient.NewSelfWatcher(fake.NewClientset(node), nodeName, log.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	ta.self = self
}

// The own Node has a maintenance annotation at the start. The agent does not arm.
func TestMaintenanceAtTheStartDoesNotArm(t *testing.T) {
	ta := newTestAgent(t)
	maintenance(t, ta)
	ta.config.updates <- settings(config.ModeCritical, 3*time.Second, 250*time.Millisecond)

	ta.run(t)

	eventually(t, "the agent must publish Maintenance", func() bool { return ta.j.index("condition Maintenance") >= 0 })

	if ta.opens.Load() != 0 {
		t.Fatalf("journal = %v, want no arm in maintenance", ta.j.all())
	}
}

func TestPublishedProfile(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mode    string
		profile string
		setup   func(*testing.T, *testAgent)
		reason  domain.Reason
		want    string
	}{
		{name: "Disabled", mode: config.ModeDisabled, profile: "critical", reason: domain.ReasonGroupDisabled, want: "disabled"},
		{name: "Normal in maintenance", mode: config.ModeNormal, profile: "normal", setup: maintenance, reason: domain.ReasonMaintenance, want: "normal"},
		{name: "Critical armed", mode: config.ModeCritical, profile: "critical", reason: domain.ReasonArmed, want: "critical"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ta := newTestAgent(t)
			if tt.setup != nil {
				tt.setup(t, ta)
			}

			s := settings(tt.mode, 3*time.Second, 250*time.Millisecond)
			s.ProfileName = tt.profile
			ta.config.updates <- s

			ta.run(t)

			eventually(t, "the agent must publish "+string(tt.reason), func() bool { return ta.j.index("condition "+string(tt.reason)) >= 0 })

			if got := profiles(ta.j.all()); !slices.Equal(got, []string{"profile " + tt.want}) {
				t.Fatalf("published profiles = %v, want [profile %s]", got, tt.want)
			}
		})
	}
}

func TestPublishedProfileFollowsTheMode(t *testing.T) {
	ta := newTestAgent(t)

	s := settings(config.ModeSlow, 30*time.Second, 2*time.Second)
	s.ProfileName = "slow"
	ta.config.updates <- s

	ta.run(t)

	eventually(t, "the agent must publish Armed", func() bool { return ta.j.index("condition Armed") >= 0 })

	ta.config.updates <- settings(config.ModeCritical, 3*time.Second, 250*time.Millisecond)

	eventually(t, "the agent must publish the profile critical", func() bool {
		return slices.Equal(profiles(ta.j.all()), []string{"profile slow", "profile critical"})
	})
}

func profiles(entries []string) []string {
	var out []string

	for _, e := range entries {
		if strings.HasPrefix(e, "profile ") {
			out = append(out, e)
		}
	}

	return out
}

// The gate of the own FencingFailedNodeState changes only the state of the
// decision engine. The observers and the arm get the state of the source.
func TestCPViewGateChangesOnlyTheEngineState(t *testing.T) {
	src := &fakeCP{updates: make(chan domain.CPState)}
	var observed []bool

	v := newCPView(func(s domain.CPState) { observed = append(observed, s.Visible) })
	v.gate = func(s domain.CPState) domain.CPState { s.Visible = false; return s }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go v.run(ctx, src)

	src.updates <- domain.CPState{Visible: true, Seen: true, ReadAt: time.Now()}

	if got := <-v.Updates(); got.Visible {
		t.Fatal("the engine got the visible control plane through the gate")
	}

	if len(observed) != 1 || !observed[0] {
		t.Fatalf("observed %v: want the state of the source", observed)
	}
}
