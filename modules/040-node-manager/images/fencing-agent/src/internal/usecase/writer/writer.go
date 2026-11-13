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

// Package writer creates the FencingFailedNodeState (FFNS) of a peer that the
// main network declares dead, and deletes it when the peer comes back with the
// same boot ID. It writes the attestation of a witness and the heartbeat in
// the FFNS of the own node.
package writer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

const (
	// waveSize is the number of writers in one wave.
	waveSize = 3
	// cancelers is the number of the first writers by HRW that delete the FFNS
	// when the peer comes back.
	cancelers = 3
	// attemptTimeout is the timeout of one API attempt.
	attemptTimeout = time.Second
)

// pauses are the pauses between the API attempts: there are len(pauses)+1 attempts.
var pauses = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}

// errNotAllowed stops the attempts: the writer may not write the FFNS now. It
// stops the create, the attestation and the delete when the peer came back.
var errNotAllowed = errors.New("the writer may not write the FFNS now")

// errUnhealthy is errNotAllowed by the health gate of the own node. The
// create of the FFNS of a dead peer waits for the gate.
var errUnhealthy = fmt.Errorf("%w: the own node is not healthy", errNotAllowed)

// API reads and writes the objects of the cluster.
type API interface {
	CreateFFNS(ctx context.Context, ffns *v1alpha1.FencingFailedNodeState) error
	GetFFNS(ctx context.Context, name string) (*v1alpha1.FencingFailedNodeState, error)
	// DeleteFFNS deletes the FFNS only if its resourceVersion did not change.
	DeleteFFNS(ctx context.Context, name, resourceVersion string) error
	// PatchStatus writes the changes of the status from old to ffns only if
	// the resourceVersion of old did not change.
	PatchStatus(ctx context.Context, old, ffns *v1alpha1.FencingFailedNodeState) error
}

// Network is the main network.
type Network interface {
	// Peers returns the static peer list without the own node.
	Peers() []domain.Peer
	Send(to domain.Peer, msg domain.Message) error
	// Leaving tells whether the node sent «leaving» less than 10 s ago.
	Leaving(node string) bool
	// Recreate creates the memberlist instance again and joins the peers.
	Recreate(ctx context.Context)
}

type Deps struct {
	// Self is the name of the own node.
	Self    string
	API     API
	Network Network
	// Settings returns the applied configuration. It is empty before the first one.
	Settings func() config.Settings
	// Maintenance tells whether the own node is in maintenance.
	Maintenance func() bool
	// Profile returns the profile of the decision: the profile of the node
	// group for an armed agent, else the memberlist profile.
	Profile func() v1alpha1.FencingSLAProfileSpec
	// Final tells whether the stop of the feed of the armed watchdog is
	// final. Then the agent writes no heartbeat.
	Final func() bool
}

// peer is a member of the main network.
type peer struct {
	// meta is the last NodeMeta. While the peer is dead, it is the NodeMeta at the death.
	meta   domain.NodeMeta
	dead   bool
	deadAt time.Time
	// deaths counts the deaths. The work for an older death stops.
	deaths uint64
	// leaving is true when «leaving» of the peer was in the hold at the death.
	leaving bool
	// created is true when the FFNS of this death exists: by the own create or by «created».
	created bool
	// wrote is true when the own create of this death succeeded or timed
	// out: the server can store the object of a create that timed out.
	wrote bool
}

// Writer is the writer of FFNS in the agent.
type Writer struct {
	deps   Deps
	logger *log.Logger

	wg sync.WaitGroup

	mu sync.Mutex
	// ctx is set while Run runs.
	ctx context.Context
	// stopped is set by Stop.
	stopped bool
	peers   map[string]*peer
	// cpSince is the start of the current period with the control plane
	// visible. It is zero while the control plane is not visible.
	cpSince time.Time
	// own is true while the FFNS of the own node exists or is not known yet.
	own bool
	// ownGoneAt is the time when the FFNS of the own node was gone.
	ownGoneAt time.Time
	// ownFFNS is the last version of the FFNS of the own node. It is nil
	// while the FFNS does not exist or is not known yet.
	ownFFNS *v1alpha1.FencingFailedNodeState
	// ownKnown is true after the first report of the FFNS of the own node.
	ownKnown bool
	// ownSince is the time when the FFNS of the own node appeared. It is
	// zero when the FFNS existed at the start of the agent.
	ownSince time.Time
	// heartbeatAt is the send time of the last successful heartbeat in the
	// current FFNS of the own node.
	heartbeatAt time.Time
	// recreated is true after the memberlist recreation for the current FFNS
	// of the own node.
	recreated bool
	// wake starts a heartbeat at once.
	wake chan struct{}
}

