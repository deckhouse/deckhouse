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
	return NewEpisode(RandRef(), slot, slotSize, Stats{})
}

func RandRef() ProbeRef {
	return ProbeRef{Group: rand.String(4), Probe: rand.String(7)}
}

func RandomEpisodesWithRef(n int, ref ProbeRef) []Episode {
	eps := RandomEpisodes(n)
	for _, e := range eps {
		e := e
		e.ProbeRef = ref
	}
	return eps
}

func RandomEpisodesWithSlot(n int, slot time.Time) []Episode {
	eps := RandomEpisodes(n)
	for i := range eps {
		eps[i].TimeSlot = slot
	}
	return eps
}

func ListReferences(eps []Episode) []*Episode {
	var refs []*Episode
	for i := range eps {
		refs = append(refs, &eps[i])
	}
	return refs
}
