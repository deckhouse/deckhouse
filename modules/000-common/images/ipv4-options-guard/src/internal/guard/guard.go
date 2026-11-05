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

package guard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const hostNetNS = "host"

type Config struct {
	// Settings file from the CNI module ConfigMap, re-read on every resync.
	SettingsFile string
	// Directory with pod netns bind mounts (containerd: /run/netns).
	NetNSDir string
	// Host netns: the CNI agent pods run with hostNetwork, so it is our own netns.
	HostNetNSPath string
	// HostMPTCPStateFile keeps the host net.mptcp.enabled from before the guard; on the host /run,
	// so a reboot clears it together with the change. Empty means the host value is not managed.
	HostMPTCPStateFile string
	// HostMPTCPSysctl is net.mptcp.enabled of the host netns; empty means /proc/sys/net/mptcp/enabled.
	HostMPTCPSysctl string
	Iptables        Iptables
	Resync          time.Duration
	// NodeDisabled reports a per-node opt-out; nil means no opt-out.
	NodeDisabled func() (bool, error)
}

type Guard struct {
	cfg Config
	log *slog.Logger
	m   *metrics

	// reconcileMu serializes reconcileOne: a new netns can be reconciled from the watcher and
	// from resync at the same time, and both read-modify-write its MPTCP value and rules.
	reconcileMu sync.Mutex

	// Last readable settings file and node opt-out, kept apart so a failed read of one
	// does not freeze the other. Only refresh touches them.
	raw    Settings
	optOut bool

	mu      sync.Mutex
	st      Settings // effective: the node opt-out turns Mode into Disabled
	backend Backend
	// stale is removed from every netns until a resync without failures: the filter of the
	// previous backend, or one left by an earlier run with a different choice.
	stale Backend
	// staleDone are the netns already cleaned of stale, so a netns that keeps failing does
	// not make every other one look for it on each resync.
	staleDone map[string]bool
	reason    string // metric label: Detection.Code
	mismatch  bool
	allowMod  bool
	counters  map[string]map[string]uint64 // netns -> rule -> last seen packets
	bypassed  map[string]bool              // netns with a rule that accepts packets ahead of the filter
}

func New(cfg Config, log *slog.Logger, m *metrics) *Guard {
	return &Guard{cfg: cfg, log: log, m: m, raw: DefaultSettings(), st: DefaultSettings(), counters: map[string]map[string]uint64{}, bypassed: map[string]bool{}}
}

func (g *Guard) state() (Settings, Backend, Backend) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.st, g.backend, g.stale
}

// refresh re-reads settings and the node opt-out and re-detects the backend if needed.
func (g *Guard) refresh() {
	if raw, err := LoadSettings(g.cfg.SettingsFile); err != nil {
		// Keep the previous settings rather than flapping on a broken file.
		g.log.Warn("read settings", "error", err)
	} else {
		g.raw = raw
	}
	if g.cfg.NodeDisabled != nil {
		// On an error the previous opt-out is kept.
		if disabled, err := g.cfg.NodeDisabled(); err != nil {
			g.log.Warn("read node labels", "error", err)
		} else {
			g.optOut = disabled
		}
	}
	st := g.raw
	if g.optOut {
		st.Mode = ModeDisabled
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if st != g.st {
		g.log.Info("settings changed", "mode", st.Mode, "disable_mptcp", st.DisableMPTCP, "allow_kernel_module_load", st.AllowKernelModuleLoad)
	}
	g.st = st
	// On a mismatch the modules of the wrapper mode may get loaded later: keep re-detecting.
	if g.backend == nil || g.mismatch || g.allowMod != st.AllowKernelModuleLoad {
		d := DetectBackend(g.cfg.Iptables, st.AllowKernelModuleLoad)
		switch {
		case d.Backend == nil && g.backend == nil:
			g.log.Error("node cannot be protected", "reason", d.Reason, "code", d.Code)
			g.reason = d.Code
		case d.Backend == nil:
			// Keep the installed filter.
		case g.backend == nil || d.Backend.Name() != g.backend.Name():
			g.log.Info("backend selected", "backend", d.Backend.Name(), "reason", d.Reason, "mismatch", d.Mismatch)
			g.stale, g.staleDone = d.Stale, map[string]bool{}
			if g.backend != nil {
				g.stale = g.backend
			}
			g.backend, g.reason, g.mismatch = d.Backend, d.Code, d.Mismatch
		default:
			g.reason, g.mismatch = d.Code, d.Mismatch
		}
		g.allowMod = st.AllowKernelModuleLoad
	}
	name := "none"
	if g.backend != nil {
		name = g.backend.Name()
	}
	g.m.SetInfo(st.Mode, name, g.reason, g.backend != nil || st.Mode == ModeDisabled, g.mismatch)
}

func (g *Guard) Run(ctx context.Context) {
	g.resync()

	w, err := fsnotify.NewWatcher()
	if err == nil {
		defer w.Close()
		err = w.Add(g.cfg.NetNSDir)
	}
	var events <-chan fsnotify.Event
	var errs <-chan error
	if err != nil {
		// New pods are then picked up on the next resync only.
		g.log.Warn("watch netns dir", "dir", g.cfg.NetNSDir, "error", err)
	} else {
		events, errs = w.Events, w.Errors
	}

	t := time.NewTicker(g.cfg.Resync)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			g.resync()
		case ev := <-events:
			if ev.Op&fsnotify.Create != 0 {
				// containerd creates the file first and bind-mounts the netns right after.
				go g.reconcileNew(filepath.Base(ev.Name))
			}
		case err := <-errs:
			g.log.Warn("netns watcher", "error", err)
		}
	}
}

