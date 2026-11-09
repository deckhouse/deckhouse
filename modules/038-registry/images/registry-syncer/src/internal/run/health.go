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

package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/report"
)

// healthEvery is how often the health watch looks at this replica's registry and store, unless
// Loop.HealthEvery says otherwise.
const healthEvery = 30 * time.Second

// registryProbeTimeout is how long the registry has to answer one probe. Generous for a request that
// touches no storage, and short beside the five minutes of silence it takes to report one.
const registryProbeTimeout = 5 * time.Second

// storeProbeTimeout is how long one write probe of the store may take, unless Loop.StoreProbeTimeout
// says otherwise. A write of a few bytes and its fsync; a disk that needs half a minute for that is not
// one a registry can serve from.
const storeProbeTimeout = 30 * time.Second

// storeFailingReported is how long the store may refuse the probe before the status says so. Shorter
// than the registry's grace: a restart of the registry is an ordinary event that takes minutes on a
// large store, while a disk that refuses a write for two of them is not going to start accepting them.
const storeFailingReported = 2 * time.Minute

// storeProbeFile is what the write probe writes, at the root of the data directory: outside
// docker/registry/v2, so that neither the registry nor the collection nor a count of the store ever
// sees it, and gone again a moment after it appears.
const storeProbeFile = ".syncer-probe"

// HealthError is what the health watch found wrong with this replica's own registry or store, and empty
// while it found nothing — or has not looked yet.
//
// Read by the pass, which publishes it as its own error when it has none, and by the election, which
// judges this replica by it before the status catches up — see WithOwnHealth. The watch publishes it as
// healthError as well, which only it writes.
func (l *Loop) HealthError() string {
	l.healthMu.Lock()
	defer l.healthMu.Unlock()
	return l.healthErr
}

// watchHealth probes this replica's registry and store on its own schedule, and publishes what it
// finds, whatever the pass is doing.
//
// A pass learns about its registry only when it copies, and nothing at all about its disk beyond a
// statfs. A leader with an upstream and nothing to copy — the steady state of a healthy cluster — went
// on reporting itself full while its registry answered nobody, or while its disk had gone read-only
// under it, and kept the lease: the elector renews on its own and does not care whether the replica
// still serves. In air-gap that leader is where every node pulls from.
//
// Apart from the pass for the same reason watchStore is, and one more. A disk stuck in the kernel
// blocks whatever touches it, and the pass touches it — it walks the store and measures it before it
// publishes — so a pass on such a disk never publishes anything, and the last report, clean and full,
// stands. The watch publishes the error itself, through a reporter that writes only those two fields,
// error and healthError, and touches the disk only from a goroutine of its own that it is prepared to
// abandon.
//
// What it publishes makes the replica not full, which is what the election reads: a leader with a full
// follower beside it steps aside, one without keeps leading, as with any other error — see MayLead. And
// healthError keeps the replica from being the one chosen to begin while nobody is full and nobody
// leads, which a pass's error does not — see leadsWhenNobodyIsFull. Taken back as soon as the registry
// answers and the disk takes a write again; the replica's completeness is left alone throughout and
// comes back with it.
func (l *Loop) watchHealth(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = healthEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	published, reported := "", false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		found := l.checkHealth(ctx)
		if ctx.Err() != nil {
			return
		}
		published, reported = l.publishHealth(ctx, found, published, reported)
	}
}

