package entity

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/probe/types"
)

func Test_CalculateStatuses_success_only(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 0, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 300, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 600, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 900, 300, 0, 0, 0),
	}

	// simple case with minimal step
	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "testProbe")

	ExpectStatuses(g, s, "testGroup", "testProbe", 3)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][0], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][1], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][2], 300, 0, 0, 0, 0)

	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, "testGroup", "testProbe")

	ExpectStatuses(g, s, "testGroup", "testProbe", 4)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][0], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][1], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][2], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][3], 300, 0, 0, 0, 0)

	// simple case, total seconds for group
	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 3)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][2], 300, 0, 0, 0, 0)

	// simple case, total seconds for group
	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 300).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 4)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][2], 300, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][3], 300, 0, 0, 0, 0)

	// 2x step
	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, "testGroup", "testProbe")

	ExpectStatuses(g, s, "testGroup", "testProbe", 2)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][0], 600, 0, 0, 0, 0)
	ExpectStatusInfo(g, s["testGroup"]["testProbe"][1], 600, 0, 0, 0, 0)

	// 3x step with grouping
	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 900, 900).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 1)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 900, 0, 0, 0, 0)
}

func Test_CalculateStatuses_with_incidents(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 0, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 300, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 600, 0, 200, 100, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 900, 300, 0, 0, 0),
	}

	incidents := []types.DowntimeIncident{
		NewDowntimeIncident(250, 400, "testGroup"),
		NewDowntimeIncident(600, 800, "testGroup"), // mute duration is 200 to mute unknown and a half of down
	}

	// 2x step with muting
	s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 2)
	// All Up is not muted
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 600, 0, 0, 0, 0)
	// unknown and down should be muted
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 300, 100, 0, 200, 0)

}

// Test UpdateMute with Nodata in episodes
func Test_CalculateStatuses_with_incidents_and_nodata(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 300, 100, 0, 200, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 900, 100, 100, 100, 0),
	}

	// 2x step with nodata
	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 2)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 100, 0, 200, 0, 300)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 100, 100, 100, 0, 300)

	incidents := []types.DowntimeIncident{
		NewDowntimeIncident(250, 400, "testGroup"),
		NewDowntimeIncident(800, 950, "testGroup"),
	}

	// 2x step with muting
	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 2)

	// incidents should not  decrease Up seconds
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 100, 0, 50, 150, 300)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 100, 50, 0, 150, 300)

	// Increase incidents to test NoData decreasing
	incidents = []types.DowntimeIncident{
		NewDowntimeIncident(100, 600, "testGroup"),
		NewDowntimeIncident(700, 1400, "testGroup"),
	}

	// 2x step with muting
	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 1200, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 2)
	// incidents should decrease NoData if mute is more than KnownSeconds and should not decrease Up seconds
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 100, 0, 0, 400, 100)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][1], 100, 0, 0, 400, 100)
}

// Test CalculateTotalForStepRange
func Test_CalculateStatuses_total_with_multiple_probes(t *testing.T) {
	g := NewWithT(t)

	var episodes []types.DowntimeEpisode
	var s map[string]map[string][]StatusInfo

	// Only success and unknown should not emit down seconds
	episodes = []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 0, 50, 0, 250, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 300, 50, 0, 250, 0),
		NewDowntimeEpisode("testGroup", "testProbe2", 0, 100, 0, 200, 0),
		NewDowntimeEpisode("testGroup", "testProbe2", 300, 100, 0, 200, 0),
	}

	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 1)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 100, 0, 500, 0, 0)

	// Only success and nodata should not emit down seconds
	episodes = []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 0, 50, 0, 0, 250),
		NewDowntimeEpisode("testGroup", "testProbe", 300, 50, 0, 0, 250),
		NewDowntimeEpisode("testGroup", "testProbe2", 0, 100, 0, 0, 200),
		NewDowntimeEpisode("testGroup", "testProbe2", 300, 100, 0, 0, 200),
	}

	s = CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 600, 600).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 1)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 100, 0, 100, 0, 400)

}

