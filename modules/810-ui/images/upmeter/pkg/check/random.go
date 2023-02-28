/*
Copyright 2023 Flant JSC

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

package check

// Utility code generating random episodes. Intended for tests

import (
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
)

func RandomEpisodes(n int) []Episode {
	var episodes []Episode
	for i := n; i > 0; i-- {
		episodes = append(episodes, RandomEpisode())
	}
	return episodes
}

func RandomEpisode() Episode {
	slotSize := 30 * time.Second
	ts := rand.Int63nRange(0, time.Now().Unix())
	slot := time.Unix(ts, 0).Truncate(slotSize)
	return NewEpisode(RandRef(), slot, slotSize, RandomStats())
}

func RandomStats() Stats {
	var (
		expected = 150
		up       = rand.Intn(expected)
		down     = rand.Intn(expected - up)
		unknown  = rand.Intn(expected - up - down)
	)

	return Stats{
		Expected: expected,
		Up:       up,
		Down:     down,
		Unknown:  unknown,
	}
}

func RandRef() ProbeRef {
	return ProbeRef{Group: rand.String(4), Probe: rand.String(7)}
}

func RandomEpisodesWithRef(n int, ref ProbeRef) []Episode {
	eps := RandomEpisodes(n)
	SetRef(eps, ref)
	return eps
}

func RandomEpisodesWithSlot(n int, slot time.Time) []Episode {
	eps := RandomEpisodes(n)
	SetSlot(eps, slot)
	return eps
}

func SetSlot(eps []Episode, slot time.Time) {
	for i := range eps {
		eps[i].TimeSlot = slot
	}
}

func SetRef(eps []Episode, ref ProbeRef) {
	for i := range eps {
		eps[i].ProbeRef = ref
	}
}

func ListReferences(eps []Episode) []*Episode {
	var refs []*Episode
	for i := range eps {
		refs = append(refs, &eps[i])
	}
	return refs
}