func New(deps Deps, logger *log.Logger) *Writer {
	return &Writer{deps: deps, logger: logger, peers: make(map[string]*peer), own: true, wake: make(chan struct{}, 1)}
}

// Run blocks until ctx ends. The writer creates and deletes FFNS only while Run runs.
func (w *Writer) Run(ctx context.Context) error {
	w.mu.Lock()
	w.ctx = ctx
	w.hygiene()
	w.mu.Unlock()

	w.wg.Go(func() { w.heartbeats(ctx) })

	<-ctx.Done()

	w.mu.Lock()
	w.ctx = nil
	w.mu.Unlock()

	w.wg.Wait()

	return nil
}

// Stop is the start of the planned stop of the agent. After it the writer
// creates no FFNS.
func (w *Writer) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.stopped = true
}

// ObserveCP takes each control plane state.
func (w *Writer) ObserveCP(cp domain.CPState) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch {
	case !cp.Visible:
		w.cpSince = time.Time{}
	case w.cpSince.IsZero():
		w.cpSince = time.Now()
		// The heartbeat waits for the visible control plane.
		if w.ownFFNS != nil {
			w.wakeHeartbeat()
		}
	}

	w.hygiene()
}

// Created takes «created»: the FFNS of node exists.
func (w *Writer) Created(node string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if p := w.peers[node]; p != nil && !w.stopped {
		p.created = true
	}
}

// Member takes a membership event of the main network: the NodeMeta of a
// member at its join and update, nil when the member is dead or left.
func (w *Writer) Member(name string, meta *domain.NodeMeta) {
	if name == w.deps.Self {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	p := w.peers[name]
	if p == nil {
		if meta == nil {
			return
		}

		p = &peer{}
		w.peers[name] = p
	}

	switch {
	case meta == nil && !p.dead:
		p.dead, p.deadAt = true, time.Now()
		p.deaths++
		p.created, p.wrote = false, false
		p.leaving = w.deps.Network.Leaving(name)

		death := p.deaths
		w.spawn(func(ctx context.Context) { w.write(ctx, name, death) })
	case meta != nil && p.dead:
		p.dead, p.meta, p.created = false, *meta, false

		wrote := p.wrote
		w.spawn(func(ctx context.Context) { w.cancelOnReturn(ctx, name, wrote) })
	case meta != nil:
		p.meta = *meta
	}
}

// spawn runs f while Run runs. Call it with mu held.
func (w *Writer) spawn(f func(ctx context.Context)) {
	if w.ctx == nil {
		return
	}

	ctx := w.ctx
	w.wg.Go(func() { f(ctx) })
}

// write waits for the wave of the own HRW rank and creates the FFNS of the
// dead peer. While the own node does not pass the health gate, it tries again
// each writerWaveInterval, until the peer comes back or the FFNS exists. The
// peer can come back while the create is in progress, before the object
// exists. Thus after the create the creator deletes the FFNS when the peer is
// alive.
func (w *Writer) write(ctx context.Context, name string, death uint64) {
	r := rank(name, w.deps.Self, w.deps.Network.Peers())
	wave := time.Duration(r/waveSize) * w.waveInterval()

	w.mu.Lock()
	p := w.peers[name]
	leaving := p.leaving || p.meta.Leaving
	w.mu.Unlock()

	w.logger.Info("peer is dead in the main network", "peer", name, "leaving", leaving, "rank", r, "wave_in", wave.String())

	if !sleep(ctx, wave) {
		return
	}

	create := func() error {
		return attempt(ctx, func(ctx context.Context) error { return w.create(ctx, name, death) }, func(n int, err error) {
			w.logger.Warn("create the FFNS", "peer", name, "attempt", n, "error", err)
		})
	}

	err := create()
	for retries := 0; errors.Is(err, errUnhealthy); retries++ {
		if retries == 0 {
			w.logger.Info("FFNS create waits for the health gate of this writer, retry each writerWaveInterval",
				"peer", name, "reason", err.Error(), "retry_in", w.waveInterval().String())
		}

		if !sleep(ctx, w.waveInterval()) {
			return
		}

		if err = create(); err == nil {
			w.logger.Info("FFNS create retry after the health gate succeeded", "peer", name, "retries", retries+1)
		}
	}

	if err != nil {
		if errors.Is(err, errNotAllowed) {
			w.logger.Info("no FFNS from this writer", "peer", name, "reason", err.Error())
		}

		// The peer came back during or after a create that timed out. The
		// FFNS can exist, so the writer deletes it as its creator.
		if w.backAfterTimeout(name, death) {
			w.logger.Info("FFNS can exist after a create that timed out, and the peer came back: delete it", "peer", name)
			w.cancel(ctx, name)
		}

		return
	}

	w.logger.Info("FFNS created", "peer", name, "rank", r)

	back := w.markCreated(name, death)
	w.broadcast(name)

	if back {
		w.cancel(ctx, name)
	}
}

// create is one attempt to create the FFNS of the dead peer. It checks first,
// in one step, that the peer is still dead, that the FFNS is not created, and
// that the own node may write.
func (w *Writer) create(ctx context.Context, name string, death uint64) error {
	w.mu.Lock()
	reason := w.denied(name, death)
	unhealthy := w.unhealthy(time.Now())
	p := w.peers[name]
	meta, deadAt := p.meta, p.deadAt
	w.mu.Unlock()

	switch {
	case reason != "":
		return fmt.Errorf("%w: %s", errNotAllowed, reason)
	case unhealthy != "":
		return fmt.Errorf("%w: %s", errUnhealthy, unhealthy)
	}

	ffns, err := w.newFFNS(name, meta, v1alpha1.SourceMemberlistDead, deadAt)
	if err != nil {
		return err
	}

	err = w.deps.API.CreateFFNS(ctx, ffns)

	switch {
	case apierrors.IsAlreadyExists(err):
		return nil
	case timedOut(err):
		w.mu.Lock()
		if p := w.peers[name]; p.deaths == death {
			p.wrote = true
		}
		w.mu.Unlock()
	}

	return err
}

// backAfterTimeout tells whether the peer came back after the death, and the
// own create of this death timed out.
func (w *Writer) backAfterTimeout(name string, death uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	p := w.peers[name]

	return p.deaths == death && !p.dead && p.wrote
}

// timedOut tells whether the request timed out. The server can complete such
// a request.
func timedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err)
}

