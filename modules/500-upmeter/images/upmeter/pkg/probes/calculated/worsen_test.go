package calculated

import (
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/checks"
)

func Test_worsen(t *testing.T) {
	type args struct {
		to   seconds
		from seconds
		step int64
	}

	tests := []struct {
		name string
		args args
		want seconds
	}{
		{
			name: "only success",
			args: args{
				to:   seconds{success: 1},
				from: seconds{success: 1},
				step: 1,
			},
			want: seconds{success: 1},
		}, {
			name: "only fail",
			args: args{
				to:   seconds{fail: 1},
				from: seconds{fail: 1},
				step: 1,
			},
			want: seconds{fail: 1},
		}, {
			name: "only unknown",
			args: args{
				to:   seconds{unknown: 1},
				from: seconds{unknown: 1},
				step: 1,
			},
			want: seconds{unknown: 1},
		}, {
			name: "only nodata",
			args: args{
				to:   seconds{nodata: 1},
				from: seconds{nodata: 1},
				step: 1,
			},
			want: seconds{nodata: 1},
		}, {
			name: "zero step",
			args: args{
				to:   seconds{nodata: 1},
				from: seconds{nodata: 1},
				step: 0,
			},
			want: seconds{},
		}, {
			name: "11 in 30",
			args: args{
				to:   seconds{unknown: 11},
				from: seconds{unknown: 11},
				step: 30,
			},
			want: seconds{unknown: 11},
		}, {
			name: "11 (to) 1 (from) in 30",
			args: args{
				to:   seconds{unknown: 11},
				from: seconds{unknown: 1},
				step: 30,
			},
			want: seconds{unknown: 1},
		}, {
			name: "11 (to) 1 (from) in 30",
			args: args{
				to:   seconds{unknown: 1},
				from: seconds{unknown: 11},
				step: 30,
			},
			want: seconds{unknown: 1},
		}, {
			name: "all equal",
			args: args{
				to:   seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				from: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				step: 30,
			},
			want: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
		}, {
			name: "all equal",
			args: args{
				to:   seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				from: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				step: 30,
			},
			want: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
		}, {
			name: "fail beats success",
			args: args{
				to:   seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				from: seconds{success: 7, fail: 3, unknown: 5, nodata: 1},
				step: 30,
			},
			want: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
		}, {
			name: "fail beats success",
			args: args{
				to:   seconds{success: 7, fail: 3, unknown: 5, nodata: 1},
				from: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				step: 30,
			},
			want: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
		}, {
			name: "fail (from) beats unknown, unknown (to) beats success",
			args: args{
				to:   seconds{success: 3, fail: 5, unknown: 7, nodata: 1},
				from: seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				step: 30,
			},
			want: seconds{success: 1, fail: 7, unknown: 7, nodata: 1},
		}, {
			name: "fail (to) beats unknown, unknown (from) beats success",
			args: args{
				to:   seconds{success: 3, fail: 7, unknown: 5, nodata: 1},
				from: seconds{success: 3, fail: 5, unknown: 7, nodata: 1},
				step: 30,
			},
			want: seconds{success: 1, fail: 7, unknown: 7, nodata: 1},
		}, {
			name: "fail beats unknown",
			args: args{
				to:   seconds{success: 3, fail: 3, unknown: 3, nodata: 3},
				from: seconds{success: 11, fail: 11, unknown: 11, nodata: 11},
				step: 30,
			},
			want: seconds{success: 0, fail: 11, unknown: 1, nodata: 0},
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			to := SimpleEpisode(tt.args.to)
			from := SimpleEpisode(tt.args.from)

			worsen(&to, &from, tt.args.step)

			g.Expect(to.FailSeconds).To(Equal(tt.want.fail),
				"FailSeconds do not match, got=%d, want=%d", to.FailSeconds, tt.want.fail)

			g.Expect(to.UnknownSeconds).To(Equal(tt.want.unknown),
				"UnknownSeconds do not match, got=%d, want=%d", to.UnknownSeconds, tt.want.unknown)

			g.Expect(to.NoDataSeconds).To(Equal(tt.want.nodata),
				"NoDataSeconds do not match, got=%d, want=%d", to.NoDataSeconds, tt.want.nodata)

			g.Expect(to.SuccessSeconds).To(Equal(tt.want.success),
				"SuccessSeconds do not match, got=%d, want=%d", to.SuccessSeconds, tt.want.success)
		})
	}
}

type seconds struct {
	success, fail, unknown, nodata int64
}

func SimpleEpisode(s seconds) checks.DowntimeEpisode {
	return checks.DowntimeEpisode{
		SuccessSeconds: s.success,
		FailSeconds:    s.fail,
		UnknownSeconds: s.unknown,
		NoDataSeconds:  s.nodata,
	}
}
