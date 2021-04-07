package entity

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/check"
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
	totalRef  = ref(group, totalProbeName)
)

func Test_CalculateStatuses_success_only(t *testing.T) {
	g := NewWithT(t)

	episodeTime := counters{up: 300}
	episodes := []check.DowntimeEpisode{
		newDowntimeEpisode(probeRef, 0, episodeTime),
		newDowntimeEpisode(probeRef, 300, episodeTime),
		newDowntimeEpisode(probeRef, 600, episodeTime),
		newDowntimeEpisode(probeRef, 900, episodeTime),
	}

	t.Run("simple case with minimal step", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, probeRef)

		// Len should be 4: 3 episodes + one total episode for a period.
		assertTree(g, s, probeRef, 3+1, "simple case with minimal step")
		assertCounters(g, s[group][probe][0], counters{up: 300})
		assertCounters(g, s[group][probe][1], counters{up: 300})
		assertCounters(g, s[group][probe][2], counters{up: 300})
		assertCounters(g, s[group][probe][3], counters{up: 300 * 3})
	})

	t.Run("mmm", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, probeRef)

		assertTree(g, s, probeRef, 4+1, "testGroup/testProbe len 4")
		assertCounters(g, s[group][probe][0], counters{up: 300})
		assertCounters(g, s[group][probe][1], counters{up: 300})
		assertCounters(g, s[group][probe][2], counters{up: 300})
		assertCounters(g, s[group][probe][3], counters{up: 300})
		assertCounters(g, s[group][probe][4], counters{up: 300 * 4})
	})

	t.Run("simple case, total seconds for group", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, totalRef)

		assertTree(g, s, totalRef, 3+1, "testGroup/__total__ len 4")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][2], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][3], counters{up: 3 * 300})
	})

	t.Run("simple case, total seconds for group", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, totalRef)

		assertTree(g, s, totalRef, 4+1, "testGroup/__total__ len 4 2")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][2], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][3], counters{up: 300})
		assertCounters(g, s[group][totalProbeName][4], counters{up: 4 * 300})
	})

	t.Run("2x step", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, probeRef)

		assertTree(g, s, probeRef, 2+1, "testGroup/testProbe len 2")
		assertCounters(g, s[group][probe][0], counters{up: 600})
		assertCounters(g, s[group][probe][1], counters{up: 600})
		assertCounters(g, s[group][probe][2], counters{up: 2 * 600})
	})

	t.Run("3x step with grouping", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 900).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 900})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 900})
	})
}

func Test_CalculateStatuses_with_incidents(t *testing.T) {
	g := NewWithT(t)

	goodTimes := counters{up: 300}
	badTimes := counters{down: 200, unknown: 100}
	episodes := []check.DowntimeEpisode{
		newDowntimeEpisode(probeRef, 0, goodTimes),
		newDowntimeEpisode(probeRef, 300, goodTimes),
		newDowntimeEpisode(probeRef, 600, badTimes),
		newDowntimeEpisode(probeRef, 900, goodTimes),
	}

	incidents := []check.DowntimeIncident{
		newDowntimeIncident(250, 400, group),
		newDowntimeIncident(600, 800, group), // mute duration is 200 to mute unknown and a half of down
	}

	// 2x step with muting
	stepRanges := CalculateAdjustedStepRanges(0, 1200, 600)
	s := CalculateStatuses(episodes, incidents, stepRanges.Ranges, totalRef)
	statusInfos := s[group][totalProbeName]

	g.Expect(statusInfos).To(HaveLen(3))

	assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2")

	// All Up is not muted
	assertCounters(g, statusInfos[0], counters{up: 600})
	// unknown and down should be muted
	assertCounters(g, statusInfos[1], counters{up: 300, down: 100, muted: 200})

	// Last item is a Total for the period.
	assertCounters(g, statusInfos[2], counters{up: 600 + 300, down: 100, muted: 200})

}

