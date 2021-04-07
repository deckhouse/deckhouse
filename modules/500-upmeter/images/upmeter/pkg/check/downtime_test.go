package check

import (
	"reflect"
	"testing"
	"time"

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
		UnknownSeconds: unknown,
		NoDataSeconds:  nodata,
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
		g.Expect(ep.UnknownSeconds).Should(BeEquivalentTo(unknown), "Check Unknown, ep: %+v", ep)
	}
	if nodata >= 0 {
		g.Expect(ep.NoDataSeconds).Should(BeEquivalentTo(nodata), "Check NoData, ep: %+v", ep)
	}
}

func Test_NewDowntimeEpisode(t *testing.T) {
	ref := ProbeRef{}
	start := time.Unix(0, 0)
	duration := 30 * time.Second

	tests := []struct {
		name  string
		stats Stats
		want  DowntimeEpisode
	}{
		{
			name: "zeros",
			want: DowntimeEpisode{},
		}, {
			name:  "1/1 up",
			stats: Stats{Expected: 1, Up: 1},
			want:  DowntimeEpisode{SuccessSeconds: 30},
		}, {
			name:  "1/1 down",
			stats: Stats{Expected: 1, Down: 1},
			want:  DowntimeEpisode{FailSeconds: 30},
		}, {
			name:  "1/1 unknown",
			stats: Stats{Expected: 1, Unknown: 1},
			want:  DowntimeEpisode{UnknownSeconds: 30},
		}, {
			name:  "1/1 nodata",
			stats: Stats{Expected: 1},
			want:  DowntimeEpisode{NoDataSeconds: 30},
		}, {
			name:  "1/30 nodata",
			stats: Stats{Expected: 30},
			want:  DowntimeEpisode{NoDataSeconds: 30},
		}, {
			name:  "1/30 up",
			stats: Stats{Expected: 30, Up: 1},
			want:  DowntimeEpisode{SuccessSeconds: 1, NoDataSeconds: 29},
		}, {
			name:  "1/30 down",
			stats: Stats{Expected: 30, Down: 1},
			want:  DowntimeEpisode{FailSeconds: 1, NoDataSeconds: 29},
		}, {
			name:  "1/30 unknown",
			stats: Stats{Expected: 30, Unknown: 1},
			want:  DowntimeEpisode{UnknownSeconds: 1, NoDataSeconds: 29},
		}, {
			name:  "15/30 up",
			stats: Stats{Expected: 30, Up: 15},
			want:  DowntimeEpisode{SuccessSeconds: 15, NoDataSeconds: 15},
		}, {
			name:  "15/30 down",
			stats: Stats{Expected: 30, Down: 15},
			want:  DowntimeEpisode{FailSeconds: 15, NoDataSeconds: 15},
		}, {
			name:  "15/30 unknown",
			stats: Stats{Expected: 30, Unknown: 15},
			want:  DowntimeEpisode{UnknownSeconds: 15, NoDataSeconds: 15},
		}, {
			name:  "30/30 up",
			stats: Stats{Expected: 30, Up: 30},
			want:  DowntimeEpisode{SuccessSeconds: 30},
		}, {
			name:  "30/30 down",
			stats: Stats{Expected: 30, Down: 30},
			want:  DowntimeEpisode{FailSeconds: 30},
		}, {
			name:  "30/30 unknown",
			stats: Stats{Expected: 30, Unknown: 30},
			want:  DowntimeEpisode{UnknownSeconds: 30},
		}, {
			name: "10+10+10/30 unknown",
			stats: Stats{
				Expected: 30,
				Up:       10,
				Down:     10,
				Unknown:  10},
			want: DowntimeEpisode{
				SuccessSeconds: 10,
				FailSeconds:    10,
				UnknownSeconds: 10,
			},
		}, {
			name: "10+10/30 unknown",
			stats: Stats{
				Expected: 30,
				Down:     10,
				Unknown:  10},
			want: DowntimeEpisode{
				FailSeconds:    10,
				UnknownSeconds: 10,
				NoDataSeconds:  10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDowntimeEpisode(ref, start, duration, tt.stats); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDowntimeEpisode() = %v, want %v", got, tt.want)
			}
		})
	}
}
