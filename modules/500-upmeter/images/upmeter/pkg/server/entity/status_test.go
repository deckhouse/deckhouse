package entity

import (
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
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
	episodes := []check.Episode{
		newEpisode(probeRef, 0, episodeTime),
		newEpisode(probeRef, 300, episodeTime),
		newEpisode(probeRef, 600, episodeTime),
		newEpisode(probeRef, 900, episodeTime),
	}

	t.Run("simple case with minimal step", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, probeRef)

		// Len should be 4: 3 episodes + one total episode for a period.
		assertTree(g, s, probeRef, 3+1, "simple case with minimal step")
		assertTimers(g, s[group][probe][0], counters{up: 300})
		assertTimers(g, s[group][probe][1], counters{up: 300})
		assertTimers(g, s[group][probe][2], counters{up: 300})
		assertTimers(g, s[group][probe][3], counters{up: 300 * 3})
	})

	t.Run("mmm", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, probeRef)

		assertTree(g, s, probeRef, 4+1, "testGroup/testProbe len 4")
		assertTimers(g, s[group][probe][0], counters{up: 300})
		assertTimers(g, s[group][probe][1], counters{up: 300})
		assertTimers(g, s[group][probe][2], counters{up: 300})
		assertTimers(g, s[group][probe][3], counters{up: 300})
		assertTimers(g, s[group][probe][4], counters{up: 300 * 4})
	})

	t.Run("simple case, total seconds for group", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, totalRef)

		assertTree(g, s, totalRef, 3+1, "testGroup/__total__ len 4")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][2], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][3], counters{up: 3 * 300})
	})

	t.Run("simple case, total seconds for group", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, totalRef)

		assertTree(g, s, totalRef, 4+1, "testGroup/__total__ len 4 2")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][2], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][3], counters{up: 300})
		assertTimers(g, s[group][totalProbeName][4], counters{up: 4 * 300})
	})

	t.Run("2x step", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, probeRef)

		assertTree(g, s, probeRef, 2+1, "testGroup/testProbe len 2")
		assertTimers(g, s[group][probe][0], counters{up: 600})
		assertTimers(g, s[group][probe][1], counters{up: 600})
		assertTimers(g, s[group][probe][2], counters{up: 2 * 600})
	})

	t.Run("3x step with grouping", func(t *testing.T) {
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 900).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 900})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 900})
	})
}

func Test_CalculateStatuses_with_incidents(t *testing.T) {
	g := NewWithT(t)

	goodTimes := counters{up: 300}
	badTimes := counters{down: 200, unknown: 100}
	episodes := []check.Episode{
		newEpisode(probeRef, 0, goodTimes),
		newEpisode(probeRef, 300, goodTimes),
		newEpisode(probeRef, 600, badTimes),
		newEpisode(probeRef, 900, goodTimes),
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
	assertTimers(g, statusInfos[0], counters{up: 600})
	// unknown and down should be muted
	assertTimers(g, statusInfos[1], counters{up: 300, down: 100, muted: 200})

	// Last item is a Total for the period.
	assertTimers(g, statusInfos[2], counters{up: 600 + 300, down: 100, muted: 200})
}

// Test UpdateMute with Nodata in episodes
func Test_CalculateStatuses_with_incidents_and_nodata(t *testing.T) {
	g := NewWithT(t)

	episodes := []check.Episode{
		newEpisode(probeRef, 300, counters{up: 100, unknown: 200}),
		newEpisode(probeRef, 900, counters{up: 100, down: 100, unknown: 100}),
	}

	t.Run("should add all statuses when no incidents", func(t *testing.T) {
		// 2x step with nodata
		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 100, unknown: 200, nodata: 300})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 100, down: 100, unknown: 100, nodata: 300})
		// Total for the period
		assertTimers(g, s[group][totalProbeName][2], counters{up: 200, down: 100, unknown: 200 + 100, nodata: 300 + 300})
	})

	t.Run("incidents should not decrease Up seconds", func(t *testing.T) {
		// 2x step with muting
		incidents := []check.DowntimeIncident{
			newDowntimeIncident(250, 400, group),
			newDowntimeIncident(800, 950, group),
		}

		s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 2+1, "testGroup/__total__ len 2 2")

		assertTimers(g, s[group][totalProbeName][0], counters{up: 100, unknown: 50, muted: 150, nodata: 300})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 100, down: 50, muted: 150, nodata: 300})
		assertTimers(g, s[group][totalProbeName][2], counters{up: 100 + 100, down: 50, unknown: 50, muted: 150 + 150, nodata: 300 + 300})
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
		assertTimers(g, s[group][totalProbeName][0], counters{up: 100, muted: 400, nodata: 100})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 100, muted: 400, nodata: 100})
		assertTimers(g, s[group][totalProbeName][2], counters{up: 100 + 100, muted: 400 + 400, nodata: 100 + 100})
	})
}

