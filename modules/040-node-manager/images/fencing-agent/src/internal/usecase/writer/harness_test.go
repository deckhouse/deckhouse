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

package writer

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

const (
	ms = time.Millisecond
	s  = time.Second
)

var ffnsResource = schema.GroupResource{Group: "node-manager.deckhouse.io", Resource: "fencingfailednodestates"}

func memberlistProfile(probeInterval time.Duration, suspicionMult, maxTimeoutMult int32, wave time.Duration) v1alpha1.FencingSLAProfileSpec {
	var p v1alpha1.FencingSLAProfileSpec
	p.Rejoin.Interval = metav1.Duration{Duration: 5 * s}
	p.Memberlist.ProbeInterval = metav1.Duration{Duration: probeInterval}
	p.Memberlist.SuspicionMult = suspicionMult
	p.Memberlist.SuspicionMaxTimeoutMult = maxTimeoutMult
	p.Eviction.WriterWaveInterval = metav1.Duration{Duration: wave}

	return p
}

var (
	critical = memberlistProfile(200*ms, 3, 4, 2*s)
	normal   = memberlistProfile(500*ms, 4, 6, 3*s)
	slow     = memberlistProfile(1*s, 4, 6, 5*s)
)

// settings is the configuration of a writer of the group "workers" in mode,
// with the critical memberlist profile.
func settings(mode string) config.Settings {
	return config.Settings{
		Config: config.Config{
			MemberlistProfile: "critical",
			NodeGroups: map[string]config.NodeGroup{
				"workers":  {Mode: mode},
				"ng-slow":  {Mode: config.ModeSlow},
				"disabled": {Mode: config.ModeDisabled},
			},
		},
		NodeGroup:  "workers",
		Mode:       mode,
		Memberlist: critical,
	}
}

// decisionProfile is the profile of the decision of the test writers: the
// critical timings of the control plane and of the heartbeat.
func decisionProfile() v1alpha1.FencingSLAProfileSpec {
	var p v1alpha1.FencingSLAProfileSpec
	p.ControlPlaneLostAfter = metav1.Duration{Duration: 4150 * ms}
	p.Heartbeat.Interval = metav1.Duration{Duration: s}
	p.Heartbeat.APITimeout = metav1.Duration{Duration: 2 * s}

	return p
}

type createCall struct {
	at time.Time
	by string
}

// fakeAPI keeps the FFNS objects of one cluster in memory.
type fakeAPI struct {
	mu      sync.Mutex
	objs    map[string]*v1alpha1.FencingFailedNodeState
	rv      int
	creates []createCall
	deletes int
	// onCreate runs before each create. An error fails the create.
	onCreate func(ctx context.Context, ffns *v1alpha1.FencingFailedNodeState) error
	// onGet runs before each get. An error fails the get.
	onGet func(name string) error
	// onDelete runs before each delete with mu held.
	onDelete func(name string)
	// patches are the status writes that succeeded.
	patches []*v1alpha1.FencingFailedNodeState
	// onPatch runs before each status write. An error fails the write.
	onPatch func(ffns *v1alpha1.FencingFailedNodeState) error
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{objs: map[string]*v1alpha1.FencingFailedNodeState{}}
}