// newFFNS returns the FFNS of the peer from its NodeMeta. For an armed peer
// the profile is victimProfile of the node group in the NodeMeta. Without the
// Node UID, or without the node group or the profile of an armed peer, it
// returns errNotAllowed: there is no FFNS and thus no taint.
func (w *Writer) newFFNS(name string, meta domain.NodeMeta,
	source v1alpha1.FencingFailedNodeStateSource, detectedAt time.Time,
) (*v1alpha1.FencingFailedNodeState, error) {
	if meta.NodeUID == "" || (meta.Armed && (meta.NodeGroup == "" || meta.Profile == "")) {
		return nil, fmt.Errorf("%w: the NodeMeta of the peer has no Node UID, node group or profile", errNotAllowed)
	}

	var profile *v1alpha1.ProfileRef

	if meta.Armed {
		p, err := victimProfile(w.deps.Settings().Config, meta.NodeGroup, meta.Profile)
		if err != nil {
			return nil, err
		}

		profile = p
	}

	blockOwnerDeletion := false
	ffns := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "v1",
				Kind:               "Node",
				Name:               name,
				UID:                types.UID(meta.NodeUID),
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			Source:     source,
			DetectedBy: w.deps.Self,
			DetectedAt: metav1.NewMicroTime(detectedAt),
			BootID:     meta.BootID,
			Armed:      meta.Armed,
			ProfileRef: profile,
		},
	}

	return ffns, nil
}

// denied returns why the writer may not create the FFNS of the death of the
// peer, or "" when it may. The health gate of the own node is separate. Call
// it with mu held.
func (w *Writer) denied(name string, death uint64) string {
	p := w.peers[name]

	switch {
	case w.stopped:
		return "the agent stops"
	case p.deaths != death || !p.dead:
		return "the peer came back"
	case p.created:
		return "the FFNS is created"
	case p.leaving || p.meta.Leaving:
		return "the peer left on purpose"
	case p.meta.Maintenance:
		return "the peer is in maintenance"
	}

	return ""
}