// Test CalculateTotalForStepRange
func Test_CalculateStatuses_total_with_multiple_probes(t *testing.T) {
	g := NewWithT(t)

	var episodes []check.Episode
	var s map[string]map[string][]StatusInfo

	t.Run("Only success and unknown should not emit down seconds", func(t *testing.T) {
		time1 := counters{up: 50, unknown: 250}
		time2 := counters{up: 100, unknown: 200}
		episodes = []check.Episode{
			newEpisode(probeRef, 0, time1),
			newEpisode(probeRef, 300, time1),
			newEpisode(probe2Ref, 0, time2),
			newEpisode(probe2Ref, 300, time2),
		}

		s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 100, unknown: 500})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 100, unknown: 500})
	})

	t.Run("Only success and nodata should not emit down seconds", func(t *testing.T) {
		time1 := counters{up: 50, nodata: 250}
		time2 := counters{up: 100, nodata: 200}
		episodes := []check.Episode{
			newEpisode(probeRef, 0, time1),
			newEpisode(probeRef, 300, time1),
			newEpisode(probe2Ref, 0, time2),
			newEpisode(probe2Ref, 300, time2),
		}

		s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, totalRef)

		assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1 2")
		assertTimers(g, s[group][totalProbeName][0], counters{up: 100, unknown: 100, nodata: 400})
		assertTimers(g, s[group][totalProbeName][1], counters{up: 100, unknown: 100, nodata: 400})
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
		{Up: 300 * time.Second},
		{Down: 300 * time.Second},
		{Unknown: 300 * time.Second},
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

	assertTimers(g, *totalInfo, counters{down: 300})

	// 2.
	setInfoTime(infos[0], counters{up: 30, down: 200, nodata: 70})
	setInfoTime(infos[1], counters{up: 50, down: 150, nodata: 100})
	setInfoTime(infos[2], counters{up: 10, unknown: 100, nodata: 190})

	CalculateTotalForStepRange(statuses, stepRange)
	g.Expect(statuses[group]).Should(HaveKey(totalProbeName))
	totalInfo = statuses[group][totalProbeName][0]

	assertTimers(g, *totalInfo, counters{up: 10, down: 200, unknown: 20, nodata: 70})
}

// Test with episodes for the same probe and the same timeslot.
func Test_CalculateStatuses_multi_episodes(t *testing.T) {
	g := NewWithT(t)

	episodes := []check.Episode{
		newEpisode(probeRef, 0, counters{up: 300}),
		newEpisode(probeRef, 0, counters{up: 100, down: 200}),
		newEpisode(probeRef, 0, counters{up: 50, down: 25, unknown: 225}),
	}

	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 300, 300).Ranges, totalRef)

	assertTree(g, s, totalRef, 1+1, "testGroup/__total__ len 1")
	assertTimers(g, s[group][totalProbeName][0], counters{up: 300})
	assertTimers(g, s[group][totalProbeName][1], counters{up: 300})
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

func assertTimers(g *WithT, got StatusInfo, expected counters) {
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

func assertTree(g *WithT, s map[string]map[string][]StatusInfo, ref check.ProbeRef, len int, msg string) {
	group := ref.Group
	probe := ref.Probe

	g.Expect(s).ShouldNot(BeNil(), msg)
	g.Expect(s).Should(HaveKey(group), msg)
	g.Expect(s[group]).Should(HaveKey(probe), msg)
	g.Expect(s[group][probe]).Should(HaveLen(len), msg)
}

func setInfoTime(info *StatusInfo, c counters) {
	t := ctimers(c)
	info.Up = t.up
	info.Down = t.down
	info.Unknown = t.unknown
	info.Muted = t.muted
	info.NoData = t.nodata
}
