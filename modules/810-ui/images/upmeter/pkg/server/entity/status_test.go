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

package entity

import (
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/server/ranges"
)

//nolint:unparam
func ref(group, probe string) check.ProbeRef {
	return check.ProbeRef{
		Group: group,
		Probe: probe,
	}
}

var (
	group = "TheGroup"
	probe = "TheProbe"

	probeRef  = ref(group, probe)
	probe2Ref = ref(group, probe+"2")
	totalRef  = ref(group, dao.GroupAggregation)
)

func Test_CalculateStatuses_success_only(t *testing.T) {
	g := NewWithT(t)

	episodeTime := counters{up: 300}
	episodes := []check.Episode{
		newEpisode(probeRef, 0, episodeTime),
		newEpisode(probeRef, 300, episodeTime),
		newEpisode(probeRef, 600, episodeTime),
		newEpisode(probeRef, 900, episodeTime),
	}

	t.Run("simple case with minimal step", func(t *testing.T) {
		s := calculateStatuses(episodes, nil, ranges.New5MinStepRange(0, 900, 300).Subranges, probeRef)

		// Len should be 4: 3 episodes + one total episode for a period.
		assertTree(g, s, probeRef, 3+1, "simple case with minimal step")
		assertTimers(g, s[group][probe][0], counters{up: 300})
		assertTimers(g, s[group][probe][1], counters{up: 300})
		assertTimers(g, s[group][probe][2], counters{up: 300})
		assertTimers(g, s[group][probe][3], counters{up: 300 * 3})
	})

	t.Run("mmm", func(t *testing.T) {
		s := calculateStatuses(episodes, nil, ranges.New5MinStepRange(0, 1200, 300).Subranges, probeRef)

		assertTree(g, s, probeRef, 4+1, "testGroup/testProbe len 4")
		assertTimers(g, s[group][probe][0], counters{up: 300})
		assertTimers(g, s[group][probe][1], counters{up: 300})
		assertTimers(g, s[group][probe][2], counters{up: 300})
		assertTimers(g, s[group][probe][3], counters{up: 300})
		assertTimers(g, s[group][probe][4], counters{up: 300 * 4})
	})

	t.Run("2x step", func(t *testing.T) {
		s := calculateStatuses(episodes, nil, ranges.New5MinStepRange(0, 1200, 600).Subranges, probeRef)

		assertTree(g, s, probeRef, 2+1, "testGroup/testProbe len 2")
		assertTimers(g, s[group][probe][0], counters{up: 600})
		assertTimers(g, s[group][probe][1], counters{up: 600})
		assertTimers(g, s[group][probe][2], counters{up: 2 * 600})
	})
}

func Test_TransformToSortedTimestampedArrays(t *testing.T) {
	g := NewWithT(t)

	statuses := map[string]map[string]map[int64]*EpisodeSummary{
		group: {
			"probe_1": {
				0:   &EpisodeSummary{TimeSlot: 0},
				300: &EpisodeSummary{TimeSlot: 300},
				600: &EpisodeSummary{TimeSlot: 600},
			},
			"probe_2": {
				0:   &EpisodeSummary{TimeSlot: 0},
				300: &EpisodeSummary{TimeSlot: 300},
				600: &EpisodeSummary{TimeSlot: 600},
			},
		},
		"testGroup_2": {
			"probe_1": {
				600: &EpisodeSummary{TimeSlot: 600},
				300: &EpisodeSummary{TimeSlot: 300},
				0:   &EpisodeSummary{TimeSlot: 0},
			},
			"probe_2": {
				300: &EpisodeSummary{TimeSlot: 300},
				600: &EpisodeSummary{TimeSlot: 600},
				0:   &EpisodeSummary{TimeSlot: 0},
			},
		},
	}

	sorted := transformTimestampedMapsToSortedArrays(statuses, ref(group, ""))
	// Structure should be without __total__ probe
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey(group))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup := sorted[group]
	g.Expect(testGroup).ShouldNot(HaveKey(dao.GroupAggregation))
	g.Expect(testGroup).Should(HaveKey("probe_1"))
	g.Expect(testGroup).Should(HaveKey("probe_2"))

	testGroup2 := sorted["testGroup_2"]
	g.Expect(testGroup2).ShouldNot(HaveKey(dao.GroupAggregation))
	g.Expect(testGroup2).Should(HaveKey("probe_1"))
	g.Expect(testGroup2).Should(HaveKey("probe_2"))

	// Check sorting
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_2"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_2"]))).Should(BeTrue())

	sorted = transformTimestampedMapsToSortedArrays(statuses, totalRef)
	// Structure should be with __total__ probe only
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey(group))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup = sorted[group]
	g.Expect(testGroup).Should(HaveLen(1))

	testGroup2 = sorted["testGroup_2"]
	g.Expect(testGroup2).Should(HaveLen(1))
}

// Helpers

type counters struct {
	up, down, unknown, muted, nodata int64
}

type timers struct {
	up, down, unknown, muted, nodata time.Duration
}

func ctimers(c counters) timers {
	return timers{
		up:      time.Second * time.Duration(c.up),
		down:    time.Second * time.Duration(c.down),
		unknown: time.Second * time.Duration(c.unknown),
		muted:   time.Second * time.Duration(c.muted),
		nodata:  time.Second * time.Duration(c.nodata),
	}
}

func newEpisode(ref check.ProbeRef, ts int64, seconds counters) check.Episode {
	t := ctimers(seconds)
	return check.Episode{
		ProbeRef: ref,
		TimeSlot: time.Unix(ts, 0),
		Up:       t.up,
		Down:     t.down,
		Unknown:  t.unknown,
		NoData:   t.nodata,
	}
}

func newDowntimeIncident(start, end int64, affected ...string) check.DowntimeIncident {
	return check.DowntimeIncident{
		Start:        start,
		End:          end,
		Duration:     0,
		Type:         "Maintenance",
		Description:  "test",
		Affected:     affected,
		DowntimeName: "",
	}
}

func assertTimers(g *WithT, got EpisodeSummary, expected counters) {
	gotT := timers{
		up:      got.Up,
		down:    got.Down,
		unknown: got.Unknown,
		muted:   got.Muted,
		nodata:  got.NoData,
	}
	expectT := ctimers(expected)

	g.Expect(gotT).Should(Equal(expectT), "unexpected got counters, info: %+v", got)
}

func assertTree(g *WithT, s map[string]map[string][]EpisodeSummary, ref check.ProbeRef, len int, msg string) {
	group := ref.Group
	probe := ref.Probe

	g.Expect(s).ShouldNot(BeNil(), msg)
	g.Expect(s).Should(HaveKey(group), msg)
	g.Expect(s[group]).Should(HaveKey(probe), msg)
	g.Expect(s[group][probe]).Should(HaveLen(len), msg)
}

func setInfoTime(info *EpisodeSummary, c counters) {
	t := ctimers(c)
	info.Up = t.up
	info.Down = t.down
	info.Unknown = t.unknown
	info.Muted = t.muted
	info.NoData = t.nodata
}