// victimProfile returns the slower of two profiles: the profile in the
// NodeMeta of the peer and the profile of its node group in the own
// configuration. While a change of the mode spreads, one of them can be old,
// and the slower one is not shorter than the watchdog timeout of the peer.
// The order of the profiles is fixed: critical, normal, slow. A group that
// the own configuration does not arm gives slow. A slower profile only makes
// the taint later.
func victimProfile(cfg config.Config, group, fromMeta string) (*v1alpha1.ProfileRef, error) {
	own := v1alpha1.ProfileSlow.ObjectName()
	if mode := cfg.ModeOf(group); mode != config.ModeDisabled {
		own = cfg.ProfileName(mode)
	}

	slowest := 0

	for _, name := range []string{fromMeta, own} {
		if name == "" {
			continue
		}

		i := slices.IndexFunc(v1alpha1.ProfileNames(), func(p v1alpha1.ProfileName) bool { return p.ObjectName() == name })
		if i < 0 {
			return nil, fmt.Errorf("node group %q has no profile %q", group, name)
		}

		slowest = max(slowest, i)
	}

	return &v1alpha1.ProfileRef{Name: v1alpha1.ProfileNames()[slowest]}, nil
}

// markCreated takes the own create of the FFNS of the death. It returns true
// when the peer came back after that death.
func (w *Writer) markCreated(name string, death uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	p := w.peers[name]
	if p.deaths != death {
		return false
	}

	p.created, p.wrote = true, true

	return !p.dead
}

// broadcast sends «created» to the peers.
func (w *Writer) broadcast(name string) {
	msg := domain.FFNSCreated{Node: name}
	failed := 0

	for _, p := range w.deps.Network.Peers() {
		if p.Name == name {
			continue
		}

		if err := w.deps.Network.Send(p, msg); err != nil {
			failed++
		}
	}

	if failed > 0 {
		w.logger.Debug("send created", "peer", name, "send_errors", failed)
	}
}

// cancelOnReturn runs when the peer comes back. The first writers by HRW and
// the creator delete the FFNS of the peer.
func (w *Writer) cancelOnReturn(ctx context.Context, name string, wrote bool) {
	if !wrote && rank(name, w.deps.Self, w.deps.Network.Peers()) >= cancelers {
		return
	}

	w.cancel(ctx, name)
}

// cancel deletes the FFNS of the peer when the peer is alive with the boot ID
// of the FFNS. It keeps the FFNS in the phase Proven or a later phase: the
// node of that FFNS stops the feed of its watchdog. It deletes the FFNS only
// when the own node may write: the condition is the same as for the create.
// After failed attempts and while the own node may not write, it tries again
// each writerWaveInterval. It stops when the FFNS is gone or has another boot
// ID, and when the peer is dead again.
func (w *Writer) cancel(ctx context.Context, name string) {
	for {
		err := w.cancelOnce(ctx, name)
		if err == nil || !sleep(ctx, w.waveInterval()) {
			return
		}
	}
}

// cancelOnce is one series of attempts of cancel.
func (w *Writer) cancelOnce(ctx context.Context, name string) error {
	return attempt(ctx, func(ctx context.Context) error {
		ffns, err := w.deps.API.GetFFNS(ctx, name)
		if apierrors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		w.mu.Lock()
		p := w.peers[name]
		back := !p.dead && p.meta.BootID == ffns.Spec.BootID
		reason := w.unhealthy(time.Now())
		w.mu.Unlock()

		if !back || Proven(ffns) {
			return nil
		}

		if reason != "" {
			w.logger.Info("no delete of the FFNS of the peer that came back", "peer", name, "reason", reason)

			return fmt.Errorf("%w: %s", errNotAllowed, reason)
		}

		err = w.deps.API.DeleteFFNS(ctx, name, ffns.ResourceVersion)
		if apierrors.IsNotFound(err) {
			return nil
		}

		if err == nil {
			w.logger.Info("FFNS deleted: the peer came back with the same boot ID", "peer", name, "boot_id", ffns.Spec.BootID)
		}

		return err
	}, func(n int, err error) {
		w.logger.Warn("delete the FFNS of the peer that came back", "peer", name, "attempt", n, "error", err)
	})
}

func (w *Writer) waveInterval() time.Duration {
	return w.deps.Settings().Memberlist.Eviction.WriterWaveInterval.Duration
}

// attempt runs op up to len(pauses)+1 times, each time with attemptTimeout.
// It stops at the first success or at errNotAllowed. failed gets the number
// of each failed attempt, from 1, and its error.
func attempt(ctx context.Context, op func(ctx context.Context) error, failed func(n int, err error)) error {
	var err error

	for i := range len(pauses) + 1 {
		if i > 0 && !sleep(ctx, pauses[i-1]) {
			return ctx.Err()
		}

		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err = op(actx)
		cancel()

		if err == nil || errors.Is(err, errNotAllowed) {
			return err
		}

		failed(i+1, err)
	}

	return err
}

// sleep waits for d. It returns false when ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
