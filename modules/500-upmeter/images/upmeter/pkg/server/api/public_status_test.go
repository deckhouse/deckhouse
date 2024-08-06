/*
Copyright 2021 Flant JSC

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

package api

import (
	"testing"
	"time"

	"d8.io/upmeter/pkg/server/entity"
	"d8.io/upmeter/pkg/server/ranges"
)

func Test_currentRange(t *testing.T) {
	ts := time.Unix(1803468883, 90)
	rng := new15MinutesStepRange(ts)

	n := (rng.To - rng.From) / rng.Step
	if n != 3 {
		t.Errorf("expected to have 3 steps, got=%d in rng=%v", n, rng)
	}
}

func Test_calculateStatus(t *testing.T) {
	type args struct {
		summary []entity.EpisodeSummary
	}
	tests := []struct {
		name string
		args args
		want PublicStatus
	}{
		{
			name: "nodata is nodata",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300},
					{TimeSlot: 600},
					{TimeSlot: 900},
				},
			},
			want: StatusNoData,
		},
		{
			name: "all up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5 * time.Minute},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, Up: 5 * time.Minute},
				},
			},
			want: StatusOperational,
		},
		{
			name: "all up, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5 * time.Minute},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: StatusOperational,
		},
		{
			name: "little fail, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5*time.Minute - time.Second, Down: time.Second},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "unknown with a littlee fail",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: 5*time.Minute - time.Second, Down: time.Second},
					{TimeSlot: 600, Unknown: 5 * time.Minute},
					{TimeSlot: 900, Unknown: 5 * time.Minute},
				},
			},
			want: StatusDegraded,
		},

		{
			name: "unknown with a littlee fail, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: 5*time.Minute - time.Second, Down: time.Second},
					{TimeSlot: 600, Unknown: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: StatusDegraded,
		},

		{
			name: "all down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 5 * time.Minute},
					{TimeSlot: 600, Down: 5 * time.Minute},
					{TimeSlot: 900, Down: 5 * time.Minute},
				},
			},
			want: StatusOutage,
		},
		{
			name: "all almost unknown but down for 1 sec",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: (5*time.Minute - time.Second), Down: time.Second},
					{TimeSlot: 600, Unknown: (5*time.Minute - time.Second), Down: time.Second},
					{TimeSlot: 900, Unknown: (5*time.Minute - time.Second), Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "all down, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 5 * time.Minute},
					{TimeSlot: 600, Down: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: StatusOutage,
		},
		{
			name: "all down for 1 sec, no more data present",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 600, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 900, Down: time.Second, NoData: 5*time.Minute - time.Second},
				},
			},
			want: StatusOutage,
		},
		{
			name: "no data",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300},
					{TimeSlot: 600},
					{TimeSlot: 900},
				},
			},
			want: StatusNoData,
		},

		//  peek for quick operational status

		{
			name: "peek: full up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 30 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: full down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 30 * time.Second},
				},
			},
			want: StatusOutage,
		},
		{
			name: "peek: 1s up, all other down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Down: 29 * time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: 1s down, all other up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 29 * time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 15 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial unknown",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: 15 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial up + unknown",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Unknown: time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial up + down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial unknown + down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial up + unknown + down",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Unknown: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: no data",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300},
				},
			},
			want: StatusNoData,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateStatus(tt.args.summary); got != tt.want {
				t.Errorf("calculateStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

// test calculateAvailability

func Test_calculateAvailability(t *testing.T) {
	type args struct {
		summary []entity.EpisodeSummary
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "no data",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300},
					{TimeSlot: 600},
					{TimeSlot: 900},
				},
			},
			want: -1,
		},
		{
			name: "all up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5 * time.Minute},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, Up: 5 * time.Minute},
				},
			},
			want: 1,
		},
		{
			name: "all up, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5 * time.Minute},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: 10.0 / 10, // nodata excluded
		},
		{
			name: "little fail, last is empty",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 5*time.Minute - time.Second, Down: time.Second},
					{TimeSlot: 600, Up: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: (5.0*60 - 1 + 5*60) / 600, // nodata excluded
		},
		{
			name: "unknown is up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: 5 * time.Minute},
					{TimeSlot: 600, Unknown: 5 * time.Minute},
					{TimeSlot: 900, Unknown: 5 * time.Minute},
				},
			},
			want: 1,
		},
		{
			name: "no up",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 5 * time.Minute},
					{TimeSlot: 600, Down: 5 * time.Minute},
					{TimeSlot: 900, Down: 5 * time.Minute},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateAvailability(tt.args.summary); got != tt.want {
				t.Errorf("calculateAvailability() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_new30SecondsStepRange(t *testing.T) {
	now := time.Date(2024, time.November, 13, 0o4, 42, 17, 647, time.UTC)
	step := 30 * time.Second
	aligned := now.Truncate(step)
	// fmt.Println(aligned)
	misaligned := aligned.Add(1 * time.Second).Add(1 * time.Millisecond)
	// fmt.Println(aligned, misaligned)
	misalignedAlittleBit := aligned.Add(1 * time.Nanosecond)
	// fmt.Println(aligned, misaligned, misalignedAlittleBit)

	// The range we expect is from the recent 30s episode. All arguments are in seconds
	recent30sRange := ranges.NewStepRange(aligned.Add(-step).Unix(), aligned.Unix(), int64(step.Seconds()))

	type args struct {
		now time.Time
	}
	tests := []struct {
		name string
		args args
		want ranges.StepRange
	}{
		{
			name: "aligned time",
			args: args{now: aligned},
			want: recent30sRange,
		},
		{
			name: "misaligned time by 1.001s",
			args: args{now: misaligned},
			want: recent30sRange,
		},
		{
			name: "misaligned time by 1ns",
			args: args{now: misalignedAlittleBit},
			want: recent30sRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := new30SecondsStepRange(tt.args.now); !stepRangeEqual(got, tt.want) {
				t.Errorf("new30SecondsStepRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func stepRangeEqual(a, b ranges.StepRange) bool {
	if !(a.From == b.From && a.To == b.To && a.Step == b.Step) {
		return false
	}
	if len(a.Subranges) != len(b.Subranges) {
		return false
	}

	for i := range a.Subranges {
		if !(a.Subranges[i].From == b.Subranges[i].From && a.Subranges[i].To == b.Subranges[i].To) {
			return false
		}
	}

	return true
}

func TestPublicStatus_Compare(t *testing.T) {
	type args struct {
		s1 PublicStatus
	}
	tests := []struct {
		name string
		s    PublicStatus
		args args
		want PublicStatus
	}{
		// twins
		{
			name: "operational + operational = operational",
			s:    StatusOperational,
			args: args{StatusOperational},
			want: StatusOperational,
		},
		{
			name: "nodata + nodata = nodata",
			s:    StatusNoData,
			args: args{StatusNoData},
			want: StatusNoData,
		},
		{
			name: "degraded + degraded = degraded",
			s:    StatusDegraded,
			args: args{StatusDegraded},
			want: StatusDegraded,
		},
		{
			name: "outage + outage = outage",
			s:    StatusOutage,
			args: args{StatusOutage},
			want: StatusOutage,
		},
		{
			name: "error + error = error",
			s:    StatusError,
			args: args{StatusError},
			want: StatusError,
		},

		// nodata
		{
			name: "operational + nodata = operational",
			s:    StatusOperational,
			args: args{StatusNoData},
			want: StatusOperational,
		},
		{
			name: "outage + nodata = outage",
			s:    StatusOutage,
			args: args{StatusNoData},
			want: StatusOutage,
		},
		{
			name: "degraded + nodata = degraded",
			s:    StatusDegraded,
			args: args{StatusNoData},
			want: StatusDegraded,
		},

		// degraded
		{
			name: "operational + degraded = degraded",
			s:    StatusOperational,
			args: args{StatusDegraded},
			want: StatusDegraded,
		},
		{
			name: "outage + degraded = degraded",
			s:    StatusOutage,
			args: args{StatusDegraded},
			want: StatusDegraded,
		},

		// outage
		{
			name: "operational + outage = degraded",
			s:    StatusOperational,
			args: args{StatusOutage},
			want: StatusDegraded,
		},

		// error is contagious, we must see it
		{
			name: "operational + error = error",
			s:    StatusOperational,
			args: args{StatusError},
			want: StatusError,
		},
		{
			name: "nodata + error = error",
			s:    StatusNoData,
			args: args{StatusError},
			want: StatusError,
		},
		{
			name: "degraded + error = error",
			s:    StatusDegraded,
			args: args{StatusError},
			want: StatusError,
		},
		{
			name: "outage + error = error",
			s:    StatusOutage,
			args: args{StatusError},
			want: StatusError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Compare(tt.args.s1); got != tt.want {
				t.Errorf("PublicStatus.Compare() = %v, want %v", got, tt.want)
			}
			// reverse
			if got := tt.args.s1.Compare(tt.s); got != tt.want {
				t.Errorf("(rev) PublicStatus.Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
