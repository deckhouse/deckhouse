// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package termui

import "testing"

func TestComputeLayout(t *testing.T) {
	c := caps{warn: 5, logboxMin: 3}
	// reserve = 1, so avail = height-1 and room = height-3 (bar+action reserved). Milestones grow
	// first (all of them, space permitting); the logbox is the flex region that fills the leftover.
	cases := []struct {
		name                               string
		height, mile, warn, bannerH, connL int
		want                               layout
	}{
		{
			// Tall terminal, few milestones: the logbox stretches to fill the bottom.
			name:   "tall few milestones logbox fills",
			height: 50, mile: 4, warn: 2, bannerH: 0, connL: 0,
			want: layout{action: true, region: true, milestones: 4, logbox: 41},
		},
		{
			// Milestones grow first — all 18 shown — then the logbox fills the remaining rows.
			name:   "many milestones grow first then logbox",
			height: 50, mile: 18, warn: 2, bannerH: 0, connL: 0,
			want: layout{action: true, region: true, milestones: 18, logbox: 27},
		},
		{
			// Medium height, more milestones than fit: milestones take all the rows, logbox starved to 0.
			name:   "milestones priority starves logbox",
			height: 14, mile: 15, warn: 2, bannerH: 0, connL: 0,
			want: layout{action: true, region: true, milestones: 9},
		},
		{
			// Banner fits without hiding anything: shown, logbox fills the remainder.
			name:   "tall with banner shows banner + fills",
			height: 50, mile: 4, warn: 2, bannerH: 5, connL: 0,
			want: layout{banner: true, action: true, region: true, milestones: 4, logbox: 36},
		},
		{
			// Banner would not leave a minimum logbox, so it is dropped; the rest still fits.
			name:   "banner dropped keeps minimum logbox",
			height: 12, mile: 4, warn: 2, bannerH: 5, connL: 0,
			want: layout{action: true, region: true, milestones: 4, logbox: 3},
		},
		{
			// Too short for even a minimum logbox: the logbox is dropped, milestones take what fits.
			name:   "logbox dropped when too short",
			height: 9, mile: 8, warn: 2, bannerH: 0, connL: 0,
			want: layout{action: true, region: true, milestones: 4},
		},
		{
			// Only bar + action fit.
			name:   "bar plus action only",
			height: 4, mile: 8, warn: 2, bannerH: 0, connL: 0,
			want: layout{action: true},
		},
		{
			// Only the bar fits.
			name:   "bar only",
			height: 2, mile: 18, warn: 2, bannerH: 0, connL: 0,
			want: layout{},
		},
		{
			name:   "bar only height one",
			height: 1, mile: 18, warn: 2, bannerH: 0, connL: 0,
			want: layout{},
		},
		{
			// No milestones: logbox fills the whole region; banner shown when it fits.
			name:   "no milestones logbox fills with banner",
			height: 30, mile: 0, warn: 0, bannerH: 3, connL: 0,
			want: layout{banner: true, action: true, region: true, milestones: 0, logbox: 24},
		},
		{
			// connLine=1 steals one row from the logbox (it is the flex region).
			// height=22, room=19, mile=4, warn=2 → without conn: logbox=19-6=13; with conn: 19-7=12.
			name:   "connLine shrinks logbox by one",
			height: 22, mile: 4, warn: 2, bannerH: 0, connL: 1,
			want: layout{action: true, region: true, milestones: 4, logbox: 12},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeLayout(tc.height, tc.mile, tc.warn, tc.bannerH, tc.connL, c)
			if got != tc.want {
				t.Fatalf("computeLayout(h=%d,m=%d,w=%d,b=%d,conn=%d) = %+v, want %+v",
					tc.height, tc.mile, tc.warn, tc.bannerH, tc.connL, got, tc.want)
			}
		})
	}
}