func (g *Guard) reconcileNew(name string) {
	for i := 0; i < 10; i++ {
		time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
		if err := g.reconcileOne(name, filepath.Join(g.cfg.NetNSDir, name)); err == nil {
			return
		}
	}
}

func (g *Guard) resync() {
	g.refresh()
	st, b, stale := g.state()
	g.syncHostMPTCP(st, b != nil)
	if b == nil {
		return
	}

	ok, failed := 0, 0
	seen := map[string]bool{hostNetNS: true}
	if err := g.reconcileOne(hostNetNS, g.cfg.HostNetNSPath); err != nil {
		failed++
		g.log.Error("reconcile", "netns", hostNetNS, "error", err)
	} else {
		ok++
	}

	entries, err := os.ReadDir(g.cfg.NetNSDir)
	if err != nil {
		// Unlisted pod netns may still hold the stale filter: not a clean pass.
		failed++
		g.log.Error("list netns", "error", err)
	}
	for _, e := range entries {
		seen[e.Name()] = true
		if err := g.reconcileOne(e.Name(), filepath.Join(g.cfg.NetNSDir, e.Name())); err != nil {
			// A netns that went away between listing and opening, or a file containerd left
			// without a netns mounted on it, is not a failure.
			if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, errNotNetNS) {
				failed++
				g.log.Error("reconcile", "netns", e.Name(), "error", err)
			}
			continue
		}
		ok++
	}

	g.mu.Lock()
	if failed == 0 && g.stale == stale {
		g.stale, g.staleDone = nil, nil
	}
	for name := range g.staleDone {
		if g.gone(name, seen) {
			delete(g.staleDone, name)
		}
	}
	for name := range g.counters {
		if g.gone(name, seen) {
			delete(g.counters, name)
		}
	}
	for name := range g.bypassed {
		if g.gone(name, seen) {
			delete(g.bypassed, name)
		}
	}
	bypassed := len(g.bypassed)
	g.mu.Unlock()
	g.m.setNetNS(ok, failed)
	g.m.bypassed.Set(float64(bypassed))
}

// gone reports whether a netns no longer exists. Not being listed is not enough: reconcileNew
// may have picked up a netns created after the listing, and its counter baseline must stay.
func (g *Guard) gone(name string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	_, err := os.Lstat(filepath.Join(g.cfg.NetNSDir, name))
	return errors.Is(err, os.ErrNotExist)
}

func (g *Guard) staleCleaned(name string, stale Backend) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stale == stale && g.staleDone[name]
}

func (g *Guard) markStaleCleaned(name string, stale Backend) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stale == stale && g.staleDone != nil {
		g.staleDone[name] = true
	}
}

// manageMPTCP: MPTCP is a behavior change, so it is only touched in Enforce.
func manageMPTCP(st Settings) bool {
	return st.Mode == ModeEnforce && st.DisableMPTCP
}

