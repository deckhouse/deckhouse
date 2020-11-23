package entity

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
