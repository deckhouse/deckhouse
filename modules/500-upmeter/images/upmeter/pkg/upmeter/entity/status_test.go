package entity

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/probe/types"
)

func Test_CalculateStatuses(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       300,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       600,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       900,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
	}

	incidents := []types.DowntimeIncident{}

	// simple case with minimal step

	s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "testProbe")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("testProbe"))
	g.Expect(s["testGroup"]["testProbe"]).Should(HaveLen(3))
	g.Expect(s["testGroup"]["testProbe"][0].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][1].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][2].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][0].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][2].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][0].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][2].Down).Should(BeEquivalentTo(0))

	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(3))
	totalInfo := s["testGroup"]["__total__"][0]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	totalInfo = s["testGroup"]["__total__"][1]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	totalInfo = s["testGroup"]["__total__"][2]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	// 2x step

	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 600).Ranges, "testGroup", "testProbe")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("testProbe"))
	g.Expect(s["testGroup"]["testProbe"]).Should(HaveLen(2))
	info := s["testGroup"]["testProbe"][0]
	g.Expect(info.TimeSlot).Should(BeEquivalentTo(0))
	g.Expect(info.Up).Should(BeEquivalentTo(600))
	g.Expect(info.Down).Should(BeEquivalentTo(0))
	g.Expect(info.Unknown).Should(BeEquivalentTo(0))
	info = s["testGroup"]["testProbe"][1]
	g.Expect(info.TimeSlot).Should(BeEquivalentTo(600))
	g.Expect(info.Up).Should(BeEquivalentTo(600))
	g.Expect(info.Down).Should(BeEquivalentTo(0))
	g.Expect(info.Unknown).Should(BeEquivalentTo(0))

	// 3x step with grouping
	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 900).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(1))
	totalInfo = s["testGroup"]["__total__"][0]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(900))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	//g.Expect(s["testGroup"]["__total__"][0].Up).Should(BeEquivalentTo(900))
	//g.Expect(s["testGroup"]["__total__"][1].Up).Should(BeEquivalentTo(300))
	//totalInfo = s["testGroup"]["__total__"][1]

	// incidents!
}

func Test_CalculateStatuses_with_incidents(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       300,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       600,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       900,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
	}

	incidents := []types.DowntimeIncident{
		{
			Start:        250,
			End:          400,
			Duration:     0,
			Type:         "Maintenance",
			Description:  "test",
			Affected:     []string{"testGroup"},
			DowntimeName: "",
		},
		{
			Start:        800,
			End:          900,
			Duration:     0,
			Type:         "Maintenance",
			Description:  "test",
			Affected:     []string{"testGroup"},
			DowntimeName: "",
		},
	}

	// 2x step with muting
	s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 600).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(2))
	// incidents should not  decrease Up seconds
	g.Expect(s["testGroup"]["__total__"][0].Up).Should(BeEquivalentTo(600))
	g.Expect(s["testGroup"]["__total__"][1].Up).Should(BeEquivalentTo(600))

	// 2x step with muting Down seconds
	episodes = []types.DowntimeEpisode{
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "probe_1",
			},
			TimeSlot:       0,
			FailSeconds:    100,
			SuccessSeconds: 100,
			Unknown:        100,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "probe_2",
			},
			TimeSlot:       0,
			FailSeconds:    100,
			SuccessSeconds: 100,
			Unknown:        100,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "probe_1",
			},
			TimeSlot:       300,
			FailSeconds:    100,
			SuccessSeconds: 200,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "probe_1",
			},
			TimeSlot:       600,
			FailSeconds:    100,
			SuccessSeconds: 100,
			Unknown:        100,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "probe_1",
			},
			TimeSlot:       900,
			FailSeconds:    100,
			SuccessSeconds: 200,
		},
	}

	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 600).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(2))
	// incidents should not  decrease Up seconds
	info := s["testGroup"]["__total__"][0]
	secondsSum := info.Up + info.Down + info.Muted + info.Unknown
	g.Expect(secondsSum).Should(BeEquivalentTo(600))
	g.Expect(info.Up).Should(BeEquivalentTo(100), "info=%+v", info) // 300 for probe_1 and 100 for probe_2 == 100
	g.Expect(info.Down).Should(BeEquivalentTo(250))                 // 500 known seconds for probe_1 - 100 up - 150 muted.
	g.Expect(info.Muted).Should(BeEquivalentTo(100 + 50))           // 50 and 100 muted from inc[0]
	g.Expect(info.Unknown).Should(BeEquivalentTo(100))              // 100 unknown from ep[0]

	//g.Expect(s.Statuses["testGroup"]["__total__"][1].Up).Should(BeEquivalentTo(600))

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
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    200,
			SuccessSeconds: 100,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    25,
			SuccessSeconds: 50,
			Unknown:        300 - 50 - 25,
		},
	}

	s := CalculateStatuses(episodes, nil, CalculateAdjustedStepRanges(0, 300, 300).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))

	info := s["testGroup"]["__total__"][0]

	// Expect combined episode for timeslot=0
	g.Expect(info.Up).Should(BeEquivalentTo(300))
	g.Expect(info.Down).Should(BeEquivalentTo(0))
	g.Expect(info.Unknown).Should(BeEquivalentTo(0))
	g.Expect(info.NoData).Should(BeEquivalentTo(0))
}