func Test_TransformToSortedTimestampedArrays(t *testing.T) {
	g := NewWithT(t)

	statuses := map[string]map[string]map[int64]*StatusInfo{
		"testGroup": {
			"__total__": {
				0: &StatusInfo{
					TimeSlot: 0,
				},
				300: &StatusInfo{
					TimeSlot: 300,
				},
				600: &StatusInfo{
					TimeSlot: 600,
				},
			},
			"probe_1": {
				0: &StatusInfo{
					TimeSlot: 0,
				},
				300: &StatusInfo{
					TimeSlot: 300,
				},
				600: &StatusInfo{
					TimeSlot: 600,
				},
			},
			"probe_2": {
				0: &StatusInfo{
					TimeSlot: 0,
				},
				300: &StatusInfo{
					TimeSlot: 300,
				},
				600: &StatusInfo{
					TimeSlot: 600,
				},
			},
		},
		"testGroup_2": {
			"__total__": {
				0: &StatusInfo{
					TimeSlot: 0,
				},
				600: &StatusInfo{
					TimeSlot: 600,
				},
				300: &StatusInfo{
					TimeSlot: 300,
				},
			},
			"probe_1": {
				600: &StatusInfo{
					TimeSlot: 600,
				},
				300: &StatusInfo{
					TimeSlot: 300,
				},
				0: &StatusInfo{
					TimeSlot: 0,
				},
			},
			"probe_2": {
				300: &StatusInfo{
					TimeSlot: 300,
				},
				600: &StatusInfo{
					TimeSlot: 600,
				},
				0: &StatusInfo{
					TimeSlot: 0,
				},
			},
		},
	}

	sorted := TransformTimestampedMapsToSortedArrays(statuses, "testGroup", "")
	// Structure should be without __total__ probe
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey("testGroup"))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup := sorted["testGroup"]
	g.Expect(testGroup).ShouldNot(HaveKey("__total__"))
	g.Expect(testGroup).Should(HaveKey("probe_1"))
	g.Expect(testGroup).Should(HaveKey("probe_2"))

	testGroup2 := sorted["testGroup_2"]
	g.Expect(testGroup2).ShouldNot(HaveKey("__total__"))
	g.Expect(testGroup2).Should(HaveKey("probe_1"))
	g.Expect(testGroup2).Should(HaveKey("probe_2"))

	// Check sorting
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["probe_2"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_1"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["probe_2"]))).Should(BeTrue())

	sorted = TransformTimestampedMapsToSortedArrays(statuses, "testGroup", "__total__")
	// Structure should be with __total__ probe only
	g.Expect(sorted).Should(HaveLen(2))
	g.Expect(sorted).Should(HaveKey("testGroup"))
	g.Expect(sorted).Should(HaveKey("testGroup_2"))

	testGroup = sorted["testGroup"]
	g.Expect(testGroup).Should(HaveLen(1))
	g.Expect(testGroup).Should(HaveKey("__total__"))

	testGroup2 = sorted["testGroup_2"]
	g.Expect(testGroup2).Should(HaveLen(1))
	g.Expect(testGroup2).Should(HaveKey("__total__"))

	// Check sorting
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup["__total__"]))).Should(BeTrue())
	g.Expect(sort.IsSorted(ByTimeSlot(testGroup2["__total__"]))).Should(BeTrue())
}

