package types

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_CombineSeconds(t *testing.T) {
	g := NewWithT(t)

	var ep1, ep2, combined DowntimeEpisode

	// only success
	ep1 = SimpleEpisode(100, 0, 0, 200)
	ep1 = SimpleEpisode(300, 0, 0, 0)
	combined = ep1.CombineSeconds(ep2, 300)
	ExpectEpisode(g, combined, 300, 0, 0, 0)

	// fail fills unknown (allowedFail == failUnknown)
	ep1 = SimpleEpisode(50, 100, 50, 100)
	ep2 = SimpleEpisode(150, 50, 0, 100)
	combined = ep1.CombineSeconds(ep2, 300)
	ExpectEpisode(g, combined, 150, 50, 0, 100)

	// allowedFail < failUnknown
	ep1 = SimpleEpisode(100, 50, 20, 0)
	ep2 = SimpleEpisode(101, 20, 100, 0)
	combined = ep1.CombineSeconds(ep2, 300)
	// 101  -  max success
	// 49 = 100+50 - 101 - (maxKnown-success)
	// 71 =  100+20+101 - 101 -49 (maxAvail-success-fail)
	ExpectEpisode(g, combined, 101, 49, 71, 79)
}

// old tests from entity/downtime_test
func Test_CombineSeconds_30s(t *testing.T) {
	g := NewWithT(t)

	var ep1, ep2, combined DowntimeEpisode

	// only fail
	ep1 = SimpleEpisode(0, 1, 0, 0)
	ep2 = SimpleEpisode(0, 0, 0, 0)
	combined = ep1.CombineSeconds(ep2, 30)
	ExpectEpisode(g, combined, 0, 1, 0, 29)

	// greater fail in one episode
	ep1 = SimpleEpisode(10, 2, 0, 0)
	ep2 = SimpleEpisode(10, 5, 0, 0)
	combined = ep1.CombineSeconds(ep2, 30)
	ExpectEpisode(g, combined, 10, 5, 0, 15)

	//
	ep1 = SimpleEpisode(10, 2, 0, 0)
	ep2 = SimpleEpisode(5, 5, 0, 0)
	combined = ep1.CombineSeconds(ep2, 30)
	ExpectEpisode(g, combined, 10, 2, 0, 18)

	// Fill failed no more than known seconds
	ep1 = SimpleEpisode(20, 2, 8, 0)
	ep2 = SimpleEpisode(10, 15, 5, 0)
	combined = ep1.CombineSeconds(ep2, 30)
	ExpectEpisode(g, combined, 20, 5, 5, 0)

	// episode with more unknown seconds and with more success seconds
	// -> set success to more, decrease fail seconds.
	ep1 = SimpleEpisode(20, 10, 0, 0)
	ep2 = SimpleEpisode(25, 2, 0, 0)
	combined = ep1.CombineSeconds(ep2, 30)
	ExpectEpisode(g, combined, 25, 5, 0, 0)
}

func SimpleEpisode(success, fail, unknown, nodata int64) DowntimeEpisode {
	return DowntimeEpisode{
		SuccessSeconds: success,
		FailSeconds:    fail,
		Unknown:        unknown,
		NoData:         nodata,
	}
}

func ExpectEpisode(g *WithT, ep DowntimeEpisode, success, fail, unknown, nodata int64) {
	if success >= 0 {
		g.Expect(ep.SuccessSeconds).Should(BeEquivalentTo(success), "Check Success, ep: %+v", ep)
	}
	if fail >= 0 {
		g.Expect(ep.FailSeconds).Should(BeEquivalentTo(fail), "Check Fail, ep: %+v", ep)
	}
	if unknown >= 0 {
		g.Expect(ep.Unknown).Should(BeEquivalentTo(unknown), "Check Unknown, ep: %+v", ep)
	}
	if nodata >= 0 {
		g.Expect(ep.NoData).Should(BeEquivalentTo(nodata), "Check NoData, ep: %+v", ep)
	}
}