// Test UpdateMute with Nodata in episodes
func Test_CalculateStatuses_with_incidents_and_nodata(t *testing.T) {
	g := NewWithT(t)

	episodes := []check.DowntimeEpisode{
		newDowntimeEpisode(probeRef, 300, counters{up: 100, unknown: 200}),
		newDowntimeEpisode(probeRef, 900, counters{up: 100, down: 100, unknown: 100}),
	}

	t.Run("should add all statuses when no incidents", func(t *testing.T) {
		// 2x step with nodata
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 100, unknown: 200, nodata: 300})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 100, down: 100, unknown: 100, nodata: 300})
		// Total for the period
		assertCounters(g, s[group][totalProbeName][2], counters{up: 200, down: 100, unknown: 200 + 100, nodata: 300 + 300})
	})

	t.Run("incidents should not decrease Up seconds", func(t *testing.T) {
		// 2x step with muting
		incidents := []check.DowntimeIncident{
			newDowntimeIncident(250, 400, group),
			newDowntimeIncident(800, 950, group),
		}

		s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2 2")

		assertCounters(g, s[group][totalProbeName][0], counters{up: 100, unknown: 50, muted: 150, nodata: 300})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 100, down: 50, muted: 150, nodata: 300})
		assertCounters(g, s[group][totalProbeName][2], counters{up: 100 + 100, down: 50, unknown: 50, muted: 150 + 150, nodata: 300 + 300})
	})

	t.Run("incidents should decrease NoData if mute is more than KnownSeconds and should not decrease Up seconds", func(t *testing.T) {
		// 2x step with muting
		// Increase incidents to test NoData decreasing
		incidents := []check.DowntimeIncident{
			newDowntimeIncident(100, 600, group),
			newDowntimeIncident(700, 1400, group),
		}

		s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2 3")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 100, muted: 400, nodata: 100})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 100, muted: 400, nodata: 100})
		assertCounters(g, s[group][totalProbeName][2], counters{up: 100 + 100, muted: 400 + 400, nodata: 100 + 100})
	})
}

// Test CalculateTotalForStepRange
func Test_CalculateStatuses_total_with_multiple_probes(t *testing.T) {
	g := NewWithT(t)

	var episodes []check.DowntimeEpisode
	var s map[string]map[string][]StatusInfo

	t.Run("Only success and unknown should not emit down seconds", func(t *testing.T) {

		time1 := counters{up: 50, unknown: 250}
		time2 := counters{up: 100, unknown: 200}
		episodes = []check.DowntimeEpisode{
			newDowntimeEpisode(probeRef, 0, time1),
			newDowntimeEpisode(probeRef, 300, time1),
			newDowntimeEpisode(probe2Ref, 0, time2),
			newDowntimeEpisode(probe2Ref, 300, time2),
		}

		s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 100, unknown: 500})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 100, unknown: 500})

	})

	t.Run("Only success and nodata should not emit down seconds", func(t *testing.T) {
		time1 := counters{up: 50, nodata: 250}
		time2 := counters{up: 100, nodata: 200}
		episodes := []check.DowntimeEpisode{
			newDowntimeEpisode(probeRef, 0, time1),
			newDowntimeEpisode(probeRef, 300, time1),
			newDowntimeEpisode(probe2Ref, 0, time2),
			newDowntimeEpisode(probe2Ref, 300, time2),
		}

		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1 2")
		assertCounters(g, s[group][totalProbeName][0], counters{up: 100, unknown: 100, nodata: 400})
		assertCounters(g, s[group][totalProbeName][1], counters{up: 100, unknown: 100, nodata: 400})
	})

}

func Test_TransformToSortedTimestampedArrays(t *testing.T) {
	g := NewWithT(t)

	statuses := map[string]map[string]map[int64]*StatusInfo{
		group: {
			totalProbeName: {
				0:   &StatusInfo{TimeSlot: 0},
				300: &StatusInfo{TimeSlot: 300},
				600: &StatusInfo{TimeSlot: 600},
			},
			"probe_1": {
				0:   &StatusInfo{TimeSlot: 0},
				300: &StatusInfo{TimeSlot: 300},
				600: &StatusInfo{TimeSlot: 600},
			},
			"probe_2": {
				0:   &StatusInfo{TimeSlot: 0},
				300: &StatusInfo{TimeSlot: 300},
				600: &StatusInfo{TimeSlot: 600},
			},
		},
		"testGroup_2": {
			totalProbeName: {
				0:   &StatusInfo{TimeSlot: 0},
				600: &StatusInfo{TimeSlot: 600},
				300: &StatusInfo{TimeSlot: 300},
			},
			"probe_1": {
				600: &StatusInfo{TimeSlot: 600},
				300: &StatusInfo{TimeSlot: 300},
				0:   &StatusInfo{TimeSlot: 0},
			},
			"probe_2": {
				300: &StatusInfo{TimeSlot: 300},
				600: &StatusInfo{TimeSlot: 600},
				0:   &StatusInfo{TimeSlot: 0},
			},
		},
	}

	sorted := TransformTimestampedMapsToSortedArrays(statuses, ref(group, ""))
	// Structure should be without __total__ probe
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey(group))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup := sorted[group]
	g.Expect(testGroup).ShouldNot(HaveKey(totalProbeName))
	g.Expect(testGroup).Should(HaveKey("probe_1"))
	g.Expect(testGroup).Should(HaveKey("probe_2"))

	testGroup2 := sorted["testGroup_2"]
	g.Expect(testGroup2).ShouldNot(HaveKey(totalProbeName))
	g.Expect(testGroup2).Should(HaveKey("probe_1"))
	g.Expect(testGroup2).Should(HaveKey("probe_2"))

	// Check sorting
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_2"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_2"]))).Should(BeTrue())

	sorted = TransformTimestampedMapsToSortedArrays(statuses, totalRef)
	// Structure should be with __total__ probe only
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey(group))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup = sorted[group]
	g.Expect(testGroup).Should(HaveLen(1))
	g.Expect(testGroup).Should(HaveKey(totalProbeName))

	testGroup2 = sorted["testGroup_2"]
	g.Expect(testGroup2).Should(HaveLen(1))
	g.Expect(testGroup2).Should(HaveKey(totalProbeName))

	// Check sorting
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup[totalProbeName]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2[totalProbeName]))).Should(BeTrue())
}

