package entity

import (
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/probe/types"
)

//
//func Test_SaveDowntimeEpisodes(t *testing.T) {
//	g := NewWithT(t)
//
//	eps := []types.DowntimeEpisode{
//		{
//			ProbeRef: types.ProbeRef{
//				Group: "g",
//				Probe: "p",
//			},
//			Start:    5,
//			End:      10,
//			Duration: 3,
//		},
//	}
//
//	g.Expect(DowntimeEpisodeStorage).Should(HaveLen(0))
//
//	SaveDowntimeEpisodes(eps)
//
//	g.Expect(DowntimeEpisodeStorage).Should(HaveLen(1))
//	g.Expect(DowntimeEpisodeStorage).Should(HaveKey(int64(5)))
//
//	// Add another timestamp
//	eps = []types.DowntimeEpisode{
//		{
//			ProbeRef: types.ProbeRef{
//				Group: "g",
//				Probe: "p",
//			},
//			Start:    10,
//			End:      15,
//			Duration: 2,
//		},
//	}
//	SaveDowntimeEpisodes(eps)
//	g.Expect(DowntimeEpisodeStorage).Should(HaveLen(2))
//	g.Expect(DowntimeEpisodeStorage).Should(HaveKey(int64(10)))
//
//	// Add better downtime
//	g.Expect(DowntimeEpisodeStorage[5]["g/p:c"].Duration).Should(Equal(int64(3)))
//	eps = []types.DowntimeEpisode{
//		{
//			ProbeRef: types.ProbeRef{
//				Group: "g",
//				Probe: "p",
//			},
//			Start:    5,
//			End:      10,
//			Duration: 1,
//		},
//	}
//	SaveDowntimeEpisodes(eps)
//	g.Expect(DowntimeEpisodeStorage).Should(HaveLen(2))
//	g.Expect(DowntimeEpisodeStorage[5]["g/p:c"].Duration).Should(BeEquivalentTo(1))
//
//}

func Test_CombineEpisodes(t *testing.T) {
	g := NewWithT(t)

	emptyEp := types.DowntimeEpisode{}

	newEp := types.DowntimeEpisode{FailSeconds: 1}

	combined := CombineEpisodes(emptyEp, newEp)
	g.Expect(combined.FailSeconds).Should(BeEquivalentTo(1))

	combined = CombineEpisodes(
		types.DowntimeEpisode{
			FailSeconds:    2,
			SuccessSeconds: 10,
		},
		types.DowntimeEpisode{
			FailSeconds:    5,
			SuccessSeconds: 10,
		},
	)
	g.Expect(combined.FailSeconds).Should(BeEquivalentTo(5))
	g.Expect(combined.SuccessSeconds).Should(BeEquivalentTo(10))

	// ignore episodes with less success seconds and less known seconds
	combined = CombineEpisodes(
		types.DowntimeEpisode{
			FailSeconds:    2,
			SuccessSeconds: 10,
		},
		types.DowntimeEpisode{
			FailSeconds:    5,
			SuccessSeconds: 5,
		},
	)
	g.Expect(combined.FailSeconds).Should(BeEquivalentTo(2))
	g.Expect(combined.SuccessSeconds).Should(BeEquivalentTo(10))

	// Fill failed no more than known seconds
	combined = CombineEpisodes(
		types.DowntimeEpisode{
			FailSeconds:    2,
			SuccessSeconds: 20,
		},
		types.DowntimeEpisode{
			FailSeconds:    15,
			SuccessSeconds: 10,
		},
	)
	g.Expect(combined.SuccessSeconds).Should(BeEquivalentTo(20))
	g.Expect(combined.FailSeconds).Should(BeEquivalentTo(5)) // Fill failed no more than known seconds

	// episode with more unknown seconds and with more success seconds
	// -> set success to more, decrease fail seconds.
	combined = CombineEpisodes(
		types.DowntimeEpisode{
			FailSeconds:    10,
			SuccessSeconds: 20,
		},
		types.DowntimeEpisode{
			FailSeconds:    2,
			SuccessSeconds: 25,
		},
	)
	g.Expect(combined.SuccessSeconds).Should(BeEquivalentTo(25))
	g.Expect(combined.FailSeconds).Should(BeEquivalentTo(5))
}