func (a *fakeAPI) CreateFFNS(ctx context.Context, ffns *v1alpha1.FencingFailedNodeState) error {
	a.mu.Lock()
	a.creates = append(a.creates, createCall{at: time.Now(), by: ffns.Spec.DetectedBy})
	hook := a.onCreate
	a.mu.Unlock()

	if hook != nil {
		if err := hook(ctx, ffns); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.objs[ffns.Name]; ok {
		return apierrors.NewAlreadyExists(ffnsResource, ffns.Name)
	}

	// As the API server: the new object has no status.
	ffns.Status = v1alpha1.FencingFailedNodeStateStatus{}
	a.put(ffns.DeepCopy())
	ffns.ResourceVersion = a.objs[ffns.Name].ResourceVersion

	return nil
}

// PatchStatus writes the status of ffns if the object still has the
// resourceVersion of old.
func (a *fakeAPI) PatchStatus(_ context.Context, old, ffns *v1alpha1.FencingFailedNodeState) error {
	a.mu.Lock()
	hook := a.onPatch
	a.mu.Unlock()

	if hook != nil {
		if err := hook(ffns); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	cur, ok := a.objs[ffns.Name]
	switch {
	case !ok:
		return apierrors.NewNotFound(ffnsResource, ffns.Name)
	case cur.ResourceVersion != old.ResourceVersion:
		return apierrors.NewConflict(ffnsResource, ffns.Name, fmt.Errorf("resourceVersion %s, want %s", old.ResourceVersion, cur.ResourceVersion))
	}

	next := cur.DeepCopy()
	next.Status = *ffns.Status.DeepCopy()
	a.put(next)
	ffns.ResourceVersion = next.ResourceVersion
	a.patches = append(a.patches, next.DeepCopy())

	return nil
}

func (a *fakeAPI) patched() []*v1alpha1.FencingFailedNodeState {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.patches)
}

// add stores ffns as a new version.
func (a *fakeAPI) add(ffns *v1alpha1.FencingFailedNodeState) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.put(ffns)
}

// put stores ffns as a new version. Call it with mu held.
func (a *fakeAPI) put(ffns *v1alpha1.FencingFailedNodeState) {
	a.rv++
	ffns.ResourceVersion = strconv.Itoa(a.rv)
	a.objs[ffns.Name] = ffns
}

func (a *fakeAPI) GetFFNS(_ context.Context, name string) (*v1alpha1.FencingFailedNodeState, error) {
	a.mu.Lock()
	hook := a.onGet
	a.mu.Unlock()

	if hook != nil {
		if err := hook(name); err != nil {
			return nil, err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	ffns, ok := a.objs[name]
	if !ok {
		return nil, apierrors.NewNotFound(ffnsResource, name)
	}

	return ffns.DeepCopy(), nil
}

func (a *fakeAPI) DeleteFFNS(_ context.Context, name, resourceVersion string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.deletes++
	if a.onDelete != nil {
		a.onDelete(name)
	}

	ffns, ok := a.objs[name]
	switch {
	case !ok:
		return apierrors.NewNotFound(ffnsResource, name)
	case ffns.ResourceVersion != resourceVersion:
		return apierrors.NewConflict(ffnsResource, name, fmt.Errorf("resourceVersion %s, want %s", resourceVersion, ffns.ResourceVersion))
	}

	delete(a.objs, name)

	return nil
}

func (a *fakeAPI) get(name string) (*v1alpha1.FencingFailedNodeState, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ffns, ok := a.objs[name]

	return ffns, ok
}

func (a *fakeAPI) calls() []createCall {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.creates)
}

// cluster runs a writer on each node of names, except the nodes in down. All
// nodes are in the static peer list of each writer.
type cluster struct {
	t       *testing.T
	api     *fakeAPI
	names   []string
	writers map[string]*Writer
	nets    map[string]*fakeNet
	start   time.Time
}

type sent struct {
	from, to string
	msg      domain.Message
}

type fakeNet struct {
	c       *cluster
	self    string
	leaving map[string]bool

	mu        sync.Mutex
	sent      []sent
	recreates int
}

func (n *fakeNet) Peers() []domain.Peer {
	peers := make([]domain.Peer, 0, len(n.c.names))
	for i, name := range n.c.names {
		if name != n.self {
			peers = append(peers, domain.Peer{Name: name, Addr: netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})})
		}
	}

	return peers
}

func (n *fakeNet) Send(to domain.Peer, msg domain.Message) error {
	n.mu.Lock()
	n.sent = append(n.sent, sent{from: n.self, to: to.Name, msg: msg})
	n.mu.Unlock()

	if m, ok := msg.(domain.FFNSCreated); ok {
		if w := n.c.writers[to.Name]; w != nil {
			w.Created(m.Node)
		}
	}

	return nil
}

