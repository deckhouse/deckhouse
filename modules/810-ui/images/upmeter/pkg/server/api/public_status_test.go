/*
Copyright 2023 Flant JSC

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
		summary []entity.EpisodeSummary
	}
	tests := []struct {
		name string
		args args
		want PublicStatus
	}{
		{
			name: "all zeroes",
			args: args{
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
			name: "no up",
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
			name: "no up, last is empty",
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
			name: "all down for 1 sec",
			args: args{
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
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: 5 * time.Minute},
					{TimeSlot: 600, Down: 5 * time.Minute},
					{TimeSlot: 900, NoData: 5 * time.Minute},
				},
			},
			want: StatusOutage,
		},
		{
			name: "all down for 1 sec",
			args: args{
				summary: []entity.EpisodeSummary{
					{TimeSlot: 300, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 600, Down: time.Second, NoData: 5*time.Minute - time.Second},
					{TimeSlot: 900, Down: time.Second, NoData: 5*time.Minute - time.Second},
				},
			},
			want: StatusOutage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateStatus(tt.args.summary, 5*time.Minute); got != tt.want {
				t.Errorf("calculateStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
