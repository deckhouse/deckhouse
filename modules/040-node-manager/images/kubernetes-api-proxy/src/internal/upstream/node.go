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

package upstream

import "time"

type node struct {
	backend   *Upstream
	score     float64
	tier      Tier
	heldUntil time.Time
	missing   int
	lastKind  probeKind
	penalized bool
	downed    bool
	warm      bool
	unsettled bool

	answeredAt  time.Time
	readySince  time.Time
	lastReadyAt time.Time
	stalling    bool
	evictions   int
}

func (n node) lastProbe() string {
	if n.lastKind == 0 {
		return ""
	}

	return n.lastKind.String()
}

func (n node) lastReadyAgo(now time.Time) *int64 {
	if n.lastReadyAt.IsZero() {
		return nil
	}

	ms := now.Sub(n.lastReadyAt).Milliseconds()

	return &ms
}

func (n node) witnesses(start, now time.Time, window time.Duration) bool {
	return n.missing == 0 && !n.warm && n.score >= 0 && !now.Before(n.heldUntil) &&
		!n.readySince.IsZero() && now.Sub(n.readySince) >= window && !n.answeredAt.Before(start)
}

type ExportNode struct {
	Upstream      string  `json:"upstream"`
	Score         float64 `json:"score"`
	Tier          Tier    `json:"tier"`
	EffectiveTier Tier    `json:"effectiveTier"`
	Missing       int     `json:"missing"`
	LastProbe     string  `json:"lastProbe"`
	Warm          bool    `json:"warm"`
	Downed        bool    `json:"downed"`
	Held          bool    `json:"held"`
	Stalling      bool    `json:"stalling"`
	Evictions     int     `json:"evictions"`
	// LastReadyAgoMs is the time in milliseconds since the last probe that
	// answered ready. It is nil when the upstream was never ready.
	LastReadyAgoMs *int64 `json:"lastReadyAgoMs,omitempty"`
}