// checkHealth runs one round of probes and records what it found, which it also returns.
//
// Each failure is reported only once it has lasted: the registry's for registryDownReported, which
// covers the walk of the whole store the wrapper makes before it opens a port after every restart, and
// the store's for storeFailingReported.
//
// The registry is not asked before a pass has configured it. The wrapper starts it only once there is a
// configuration, and a pass writes one only after it has read the storage object: a pod that starts
// while the API server is away for minutes — a cluster powering up — would otherwise have timed a
// silence of its own making, and reported it the moment the registry began its walk of the store.
func (l *Loop) checkHealth(ctx context.Context) string {
	now := time.Now()
	var found []string

	if !l.configured.Load() {
		// Not configured yet: nothing to ask, and no silence to time.
	} else if err := l.probeRegistry(ctx); err != nil {
		since := l.registryDownSince.Load()
		if since == 0 {
			since = now.UnixNano()
			l.registryDownSince.Store(since)
		}
		if now.Sub(time.Unix(0, since)) >= registryDownReported {
			found = append(found, fmt.Sprintf("this replica's registry is not answering: %v", err))
		}
	} else {
		l.registryDownSince.Store(0)
	}

	if err := l.probeStoreWrite(ctx); err != nil {
		if l.storeFailingSince.IsZero() {
			l.storeFailingSince = now
		}
		if now.Sub(l.storeFailingSince) >= storeFailingReported {
			found = append(found, fmt.Sprintf("this replica's store does not accept writes: %v", err))
		}
	} else {
		l.storeFailingSince = time.Time{}
	}

	healthErr := strings.Join(found, "; ")
	l.healthMu.Lock()
	changed := l.healthErr != healthErr
	l.healthErr = healthErr
	l.healthMu.Unlock()

	if changed {
		if healthErr != "" {
			l.Log.Warn("this replica cannot serve what it holds, and says so in its status", "error", healthErr)
		} else {
			l.Log.Info("this replica's registry answers and its store takes writes again")
		}
	}
	return healthErr
}

// publishHealth writes what the health watch found into this replica's status entry, and returns what
// stands there now from this watch.
//
// Written again on every round while something is wrong, not only when it changes: a pass that began
// before the failure was found publishes its report without it, and the next round puts it back. A
// write that changes nothing costs one read — see report.Publisher.publish.
//
// And written on the first round that gets through, reported or not, even with nothing found. Only the
// watch writes healthError — every pass keeps the one standing — so a value left by an earlier run of
// this process, or by the pod before it, can only be taken back here; skipped while nothing was found,
// it would stand in the status for good, and keep a replica that serves out of the running whenever
// nobody is full and nobody leads.
func (l *Loop) publishHealth(ctx context.Context, found, published string, reported bool) (string, bool) {
	if l.Publisher == nil {
		return found, reported
	}
	if reported && found == "" && published == "" {
		return found, reported
	}
	state := report.State{
		Node:        l.Node,
		Role:        Role(l.Leadership != nil && l.Leadership.IsLeader()),
		Address:     l.ReportedAddress,
		Error:       found,
		HealthError: found,
	}
	if err := l.Publisher.PublishHealth(ctx, state, published); err != nil {
		if ctx.Err() == nil {
			l.Log.Warn("reporting this replica's health failed", "error", err.Error())
		}
		return published, reported
	}
	return found, true
}

// probeRegistry asks both of this replica's listeners for the API root: the serving one the nodes pull
// from, and the write endpoint a fill, a replication and a push land on.
//
// Any HTTP answer is an answer. The root asks for a token, so a healthy registry says 401, and says it
// without touching the store — which is the point: this asks whether the process serves, and the write
// probe asks about the disk. A connection that is accepted and never answered is the case a dial cannot
// see and this does.
//
// Through localTransport, the transport the fill's clients are built on, so that the probe trusts what
// the copy trusts and presents what the copy presents: a listener that admits the fill and refuses the
// probe would have every replica report its registry as not answering. The write endpoint's address is
// the one writeRegistry gives. Each probe on a connection of its own: a connection kept from the last
// one says nothing about whether the registry still accepts new ones.
func (l *Loop) probeRegistry(ctx context.Context) error {
	if l.LocalAddress == "" {
		return nil
	}
	write, err := l.writeRegistry()
	if err != nil {
		return err
	}
	transport, err := l.localTransport()
	if err != nil {
		return err
	}
	transport.DisableKeepAlives = true
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: registryProbeTimeout}

	for _, address := range []string{l.LocalAddress, write.Address} {
		if err := pingRegistry(ctx, client, address); err != nil {
			return fmt.Errorf("%s: %w", address, err)
		}
	}
	return nil
}

