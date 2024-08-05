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
		summary  []entity.EpisodeSummary
		slotSize time.Duration
	}
	tests := []struct {
		name string
		args args
		want PublicStatus
	}{
		{
			name: "nodata is degraded",
			args: args{
				slotSize: 5 * time.Minute,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300},
					{TimeSlot: 600},
					{TimeSlot: 900},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "all up",
			args: args{
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 5 * time.Minute},
					{TimeSlot: 600, Down: 5 * time.Minute},
					{TimeSlot: 900, Down: 5 * time.Minute},
				},
			},
			want: StatusOutage,
		},
		{
			name: "all down for 1 sec",
			args: args{
				slotSize: 5 * time.Minute,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: (5*time.Minute - time.Second), Down: time.Second},
					{TimeSlot: 600, Unknown: (5*time.Minute - time.Second), Down: time.Second},
					{TimeSlot: 900, Unknown: (5*time.Minute - time.Second), Down: time.Second},
				},
			},
			want: StatusOutage,
		},
		{
			name: "all down, last is empty",
			args: args{
				slotSize: 5 * time.Minute,
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
				slotSize: 5 * time.Minute,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 600, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 900, Down: time.Second, NoData: 5*time.Minute - time.Second},
				},
			},
			want: StatusOutage,
		},

		//  peek for quick operational status

		{
			name: "peek: full up",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 30 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: full down",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 30 * time.Second},
				},
			},
			want: StatusOutage,
		},
		{
			name: "peek: 1s up, all other down",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Down: 29 * time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: 1s down, all other up",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 29 * time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial up",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: 15 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial unknown",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: 15 * time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial up + unknown",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Unknown: time.Second},
				},
			},
			want: StatusOperational,
		},
		{
			name: "peek: partial up + down",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial unknown + down",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Unknown: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
		{
			name: "peek: partial up + unknown + down",
			args: args{
				slotSize: 30 * time.Second,
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Up: time.Second, Unknown: time.Second, Down: time.Second},
				},
			},
			want: StatusDegraded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateStatus(tt.args.summary, tt.args.slotSize); got != tt.want {
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
