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
	"fmt"
	"strings"
	"sync"
)

type Mode string

const (
	ModeEnforce  Mode = "Enforce"
	ModeAudit    Mode = "Audit"
	ModeDisabled Mode = "Disabled"
)

func ParseMode(s string) (Mode, error) {
	switch m := Mode(s); m {
	case ModeEnforce, ModeAudit, ModeDisabled:
		return m, nil
	}
	return "", fmt.Errorf("unknown mode %q", s)
}

// MPTCP values recorded in the marker, so that Disabled restores exactly what was there before.
const (
	mptcpUnmanaged = "unmanaged" // we did not touch net.mptcp.enabled
	mptcpAbsent    = "absent"    // kernel without MPTCP
)

// State is what the guard installed into a netns; it is stored in each rule's marker.
type State struct {
	Mode Mode
	// Original net.mptcp.enabled value before the guard changed it: "0", "1", mptcpUnmanaged or mptcpAbsent.
	MPTCPOrig string
}

const markerPrefix = "d8-ipv4-options-guard"

var ruleNames = []string{"syn", "fragment"}

func marker(rule string, st State) string {
	return fmt.Sprintf("%s;rule=%s;mode=%s;mptcp=%s", markerPrefix, rule, st.Mode, st.MPTCPOrig)
}

func parseMarker(s string) (string, State, bool) {
	parts := strings.Split(s, ";")
	if len(parts) != 4 || parts[0] != markerPrefix {
		return "", State{}, false
	}
	kv := map[string]string{}
	for _, p := range parts[1:] {
		k, v, found := strings.Cut(p, "=")
		if !found {
			return "", State{}, false
		}
		kv[k] = v
	}
	return kv["rule"], State{Mode: Mode(kv["mode"]), MPTCPOrig: kv["mptcp"]}, true
}

// Backend installs and removes the filter in a single network namespace.
// Applied is what Backend.Apply found.
type Applied struct {
	// Repaired: a filter of the same state was there but changed (by hand or by another
	// controller) and was rewritten.
	Repaired bool
	// Bypassed: a rule ahead of the filter can accept packets before they reach it. The rule is
	// left alone: whatever added it may keep re-adding it, and the guard does not fight it.
	Bypassed bool
}

type Backend interface {
	Name() string
	// Apply installs the filter of st.
	Apply(ns *NetNS, st State) (Applied, error)
	// Read returns the state installed in the netns, if any.
	Read(ns *NetNS) (State, bool)
	// Remove deletes the filter and returns the state it carried, if any.
	Remove(ns *NetNS) (State, bool, error)
	// Counters returns matched packets per rule name.
	Counters(ns *NetNS) (map[string]uint64, error)
}

// renderings keeps how the kernel listed a filter right after the guard wrote it. Another kernel,
// library or iptables version may list the same filter differently from the expected form;
// what was read back after our own write is ours too, so it is not rewritten on every resync.
var renderings = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

func rememberRendering(key, r string) {
	renderings.Lock()
	defer renderings.Unlock()
	renderings.m[key] = r
}

func hasRendering(key string) bool {
	renderings.Lock()
	defer renderings.Unlock()
	_, ok := renderings.m[key]
	return ok
}

func knownRendering(key, r string) bool {
	renderings.Lock()
	defer renderings.Unlock()
	v, ok := renderings.m[key]
	return ok && v == r
}
