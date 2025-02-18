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

package check

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_CombineSeconds(t *testing.T) {
	tests := []struct {
		name             string
		slotSize         time.Duration
		arg1, arg2, want seconds
	}{
		{
			name:     "only success",
			slotSize: 5 * time.Minute,
			arg1:     seconds{up: 100, nodata: 200},
			arg2:     seconds{up: 300},
			want:     seconds{up: 300},
		}, {
			name:     "fail fills unknown (allowedFail == failUnknown)",
			slotSize: 5 * time.Minute,
			arg1:     seconds{up: 50, down: 100, unknown: 50, nodata: 100},
			arg2:     seconds{up: 150, down: 50, nodata: 100},
			want:     seconds{up: 150, down: 50, nodata: 100},
		}, {
			name:     "allowedFail < failUnknown",
			slotSize: 5 * time.Minute,
			arg1:     seconds{up: 100, down: 50, unknown: 20},
			arg2:     seconds{up: 101, down: 20, unknown: 100},
			// 101  -  max success
			// 49 = 100+50 - 101 - (maxKnown-success)
			// 71 =  100+20+101 - 101 -49 (maxAvail-success-fail)
			want: seconds{up: 101, down: 49, unknown: 71, nodata: 79},
		}, {
			name:     "only fail",
			slotSize: 30 * time.Second,
			arg1:     seconds{down: 1},
			arg2:     seconds{},
			want:     seconds{down: 1, nodata: 29},
		}, {
			name:     "greater fail in one episode",
			slotSize: 30 * time.Second,
			arg1:     seconds{10, 2, 0, 0},
			arg2:     seconds{10, 5, 0, 0},
			want:     seconds{up: 10, down: 5, nodata: 15},
		}, {
			name:     "",
			slotSize: 30 * time.Second,
			arg1:     seconds{up: 10, down: 2},
			arg2:     seconds{up: 5, down: 5},
			want:     seconds{up: 10, down: 2, nodata: 18},
		}, {
			name:     "Fill failed no more than known seconds",
			slotSize: 30 * time.Second,
			arg1:     seconds{up: 20, down: 2, unknown: 8},
			arg2:     seconds{up: 10, down: 15, unknown: 5},
			want:     seconds{up: 20, down: 5, unknown: 5},
		}, {
			// -> set success to more, decrease fail seconds.
			name:     "episode with more unknown seconds and with more success seconds",
			slotSize: 30 * time.Second,
			arg1:     seconds{up: 20, down: 10},
			arg2:     seconds{up: 25, down: 2},
			want:     seconds{up: 25, down: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep1 := SimpleEpisode(tt.arg1)
			ep2 := SimpleEpisode(tt.arg2)

			got := ep1.Combine(ep2, tt.slotSize)
			want := SimpleEpisode(tt.want)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("CombineSeconds() = %v, want %v", got, want)
			}
		})
	}
}

func SimpleEpisode(c seconds) Episode {
	return Episode{
		Up:      time.Second * c.up,
		Down:    time.Second * c.down,
		Unknown: time.Second * c.unknown,
		NoData:  time.Second * c.nodata,
	}
}

// this struct might tell us that we should keep it in the business logic as a separate field of an episode
// along with a time slot and a probe ref.
type seconds struct {
	up, down, unknown, nodata time.Duration
}

func Test_NewEpisode(t *testing.T) {
	ref := ProbeRef{}
	var start time.Time

	tests := []struct {
		name         string
		scrapePeriod time.Duration
		series       *StatusSeries
		want         Episode
	}{
		{
			name:         "zeros",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE(""),
			want:         Episode{},
		}, {
			name:         "1/1 up",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1."),
			want: Episode{
				SeriesRLE: "1.",
				Up:        time.Second,
			},
		}, {
			name:         "1/1 down",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1X"),
			want: Episode{
				SeriesRLE: "1X",
				Down:      time.Second,
			},
		}, {
			name:         "1/1 unknown",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1u"),
			want: Episode{
				SeriesRLE: "1u",
				Unknown:   time.Second,
			},
		}, {
			name:         "1/1 nodata",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1o"),
			want: Episode{
				SeriesRLE: "1o",
				NoData:    time.Second,
			},
		}, {
			name:         "30 nodata",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("30o"),
			want: Episode{
				SeriesRLE: "30o",
				NoData:    30 * time.Second,
			},
		}, {
			name:         "1 up 29 nodata",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1.29o"),
			want: Episode{
				SeriesRLE: "1.29o",
				Up:        1 * time.Second,
				NoData:    29 * time.Second,
			},
		}, {
			name:         "1 down 29 nodata",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1X29o"),
			want: Episode{
				SeriesRLE: "1X29o",
				Down:      1 * time.Second,
				NoData:    29 * time.Second,
			},
		}, {
			name:         "1 unknown 29 nodata",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("1u29o"),
			want: Episode{
				SeriesRLE: "1u29o",
				Unknown:   1 * time.Second,
				NoData:    29 * time.Second,
			},
		}, {
			name:         "15/30 up",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("15.15o"),
			want: Episode{
				SeriesRLE: "15.15o",
				Up:        15 * time.Second,
				NoData:    15 * time.Second,
			},
		}, {
			name:         "15/30 down",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("15X15o"),
			want: Episode{
				SeriesRLE: "15X15o",
				Down:      15 * time.Second,
				NoData:    15 * time.Second,
			},
		}, {
			name:         "15/30 unknown",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("15u15o"),
			want: Episode{
				SeriesRLE: "15u15o",
				Unknown:   15 * time.Second,
				NoData:    15 * time.Second,
			},
		}, {
			name:         "30/30 up",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("30."),
			want: Episode{
				SeriesRLE: "30.",
				Up:        30 * time.Second,
			},
		}, {
			name:         "30/30 down",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("30X"),
			want: Episode{
				SeriesRLE: "30X",
				Down:      30 * time.Second,
			},
		}, {
			name:         "30/30 unknown",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("30u"),
			want: Episode{
				SeriesRLE: "30u",
				Unknown:   30 * time.Second,
			},
		}, {
			name:         "10+10+10/30 unknown",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("10.10X10u"),
			want: Episode{
				SeriesRLE: "10.10X10u",
				Up:        10 * time.Second,
				Down:      10 * time.Second,
				Unknown:   10 * time.Second,
			},
		}, {
			name:         "10+10/30 unknown",
			scrapePeriod: time.Second,
			series:       NewStatusSeriesFromRLE("10X10u10o"),
			want: Episode{
				SeriesRLE: "10X10u10o",
				Down:      10 * time.Second,
				Unknown:   10 * time.Second,
				NoData:    10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// got := NewEpisode(ref, start, tt.step, tt.series)
			got := NewEpisode(ref, start, tt.scrapePeriod, tt.series)
			assert.Equal(t, tt.want.SeriesRLE, got.SeriesRLE, ".SeriesRLE")
			assert.Equal(t, tt.want.String(), got.String())
		})
	}
}