func Test_CalculateTotalForStepRange(t *testing.T) {
	g := NewWithT(t)

	stepRange := check.Range{From: 0, To: 300}

	infos := []*StatusInfo{
		{Up: 300},
		{Down: 300},
		{Unknown: 300},
	}

	statuses := map[string]map[string]map[int64]*StatusInfo{
		group: {
			"testProbe1": {0: infos[0]},
			"testProbe2": {0: infos[1]},
			"testProbe3": {0: infos[2]},
		},
	}

	CalculateTotalForStepRange(statuses, stepRange)

	g.Expect(statuses[group]).Should(HaveKey(totalProbeName))
	totalInfo := statuses[group][totalProbeName][0]

	g.Expect(totalInfo.NoData).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	// 2.
	infos[0].SetSeconds(30, 200, 0, 70)
	infos[1].SetSeconds(50, 150, 0, 100)
	infos[2].SetSeconds(10, 0, 100, 190)

	CalculateTotalForStepRange(statuses, stepRange)
	g.Expect(statuses[group]).Should(HaveKey(totalProbeName))
	totalInfo = statuses[group][totalProbeName][0]

	g.Expect(totalInfo.Up).Should(BeEquivalentTo(10))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(200))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(20))
	g.Expect(totalInfo.NoData).Should(BeEquivalentTo(70))
}

// Test with episodes for the same probe and the same timeslot.
func Test_CalculateStatuses_multi_episodes(t *testing.T) {
	g := NewWithT(t)

	episodes := []check.DowntimeEpisode{
		newDowntimeEpisode(probeRef, 0, counters{up: 300}),
		newDowntimeEpisode(probeRef, 0, counters{up: 100, down: 200}),
		newDowntimeEpisode(probeRef, 0, counters{up: 50, down: 25, unknown: 225}),
	}

	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 300, 300).Ranges, totalRef)

	assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
	assertCounters(g, s[group][totalProbeName][0], counters{up: 300})
	assertCounters(g, s[group][totalProbeName][1], counters{up: 300})
}

// Helpers

type counters struct {
	up, down, unknown, muted, nodata int64
}

func newDowntimeEpisode(ref check.ProbeRef, ts int64, seconds counters) check.DowntimeEpisode {
	return check.DowntimeEpisode{
		ProbeRef:       ref,
		TimeSlot:       ts,
		SuccessSeconds: seconds.up,
		FailSeconds:    seconds.down,
		UnknownSeconds: seconds.unknown,
		NoDataSeconds:  seconds.nodata,
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

func assertCounters(g *WithT, info StatusInfo, c counters) {
	got := counters{}

	got.up = info.Up
	got.down = info.Down
	got.unknown = info.Unknown
	got.muted = info.Muted
	got.nodata = info.NoData

	g.Expect(got).Should(Equal(c), "unexpected info counters, info: %+v", info)
}

func assertTree(g *WithT, s map[string]map[string][]StatusInfo, ref check.ProbeRef, len int, msg string) {
	group := ref.Group
	probe := ref.Probe

	g.Expect(s).ShouldNot(BeNil(), msg)
	g.Expect(s).Should(HaveKey(group), msg)
	g.Expect(s[group]).Should(HaveKey(probe), msg)
	g.Expect(s[group][probe]).Should(HaveLen(len), msg)
}
