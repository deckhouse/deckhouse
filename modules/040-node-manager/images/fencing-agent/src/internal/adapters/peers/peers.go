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

// Package peers keeps the static peer list of the main network. The source is
// the pods of the agent DaemonSets.
package peers

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const (
	Namespace = "d8-cloud-instance-manager"

	// removeDelay is the time from the loss of the last agent pod of a node
	// to the removal of the node from the list.
	removeDelay = 60 * time.Second
	// leavingHold is the time that the list keeps "leaving" of a node.
	leavingHold = 10 * time.Second

	agentContainer = "fencing-agent"
)

// podSelector selects the pods of the agent 2.0.
var podSelector = labels.SelectorFromSet(labels.Set{
	"app": "fencing-agent",
	"node-manager.deckhouse.io/fencing-agent-version": "2",
}).String()

type entry struct {
	addr netip.Addr
	// pods maps the UID of each agent pod of the node to its image.
	// It is empty for a node that is known only from memberlist.
	pods map[types.UID]string
	// goneAt is the time when the node lost its last pod.
	goneAt time.Time
	// changed is the time of the last change from the informer or from memberlist.
	changed time.Time
}

// List is the static peer list. The key is the node name.
type List struct {
	client   kubernetes.Interface
	nodeName string
	logger   *log.Logger

	refresh atomic.Int64
	wake    chan struct{}

	mu    sync.Mutex
	nodes map[string]*entry
	// left holds the time of "leaving" of each node. Peers does not give
	// the node for leavingHold after it. The events of the node change its
	// entry as usual.
	left     map[string]time.Time
	ownImage string
}

// New returns a List for the agent on nodeName. The safety LIST is off until
// SetRefreshInterval.
func New(client kubernetes.Interface, nodeName string, logger *log.Logger) *List {
	return &List{
		client:   client,
		nodeName: nodeName,
		logger:   logger,
		wake:     make(chan struct{}, 1),
		nodes:    make(map[string]*entry),
		left:     make(map[string]time.Time),
	}
}

// SetRefreshInterval sets the period of the safety LIST. Zero turns it off.
func (l *List) SetRefreshInterval(d time.Duration) {
	if time.Duration(l.refresh.Swap(int64(d))) == d {
		return
	}

	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run watches the agent pods and does the safety LIST. It blocks until ctx ends.
func (l *List) Run(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(l.client, 0,
		informers.WithNamespace(Namespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) { o.LabelSelector = podSelector }),
	)
	defer factory.Shutdown()

	informer := factory.Core().V1().Pods().Informer()
	if err := informer.SetTransform(stripManagedFields); err != nil {
		return fmt.Errorf("set the pod transform: %w", err)
	}

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    l.onPod,
		UpdateFunc: func(_, obj any) { l.onPod(obj) },
		DeleteFunc: l.onPodDelete,
	}); err != nil {
		return fmt.Errorf("add the pod event handler: %w", err)
	}

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil
	}

	l.logger.Info("peer list synced", "peers", len(l.Peers()))

	l.refreshLoop(ctx)

	return nil
}

func (l *List) refreshLoop(ctx context.Context) {
	for {
		var tick <-chan time.Time
		if d := time.Duration(l.refresh.Load()); d > 0 {
			tick = time.After(d)
		}

		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		case <-tick:
			l.relist(ctx)
		}
	}
}

// Peers returns the static peer list without the own node and without the
// nodes in the hold of «leaving», sorted by name.
func (l *List) Peers() []domain.Peer {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	peers := make([]domain.Peer, 0, len(l.nodes))

	for name, at := range l.left {
		if now.Sub(at) >= leavingHold {
			delete(l.left, name)
		}
	}

	for name, e := range l.nodes {
		if e.removed(now) {
			delete(l.nodes, name)
			l.logger.Info("peer removed: its agent pod is gone", "peer", name)

			continue
		}

		if _, ok := l.left[name]; ok {
			continue
		}

		peers = append(peers, domain.Peer{
			Name:       name,
			Addr:       e.addr,
			Compatible: l.compatible(e),
		})
	}

	slices.SortFunc(peers, func(a, b domain.Peer) int { return cmp.Compare(a.Name, b.Name) })

	return peers
}

// Peer finds a node of the list by name. Unlike Peers, it also finds a node
// in the hold of «leaving».
func (l *List) Peer(name string) (domain.Peer, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.nodes[name]
	if e == nil || e.removed(time.Now()) {
		return domain.Peer{}, false
	}

	return domain.Peer{Name: name, Addr: e.addr, Compatible: l.compatible(e)}, true
}

// removed tells whether removeDelay passed after the loss of the last pod.
func (e *entry) removed(now time.Time) bool {
	return !e.goneAt.IsZero() && now.Sub(e.goneAt) >= removeDelay
}

