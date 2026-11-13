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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// unhealthy returns why the own node may not write, or "" when it may: it
// continuously sees the control plane for writerHealthyFor or more, it has no
// FFNS and it is not in maintenance. The feed of the watchdog is not a
// condition. Call it with mu held.
func (w *Writer) unhealthy(now time.Time) string {
	s := w.deps.Settings()

	switch {
	case s.Config.MemberlistProfile == "":
		return "no configuration"
	case w.own:
		return "the own FFNS exists"
	case w.deps.Maintenance():
		return "the own node is in maintenance"
	case w.cpSince.IsZero():
		return "the control plane is not visible"
	}

	since := w.cpSince
	if w.ownGoneAt.After(since) {
		since = w.ownGoneAt
	}

	need := writerHealthyFor(s.Memberlist, len(w.deps.Network.Peers())+1)
	if healthy := now.Sub(since); healthy < need {
		return fmt.Sprintf("healthy for %s of %s", healthy, need)
	}

	return ""
}

// writerHealthyFor is rejoin.interval + memberlist.suspicionMaxTimeoutMult ·
// memberlist.suspicionMult · max(1, log10 nodeCount) · memberlist.probeInterval
// of the memberlist profile p. nodeCount is the number of nodes in the static
// peer list with the own node.
func writerHealthyFor(p v1alpha1.FencingSLAProfileSpec, nodeCount int) time.Duration {
	m := p.Memberlist
	mult := float64(m.SuspicionMaxTimeoutMult) * float64(m.SuspicionMult) * max(1, math.Log10(float64(nodeCount)))

	return p.Rejoin.Interval.Duration + time.Duration(mult*float64(m.ProbeInterval.Duration))
}

// rank returns the HRW rank of self among the writers of victim: the number
// of writers with a higher score. The writers are the static peer list and
// the own node, without victim.
func rank(victim, self string, peers []domain.Peer) int {
	own := score(victim, self)
	r := 0

	for _, p := range peers {
		if p.Name == victim || p.Name == self {
			continue
		}

		if s := score(victim, p.Name); s > own || s == own && p.Name < self {
			r++
		}
	}

	return r
}

func score(victim, writer string) uint64 {
	sum := sha256.Sum256([]byte(victim + "\x00" + writer))

	return binary.BigEndian.Uint64(sum[:8])
}