func pingRegistry(ctx context.Context, client *http.Client, address string) error {
	ctx, cancel := context.WithTimeout(ctx, registryProbeTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+address+"/v2/", http.NoBody)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.Body.Close()
}

// probeStoreWrite writes to the store and takes the write back, in a goroutine it does not wait on for
// longer than the probe's timeout.
//
// At most one probe at a time. A write stuck in the kernel cannot be interrupted, and its goroutine with
// it; starting another on every round would pile them up for as long as the disk hangs. So a probe that
// has not returned is the answer to every round until it does: the store still does not take a write.
//
// A store that is full is not a failing store. The registry refuses writes long before the disk runs out
// — see the reserve — and a store at its budget or at the reserve is reported as such already; read as a
// failure here, it would take the lease off a leader whose only problem is room.
func (l *Loop) probeStoreWrite(ctx context.Context) error {
	if l.DataDir == "" {
		return nil
	}
	timeout := l.StoreProbeTimeout
	if timeout <= 0 {
		timeout = storeProbeTimeout
	}

	if l.storeProbing == nil {
		probe := l.StoreProbe
		if probe == nil {
			probe = probeStore
		}
		done := make(chan error, 1)
		dir := l.DataDir
		go func() { done <- probe(dir) }()
		l.storeProbing, l.storeProbeStarted = done, time.Now()
	}

	finished := func(err error) error {
		l.storeProbing = nil
		if isOutOfRoom(err) {
			return nil
		}
		return err
	}

	// A probe that returned since the last round is its answer, even past its time: asked together
	// with an expired timer, the select below would pick either.
	select {
	case err := <-l.storeProbing:
		return finished(err)
	default:
	}

	wait := time.NewTimer(max(timeout-time.Since(l.storeProbeStarted), 0))
	defer wait.Stop()

	select {
	case err := <-l.storeProbing:
		return finished(err)
	case <-wait.C:
		// The same words for as long as the write hangs, how long it has hung included: the error is
		// published, and a text that changed every round would be a new status write and a new warning
		// every half a minute — and would no longer match what the watch published, so that the watch
		// could not take it back. See report.Publisher.PublishHealth.
		return fmt.Errorf("a write to %s has not finished within %s", l.DataDir, timeout)
	case <-ctx.Done():
		return nil
	}
}

// isOutOfRoom reports whether a write was refused for want of room rather than by a failing disk.
func isOutOfRoom(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}

// probeStore creates a file in the store, writes to it, syncs it to the disk and removes it. What the
// registry does to every blob it receives, in miniature — and what a filesystem remounted read-only
// after an error, or a disk that has stopped answering, refuses.
func probeStore(dir string) error {
	path := filepath.Join(dir, storeProbeFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	syncErr := file.Sync()
	closeErr := file.Close()
	removeErr := os.Remove(path)
	return errors.Join(writeErr, syncErr, closeErr, removeErr)
}

// WithOwnHealth is the reports the election reads, with this replica's own entry carrying what its
// health watch found: in healthError always, and in error when the entry says nothing worse.
//
// The status catches up within one round of the watch, or not at all when the API server is what
// stands in the way. The replica deciding about itself does not have to wait for it: a leader whose
// registry does not answer, or whose disk refuses writes, is not full in its own eyes at once, and
// steps aside for a full replica on the next review rather than when the others read its report. With
// no full replica beside it, it keeps leading — moving the lease to an incomplete one would gain nothing
// — exactly as with an error it had published. Nor does it stand to begin while nobody leads.
//
// Only ever in that direction. A finding the watch has published and since taken back is left in the
// reports until the status says so: the replica does not judge itself healthier than the others see it,
// or it would stand where they stand aside for each other.
func WithOwnHealth(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, healthErr string,
) []registryv1alpha1.StorageReplicaStatus {
	if healthErr == "" {
		return replicas
	}
	judged := make([]registryv1alpha1.StorageReplicaStatus, len(replicas))
	copy(judged, replicas)
	for i := range judged {
		if judged[i].Node != self {
			continue
		}
		judged[i].HealthError = healthErr
		if judged[i].Error == "" {
			judged[i].Error = healthErr
		}
	}
	return judged
}