// AddMember adds a node that became a member of memberlist. It does not
// change a node that is already in the list.
func (l *List) AddMember(name string, addr netip.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if name == l.nodeName || !addr.IsValid() || l.nodes[name] != nil {
		return
	}

	l.nodes[name] = &entry{addr: addr, pods: map[types.UID]string{}, changed: time.Now()}
	l.logger.Debug("peer added from memberlist", "peer", name, "address", addr)
}

// Remove takes "leaving" of a node: Peers does not give the node for
// leavingHold. Then the usual rules apply: the node stays while it has an
// agent pod and for removeDelay after the loss of the last one.
func (l *List) Remove(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// The copies of "leaving" do not make the hold longer.
	now := time.Now()
	if at, ok := l.left[name]; ok && now.Sub(at) < leavingHold {
		return
	}

	l.left[name] = now
	l.logger.Info("peer is leaving", "peer", name)
}

// Leaving tells whether the node sent "leaving" less than leavingHold ago.
func (l *List) Leaving(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	at, ok := l.left[name]

	return ok && time.Since(at) < leavingHold
}

// compatible tells whether a pod of the node has the image of the own pod.
func (l *List) compatible(e *entry) bool {
	for _, img := range e.pods {
		if img != "" && img == l.ownImage {
			return true
		}
	}

	return false
}

func (l *List) onPod(obj any) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.upsert(pod, time.Now())
}

func (l *List) onPodDelete(obj any) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.dropPod(pod.Spec.NodeName, pod.UID, time.Now())
}

func (l *List) upsert(pod *corev1.Pod, now time.Time) {
	node := pod.Spec.NodeName

	switch node {
	case "":
		return
	case l.nodeName:
		l.ownImage = image(pod)
		return
	}

	addr, ok := hostIP(pod)
	if !ok {
		// A Pending pod without an address is not a peer yet.
		l.dropPod(node, pod.UID, now)
		return
	}

	e := l.nodes[node]
	if e == nil {
		e = &entry{pods: map[types.UID]string{}}
		l.nodes[node] = e
		l.logger.Debug("peer added", "peer", node, "address", addr)
	}

	e.addr = addr
	e.pods[pod.UID] = image(pod)
	e.goneAt = time.Time{}
	e.changed = now
}

func (l *List) dropPod(node string, uid types.UID, now time.Time) {
	e := l.nodes[node]
	if e == nil {
		return
	}

	if _, ok := e.pods[uid]; !ok {
		return
	}

	delete(e.pods, uid)
	e.changed = now

	if len(e.pods) == 0 {
		e.goneAt = now
	}
}

// relist replaces the list with a full LIST of the agent pods. It does not
// touch a node that changed after the LIST started: that change is newer.
func (l *List) relist(ctx context.Context) {
	start := time.Now()

	list, err := l.client.CoreV1().Pods(Namespace).List(ctx, metav1.ListOptions{
		LabelSelector:   podSelector,
		ResourceVersion: "0",
	})
	if err != nil {
		l.logger.Warn("safety LIST of the agent pods failed", "error", err)
		return
	}

	// fresh applies the rules of the informer to the LIST.
	fresh := New(l.client, l.nodeName, l.logger)
	for i := range list.Items {
		fresh.upsert(&list.Items[i], start)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if fresh.ownImage != "" {
		l.ownImage = fresh.ownImage
	}

	for name, f := range fresh.nodes {
		e := l.nodes[name]
		if e != nil && e.changed.After(start) {
			continue
		}

		f.changed = now
		l.nodes[name] = f
	}

	for name, e := range l.nodes {
		if fresh.nodes[name] == nil && e.goneAt.IsZero() && !e.changed.After(start) {
			e.pods = map[types.UID]string{}
			e.goneAt = now
			e.changed = now
		}
	}
}

func stripManagedFields(obj any) (any, error) {
	if m, ok := obj.(metav1.Object); ok {
		m.SetManagedFields(nil)
	}

	return obj, nil
}

// hostIP returns the IPv4 address of the node of the pod.
func hostIP(pod *corev1.Pod) (netip.Addr, bool) {
	candidates := make([]string, 0, 1+len(pod.Status.HostIPs))
	candidates = append(candidates, pod.Status.HostIP)
	for _, ip := range pod.Status.HostIPs {
		candidates = append(candidates, ip.IP)
	}

	for _, c := range candidates {
		if addr, err := netip.ParseAddr(c); err == nil && addr.Is4() {
			return addr, true
		}
	}

	return netip.Addr{}, false
}

func image(pod *corev1.Pod) string {
	for _, c := range pod.Spec.Containers {
		if c.Name == agentContainer {
			return c.Image
		}
	}

	return ""
}