func (g *Guard) reconcileOne(name, path string) (err error) {
	g.reconcileMu.Lock()
	defer g.reconcileMu.Unlock()

	st, backend, stale := g.state()
	if backend == nil {
		return nil
	}
	if stale != nil {
		if g.staleCleaned(name, stale) {
			stale = nil
		} else {
			defer func() {
				if err == nil {
					g.markStaleCleaned(name, stale)
				}
			}()
		}
	}
	ns, err := OpenNetNS(name, path)
	if err != nil {
		return err
	}
	defer ns.Close()
	// The host MPTCP value is managed by syncHostMPTCP through its state file.
	podMPTCP := name != hostNetNS

	if st.Mode == ModeDisabled {
		g.setBypassed(name, false)
		return restore(ns, backend, stale, podMPTCP)
	}

	orig := mptcpUnmanaged
	prev, ok := backend.Read(ns)
	if !ok && stale != nil {
		// Switching backends: the MPTCP value before the guard is kept in the old filter.
		prev, ok = stale.Read(ns)
	}
	if ok && podMPTCP {
		orig = prev.MPTCPOrig
	}
	if !podMPTCP {
		return g.apply(ns, backend, stale, State{Mode: st.Mode, MPTCPOrig: orig})
	}

	cur, err := ns.MPTCP()
	if err != nil {
		return err
	}
	switch {
	case manageMPTCP(st) && orig == mptcpUnmanaged:
		orig = cur
	case !manageMPTCP(st) && orig != mptcpUnmanaged:
		if orig == "1" && cur == "0" {
			if err := ns.SetMPTCP("1"); err != nil {
				return err
			}
		}
		orig = mptcpUnmanaged
	}
	if manageMPTCP(st) && cur == "1" {
		// Affects new sockets only: listeners opened earlier stay MPTCP until restarted.
		if err := ns.SetMPTCP("0"); err != nil {
			return err
		}
	}

	return g.apply(ns, backend, stale, State{Mode: st.Mode, MPTCPOrig: orig})
}

// apply installs the filter and only then removes the stale one, so the netns is never left unfiltered.
func (g *Guard) apply(ns *NetNS, backend, stale Backend, st State) error {
	res, err := backend.Apply(ns, st)
	if res.Repaired {
		// Something else edits the filter: worth a look even though it is restored.
		g.log.Warn("filter was changed outside the guard and rewritten", "netns", ns.Name, "backend", backend.Name())
		g.m.repaired.WithLabelValues(backend.Name()).Inc()
	}
	if err != nil {
		return err
	}
	g.setBypassed(ns.Name, res.Bypassed)
	if stale != nil {
		if _, _, err := stale.Remove(ns); err != nil {
			return fmt.Errorf("remove the %s filter: %w", stale.Name(), err)
		}
	}
	g.collect(ns, backend)
	return nil
}

// setBypassed records whether packets can skip the filter in a netns and logs when that starts.
func (g *Guard) setBypassed(name string, bypassed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !bypassed {
		delete(g.bypassed, name)
		return
	}
	if !g.bypassed[name] {
		g.log.Warn("a rule ahead of the filter in raw PREROUTING can accept packets before they reach it; it is left in place",
			"netns", name)
	}
	g.bypassed[name] = true
}

// restore removes the filter and, in a pod netns, returns net.mptcp.enabled to its value before the guard.
func restore(ns *NetNS, backend, stale Backend, podMPTCP bool) error {
	st, found, err := backend.Remove(ns)
	if err != nil {
		return err
	}
	if stale != nil {
		staleSt, staleFound, err := stale.Remove(ns)
		if err != nil {
			return fmt.Errorf("remove the %s filter: %w", stale.Name(), err)
		}
		if !found {
			st, found = staleSt, staleFound
		}
	}
	if !found || !podMPTCP {
		return nil
	}
	if st.MPTCPOrig == "1" {
		if cur, err := ns.MPTCP(); err == nil && cur == "0" {
			return ns.SetMPTCP("1")
		}
	}
	return nil
}

func (g *Guard) collect(ns *NetNS, backend Backend) {
	cnt, err := backend.Counters(ns)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	last := g.counters[ns.Name]
	if last == nil {
		// The first look at a netns only takes the baseline: the counters survive guard restarts
		// and filter rewrites, so adding them now would count the same packets again.
		last = map[string]uint64{}
		for rule, v := range cnt {
			last[rule] = v
		}
		g.counters[ns.Name] = last
		return
	}
	for rule, v := range cnt {
		if v > last[rule] {
			g.m.matched.WithLabelValues(rule).Add(float64(v - last[rule]))
		}
		last[rule] = v
	}
}
