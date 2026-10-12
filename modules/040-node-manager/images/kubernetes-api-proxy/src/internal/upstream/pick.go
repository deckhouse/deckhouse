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

func (list *List) Pick() (*Upstream, error) {
	list.mu.Lock()
	defer list.mu.Unlock()

	if len(list.nodes) == 0 {
		return nil, ErrNoUpstreams
	}

	list.current = list.pickLocked(func(int) bool { return true })

	return list.nodes[list.current].backend, nil
}

func (list *List) PickNot(exclude *Upstream) (*Upstream, error) {
	list.mu.Lock()
	defer list.mu.Unlock()

	if len(list.nodes) == 0 {
		return nil, ErrNoUpstreams
	}

	best := list.pickLocked(func(i int) bool { return !list.cmp(list.nodes[i].backend, exclude) })
	if best < 0 {
		return nil, ErrNoUpstreams
	}

	list.current = best

	return list.nodes[best].backend, nil
}

func (list *List) pickLocked(allowed func(int) bool) int {
	best := -1

	for j := range list.nodes {
		i := (list.current + 1 + j) % len(list.nodes)
		if !allowed(i) {
			continue
		}

		if best < 0 || list.preferredLocked(i, best) {
			best = i
		}
	}

	return best
}

func (list *List) preferredLocked(a, b int) bool {
	ta, tb := list.effectiveTierLocked(a), list.effectiveTierLocked(b)
	if (ta != tierFailed) != (tb != tierFailed) {
		return ta != tierFailed
	}

	if ta == tierFailed {
		return list.preferredFailedLocked(a, b)
	}

	sa, sb := list.nodes[a].score, list.nodes[b].score
	if (sa >= 0) != (sb >= 0) {
		return sa >= 0
	}

	return ta < tb
}

func (list *List) preferredFailedLocked(a, b int) bool {
	ra, rb := list.failRankLocked(a), list.failRankLocked(b)
	if ra != rb {
		return ra < rb
	}

	sa, sb := list.nodes[a].score, list.nodes[b].score
	if (sa >= 0) != (sb >= 0) {
		return sa >= 0
	}

	return sa < 0 && sa > sb
}

func (list *List) failRankLocked(i int) int {
	n := list.nodes[i]

	switch {
	case n.missing > 0:
		return 4
	case n.downed || n.lastKind == probeHard:
		return 3
	case n.warm:
		return 2
	case n.penalized:
		return 1
	default:
		return 0
	}
}
