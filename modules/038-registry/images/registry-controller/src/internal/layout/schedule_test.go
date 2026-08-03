/*
Copyright 2026 Flant JSC

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

package layout

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGarbageCollectionSchedule(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		windows    []MaintenanceWindow
		want       string
	}{{
		// The only source that knows what the operator meant.
		name:       "what was configured wins over everything",
		configured: "0 5 * * 0",
		windows:    []MaintenanceWindow{{From: "06:00", To: "08:00"}},
		want:       "0 5 * * 0",
	}, {
		name: "nothing configured and no window",
		want: DefaultGarbageCollectionSchedule,
	}, {
		// When the operator has already said disruption is acceptable, and collecting is a
		// small disruption of exactly that kind.
		name:    "the maintenance window of the master node group",
		windows: []MaintenanceWindow{{From: "06:00", To: "08:00", Days: []string{"Tue", "Sun"}}},
		want:    "0 6 * * 0,2",
	}, {
		name:    "a window on every day",
		windows: []MaintenanceWindow{{From: "02:30", To: "04:00"}},
		want:    "30 2 * * *",
	}, {
		// Sorted, so the same window always renders the same expression. Otherwise the
		// schedule would appear to change on every reconciliation and the storage spec
		// would never settle.
		name:    "days are ordered regardless of how they were written",
		windows: []MaintenanceWindow{{From: "01:00", To: "02:00", Days: []string{"Sun", "Fri", "Mon"}}},
		want:    "0 1 * * 0,1,5",
	}, {
		name:    "a duplicated day is not repeated",
		windows: []MaintenanceWindow{{From: "01:00", To: "02:00", Days: []string{"Mon", "Mon"}}},
		want:    "0 1 * * 1",
	}, {
		// A window that cannot be read is no reason to refuse to collect at all. The night
		// default is what an unconfigured cluster gets anyway.
		name:    "an unreadable window falls back to the default",
		windows: []MaintenanceWindow{{From: "not a time", To: "08:00"}},
		want:    DefaultGarbageCollectionSchedule,
	}, {
		name:    "an unknown weekday falls back to the default",
		windows: []MaintenanceWindow{{From: "06:00", To: "08:00", Days: []string{"Funday"}}},
		want:    DefaultGarbageCollectionSchedule,
	}, {
		name:    "an out-of-range hour falls back to the default",
		windows: []MaintenanceWindow{{From: "25:00", To: "26:00"}},
		want:    DefaultGarbageCollectionSchedule,
	}, {
		// The first readable window is taken, so a broken one earlier in the list does not
		// discard a usable one after it.
		name: "the first readable window is used",
		windows: []MaintenanceWindow{
			{From: "bad", To: "08:00"},
			{From: "04:15", To: "05:00", Days: []string{"Wed"}},
		},
		want: "15 4 * * 3",
	}, {
		name:       "whitespace around a configured schedule is not part of it",
		configured: "  0 4 * * *  ",
		want:       "0 4 * * *",
	}, {
		name:       "a configured value of only whitespace is not a configuration",
		configured: "   ",
		want:       DefaultGarbageCollectionSchedule,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, GarbageCollectionSchedule(test.configured, test.windows))
		})
	}
}

// TestDefaultScheduleIsAtNight states the reason the default is what it is: collecting puts a
// replica read-only, and that belongs where the cluster is quiet.
func TestDefaultScheduleIsAtNight(t *testing.T) {
	fields := strings.Fields(DefaultGarbageCollectionSchedule)
	require.Len(t, fields, 5, "not a five-field cron expression")

	hour, err := strconv.Atoi(fields[1])
	require.NoError(t, err, "the default runs at no fixed hour")
	assert.Less(t, hour, 6, "the default no longer runs at night")

	// Not on the hour. Every replica in every cluster firing at exactly 03:00 would put
	// them all in the same minute, and the lease would make all but one wait through it.
	assert.NotEqual(t, "0", fields[0])
}