// Catch problem with last episode
func Test_CalculateStatuses_combine_last_5mepisode_into_last_range_bucket(t *testing.T) {
	g := NewWithT(t)

	episodes := []types.DowntimeEpisode{
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       0,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       300,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       600,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "testGroup",
				Probe: "testProbe",
			},
			TimeSlot:       900,
			FailSeconds:    0,
			SuccessSeconds: 300,
		},
	}

	incidents := []types.DowntimeIncident{}

	// simple case with minimal step

	s := CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "testProbe")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("testProbe"))
	g.Expect(s["testGroup"]["testProbe"]).Should(HaveLen(4))
	g.Expect(s["testGroup"]["testProbe"][0].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][1].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][2].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][3].Up).Should(BeEquivalentTo(300))
	g.Expect(s["testGroup"]["testProbe"][0].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][2].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][3].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][0].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][2].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][3].Down).Should(BeEquivalentTo(0))

	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 300).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(4))
	totalInfo := s["testGroup"]["__total__"][0]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	totalInfo = s["testGroup"]["__total__"][1]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	totalInfo = s["testGroup"]["__total__"][2]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))
	totalInfo = s["testGroup"]["__total__"][3]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(300))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	// 2x step

	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 600).Ranges, "testGroup", "testProbe")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("testProbe"))
	g.Expect(s["testGroup"]["testProbe"]).Should(HaveLen(2))
	g.Expect(s["testGroup"]["testProbe"][0].Up).Should(BeEquivalentTo(600))
	g.Expect(s["testGroup"]["testProbe"][0].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][0].Unknown).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Up).Should(BeEquivalentTo(600))
	g.Expect(s["testGroup"]["testProbe"][1].Down).Should(BeEquivalentTo(0))
	g.Expect(s["testGroup"]["testProbe"][1].Unknown).Should(BeEquivalentTo(0))

	// 3x step with grouping
	s = CalculateStatuses(episodes, incidents, CalculateAdjustedStepRanges(0, 900, 900).Ranges, "testGroup", "__total__")

	g.Expect(s).ShouldNot(BeNil())
	g.Expect(s).Should(HaveKey("testGroup"))
	g.Expect(s["testGroup"]).Should(HaveKey("__total__"))
	g.Expect(s["testGroup"]["__total__"]).Should(HaveLen(2))
	totalInfo = s["testGroup"]["__total__"][0]
	g.Expect(totalInfo.Up).Should(BeEquivalentTo(900))
	g.Expect(totalInfo.Down).Should(BeEquivalentTo(0))
	g.Expect(totalInfo.Unknown).Should(BeEquivalentTo(0))

	g.Expect(s["testGroup"]["__total__"][0].Up).Should(BeEquivalentTo(900))
	g.Expect(s["testGroup"]["__total__"][1].Up).Should(BeEquivalentTo(300))
	totalInfo = s["testGroup"]["__total__"][1]

	// incidents!
}