func (n *fakeNet) Recreate(context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.recreates++
}

func (n *fakeNet) recreated() int {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.recreates
}

func (n *fakeNet) Leaving(node string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.leaving[node]
}

func (n *fakeNet) createdSent(node string) int {
	n.mu.Lock()
	defer n.mu.Unlock()

	count := 0
	for _, m := range n.sent {
		if c, ok := m.msg.(domain.FFNSCreated); ok && c.Node == node {
			count++
		}
	}

	return count
}

type clusterOptions struct {
	settings    config.Settings
	maintenance *atomic.Bool
	// final tells that the stop of the feed of the armed watchdog is final.
	final *atomic.Bool
	down  []string
}

// newCluster starts the writers in the synctest bubble of t. The writers see
// the control plane, have no FFNS and know the NodeMeta of each node with
// metaOf. The test time is then healthyAfter after the start.
func newCluster(t *testing.T, names []string, opts clusterOptions, metaOf func(name string) domain.NodeMeta) *cluster {
	t.Helper()

	if opts.settings.Config.MemberlistProfile == "" {
		opts.settings = settings(config.ModeCritical)
	}

	if opts.maintenance == nil {
		opts.maintenance = &atomic.Bool{}
	}

	if opts.final == nil {
		opts.final = &atomic.Bool{}
	}

	c := &cluster{t: t, api: newFakeAPI(), names: names, writers: map[string]*Writer{}, nets: map[string]*fakeNet{}}

	for _, name := range names {
		if slices.Contains(opts.down, name) {
			continue
		}

		n := &fakeNet{c: c, self: name, leaving: map[string]bool{}}
		w := New(Deps{
			Self:        name,
			API:         c.api,
			Network:     n,
			Settings:    func() config.Settings { return opts.settings },
			Maintenance: opts.maintenance.Load,
			Profile:     decisionProfile,
			Final:       opts.final.Load,
		}, log.NewNop())
		c.writers[name], c.nets[name] = w, n

		go func() { _ = w.Run(t.Context()) }()
	}

	time.Sleep(time.Millisecond)

	for _, w := range c.writers {
		w.ObserveCP(domain.CPState{Visible: true, Seen: true})
		w.SetOwn(nil)

		for _, name := range names {
			meta := metaOf(name)
			w.Member(name, &meta)
		}
	}

	time.Sleep(healthyAfter)
	c.start = time.Now()

	return c
}

// healthyAfter is more than writerHealthyFor of each test cluster.
const healthyAfter = 100 * s

func (c *cluster) die(victim string) {
	for _, w := range c.writers {
		w.Member(victim, nil)
	}
}

func (c *cluster) back(victim string, meta domain.NodeMeta) {
	for _, w := range c.writers {
		w.Member(victim, &meta)
	}
}

// byRank returns the writers of victim from rank 0.
func (c *cluster) byRank(victim string) []string {
	ranked := make([]string, len(c.names)-1)
	for _, name := range c.names {
		if name == victim {
			continue
		}

		ranked[rank(victim, name, (&fakeNet{c: c, self: name}).Peers())] = name
	}

	return ranked
}

func names(prefix string, count int) []string {
	list := make([]string, count)
	for i := range list {
		list[i] = fmt.Sprintf("%s-%d", prefix, i)
	}

	return list
}

func armedMeta(name string) domain.NodeMeta {
	return domain.NodeMeta{BootID: "boot-" + name, Armed: true, NodeUID: "uid-" + name, NodeGroup: "workers", Profile: "critical"}
}

// firstWave returns the nodes of c for which self is in the first wave of writers.
func (c *cluster) firstWave(self string) []string {
	var victims []string

	peers := (&fakeNet{c: c, self: self}).Peers()
	for _, p := range peers {
		if rank(p.Name, self, peers) < waveSize {
			victims = append(victims, p.Name)
		}
	}

	return victims
}
