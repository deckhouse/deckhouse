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

package check

import (
	"fmt"
	"time"
)

// Episode with time counters and start aligned to 30s or 5m
type Episode struct {
	ProbeRef ProbeRef      `json:"probeRef"`
	TimeSlot time.Time     `json:"ts"`      // timestamp: 30s or 5m time slot (timestamp that is a multiple of 30 seconds or 5 min)
	Down     time.Duration `json:"fail"`    // seconds of fail state during the slot range [timeslot;timeslot+30)
	Up       time.Duration `json:"success"` // seconds of success state during the slot range [timeslot;timeslot+30)
	Unknown  time.Duration `json:"unknown"` // seconds of "unknown" state
	NoData   time.Duration `json:"nodata"`  // seconds without data
}

func NewEpisode(ref ProbeRef, start time.Time, step time.Duration, counters Stats) Episode {
	var (
		total   = step * time.Duration(counters.Expected)
		up      = step * time.Duration(counters.Up)
		down    = step * time.Duration(counters.Down)
		unknown = step * time.Duration(counters.Unknown)
		nodata  = total - up - down - unknown
	)

	return Episode{
		ProbeRef: ref,
		TimeSlot: start,
		Up:       up,
		Down:     down,
		Unknown:  unknown,
		NoData:   nodata,
	}
}

func (e Episode) IsInRange(from, to int64) bool {
	return e.TimeSlot.Unix() >= from && e.TimeSlot.Unix() < to
}

func (e Episode) Known() time.Duration {
	return e.Up + e.Down
}

func (e Episode) Avail() time.Duration {
	return e.Up + e.Down + e.Unknown
}

func (e Episode) Total() time.Duration {
	return e.Up + e.Down + e.Unknown + e.NoData
}

func (e Episode) IsCorrect(step time.Duration) bool {
	return e.Total() <= step
}

// Combine deduces en episode from the two choosing the longest possible uptime, then the longest
// possible downtime, then longest possible uncertainty. All the range time left will be unknown.
func (e Episode) Combine(o Episode, slotSize time.Duration) Episode {
	target := Episode{
		ProbeRef: e.ProbeRef,
		TimeSlot: e.TimeSlot,
	}

	// Combined NoData is a minimum of unavailable seconds.
	// Episodes can be incomplete, so use slotSize for proper calculation.
	targetAvail := longest(e.Avail(), o.Avail())
	target.NoData = slotSize - targetAvail

	target.Up = longest(e.Up, o.Up)

	failUnknown := targetAvail - target.Up

	// '==' is a "fail=0, unknown=0" case
	// '<' case is impossible, but who knows.
	if failUnknown <= 0 {
		target.Unknown = 0
		target.Down = 0
		return target
	}

	// Success and Fail seconds are filling Unknown, but not more than
	// maximum sum of known seconds.
	maxKnown := longest(e.Known(), o.Known())
	allowedFail := maxKnown - target.Up

	if allowedFail == failUnknown {
		target.Down = allowedFail
		target.Unknown = 0
	}
	if allowedFail < failUnknown {
		target.Down = allowedFail
		target.Unknown = failUnknown - allowedFail
	}
	if allowedFail > failUnknown {
		// Impossible. targetAvail is always greater than maxKnown.
		target.Down = failUnknown
		target.Unknown = 0
	}

	return target
}

func (e Episode) EqualTimers(a Episode) bool {
	return e.Up == a.Up &&
		e.Down == a.Down &&
		e.Unknown == a.Unknown &&
		e.NoData == a.NoData
}

func (e Episode) String() string {
	return fmt.Sprintf("slot=%s probe=%s up=%s down=%s uncertain=%s notmeasured=%s",
		e.TimeSlot.Format(time.Stamp),
		e.ProbeRef.Id(),
		e.Up,
		e.Down,
		e.Unknown,
		e.NoData,
	)
}

// ByTimeSlot implements sort.Interface based on the TimeSlot field.
type ByTimeSlot []Episode

func (a ByTimeSlot) Len() int           { return len(a) }
func (a ByTimeSlot) Less(i, j int) bool { return a[j].TimeSlot.After(a[i].TimeSlot) }
func (a ByTimeSlot) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// DowntimeIncident defines a long lasting downtime. It is stored in cluster as a CR.
type DowntimeIncident struct {
	Start        int64 // start of downtime ()
	End          int64 // end of downtime ()
	Duration     int64 // duration in seconds
	Type         string
	Description  string
	Affected     []string // a list of affected groups
	DowntimeName string   // a checkName of a Downtime custom resource
}

type Stats struct {
	Expected, Up, Down, Unknown int
}

func (s Stats) String() string {
	return fmt.Sprintf("(Σ%d ↑%d ↓%d ?%d)", s.Expected, s.Up, s.Down, s.Unknown)
}

func longest(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