func Test_CalculateTotalForStepRange(t *testing.T) {
	g := NewWithT(t)

	stepRange := []int64{0, 300}

	infos := []*StatusInfo{
		&StatusInfo{
			TimeSlot: 0,
			Up:       300,
			Down:     0,
			Unknown:  0,
			Muted:    0,
			NoData:   0,
		},
		&StatusInfo{
			TimeSlot: 0,
			Up:       0,
			Down:     300,
			Unknown:  0,
			Muted:    0,
			NoData:   0,
		},
		&StatusInfo{
			TimeSlot: 0,
			Up:       0,
			Down:     0,
			Unknown:  300,
			Muted:    0,
			NoData:   0,
		},
	}

	statuses := map[string]map[string]map[int64]*StatusInfo{
		"testGroup": {
			"testProbe1": {
				0: infos[0],
			},
			"testProbe2": {
				0: infos[1],
			},
			"testProbe3": {
				0: infos[2],
			},
		},
	}

	CalculateTotalForStepRange(statuses, stepRange)

	g.Expect(statuses["testGroup"]).Should(HaveKey("__total__"))
	totalInfo := statuses["testGroup"]["__total__"][0]

	g.Expect(totalInfo.NoData).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	// 2.
	infos[0].SetSeconds(30, 200, 0, 70)
	infos[1].SetSeconds(50, 150, 0, 100)
	infos[2].SetSeconds(10, 0, 100, 190)

	CalculateTotalForStepRange(statuses, stepRange)
	g.Expect(statuses["testGroup"]).Should(HaveKey("__total__"))
	totalInfo = statuses["testGroup"]["__total__"][0]

	g.Expect(totalInfo.Up).Should(BeEquivalentTo(10))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(220))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.NoData).Should(BeEquivalentTo(70))
}

// Test with episodes for the same probe and the same timeslot.
func Test_CalculateStatuses_multi_episodes(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		NewDowntimeEpisode("testGroup", "testProbe", 0, 300, 0, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 0, 100, 200, 0, 0),
		NewDowntimeEpisode("testGroup", "testProbe", 0, 50, 25, 300-50-25, 0),
	}

	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 300, 300).Ranges, "testGroup", "__total__")

	ExpectStatuses(g, s, "testGroup", "__total__", 1)
	ExpectStatusInfo(g, s["testGroup"]["__total__"][0], 300, 0, 0, 0, 0)
}

// Helpers

func NewDowntimeEpisode(group, probe string, ts, success, fail, unknown, nodata int64) types.DowntimeEpisode {
	return types.DowntimeEpisode{
		ProbeRef: types.ProbeRef{
			Group: group,
			Probe: probe,
		},
		TimeSlot:       ts,
		SuccessSeconds: success,
		FailSeconds:    fail,
		Unknown:        unknown,
		NoData:         nodata,
	}
}

func NewDowntimeIncident(start, end int64, affected ...string) types.DowntimeIncident {
	return types.DowntimeIncident{
		Start:        start,
		End:          end,
		Duration:     0,
		Type:         "Maintenance",
		Description:  "test",
		Affected:     affected,
		DowntimeName: "",
	}
}

func ExpectStatusInfo(g *WithT, status StatusInfo, up, down, unknown, muted, nodata int64) {
	if up >= 0 {
		g.Expect(status.Up).Should(BeEquivalentTo(up), "Check info.Up, info: %+v", status)
	}
	if down >= 0 {
		g.Expect(status.Down).Should(BeEquivalentTo(down), "Check info.Down, info: %+v", status)
	}
	if unknown >= 0 {
		g.Expect(status.Unknown).Should(BeEquivalentTo(unknown), "Check info.Unknown, info: %+v", status)
	}
	if muted >= 0 {
		g.Expect(status.Muted).Should(BeEquivalentTo(muted), "Check info.Muted, info: %+v", status)
	}
	if nodata >= 0 {
		g.Expect(status.NoData).Should(BeEquivalentTo(nodata), "Check info.NoData, info: %+v", status)
	}

}

func ExpectStatuses(g *WithT, s map[string]map[string][]StatusInfo, group, probe string, len int) {
	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey(group))
	g.Expect(s[group]).Should(HaveKey(probe))
	g.Expect(s[group][probe]).Should(HaveLen(len))
}
