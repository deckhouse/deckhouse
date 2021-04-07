package check

import (
	"fmt"
	"time"

	"upmeter/pkg/util"
)

// Downtime with start and end timestamps aligned to 30s or 5m
type DowntimeEpisode struct {
	ProbeRef       ProbeRef `json:"probeRef"`
	TimeSlot       int64    `json:"ts"`      // timestamp: 30s or 5m time slot (timestamp that is a multiple of 30 seconds or 5 min)
	FailSeconds    int64    `json:"fail"`    // seconds of fail state during the slot range [timeslot;timeslot+30)
	SuccessSeconds int64    `json:"success"` // seconds of success state during the slot range [timeslot;timeslot+30)
	UnknownSeconds int64    `json:"unknown"` // seconds of "unknown" state
	NoDataSeconds  int64    `json:"nodata"`  // seconds without data
}

func NewDowntimeEpisode(ref ProbeRef, start time.Time, duration time.Duration, stats Stats) DowntimeEpisode {
	var (
		failureSec = timeInSeconds(duration, int64(stats.Down), int64(stats.Expected))
		successSec = timeInSeconds(duration, int64(stats.Up), int64(stats.Expected))
		unknownSec = timeInSeconds(duration, int64(stats.Unknown), int64(stats.Expected))
		nodata     = int64(stats.Expected - stats.Down - stats.Up - stats.Unknown)
		nodataSec  = timeInSeconds(duration, nodata, int64(stats.Expected))
	)

	return DowntimeEpisode{
		ProbeRef:       ref,
		TimeSlot:       start.Unix(),
		FailSeconds:    failureSec,
		SuccessSeconds: successSec,
		UnknownSeconds: unknownSec,
		NoDataSeconds:  nodataSec,
	}
}

func (e DowntimeEpisode) IsInRange(from int64, to int64) bool {
	return e.TimeSlot >= from && e.TimeSlot < to
}

func (e DowntimeEpisode) Known() int64 {
	return e.SuccessSeconds + e.FailSeconds
}

func (e DowntimeEpisode) Avail() int64 {
	return e.SuccessSeconds + e.FailSeconds + e.UnknownSeconds
}

func (e DowntimeEpisode) Total() int64 {
	return e.SuccessSeconds + e.FailSeconds + e.UnknownSeconds + e.NoDataSeconds
}

func (e DowntimeEpisode) IsCorrect(step int64) bool {
	return e.Total() <= step
}

func (e DowntimeEpisode) CombineSeconds(new DowntimeEpisode, step int64) DowntimeEpisode {
	target := DowntimeEpisode{
		ProbeRef: ProbeRef{
			Group: e.ProbeRef.Group,
			Probe: e.ProbeRef.Probe,
		},
		TimeSlot: e.TimeSlot,
	}

	// Combined NoDataSeconds is a minimum of unavailable seconds.
	// Episodes can be incomplete, so use step for proper calculation.
	targetAvail := util.Max(e.Avail(), new.Avail())
	target.NoDataSeconds = step - targetAvail

	target.SuccessSeconds = util.Max(e.SuccessSeconds, new.SuccessSeconds)

	failUnknown := targetAvail - target.SuccessSeconds

	// '==' is a "fail=0, unknown=0" case
	// '<' case is impossible, but who knows.
	if failUnknown <= 0 {
		target.UnknownSeconds = 0
		target.FailSeconds = 0
		return target
	}

	// Success and Fail seconds are filling UnknownSeconds, but not more than
	// maximum sum of known seconds.
	maxKnown := util.Max(e.Known(), new.Known())
	allowedFail := maxKnown - target.SuccessSeconds

	if allowedFail == failUnknown {
		target.FailSeconds = allowedFail
		target.UnknownSeconds = 0
	}
	if allowedFail < failUnknown {
		target.FailSeconds = allowedFail
		target.UnknownSeconds = failUnknown - allowedFail
	}
	if allowedFail > failUnknown {
		// Impossible. targetAvail is always greater than maxKnown.
		target.FailSeconds = failUnknown
		target.UnknownSeconds = 0
	}

	return target
}

func (e DowntimeEpisode) IsEqualSeconds(a DowntimeEpisode) bool {
	if e.SuccessSeconds != a.SuccessSeconds {
		return false
	}
	if e.FailSeconds != a.FailSeconds {
		return false
	}
	if e.UnknownSeconds != a.UnknownSeconds {
		return false
	}
	if e.NoDataSeconds != a.NoDataSeconds {
		return false
	}
	return true
}

func (e DowntimeEpisode) DumpString() string {
	return fmt.Sprintf("ts=%d probe='%s' s=%d f=%d u=%d n=%d",
		e.TimeSlot,
		e.ProbeRef.Id(),
		e.SuccessSeconds,
		e.FailSeconds,
		e.UnknownSeconds,
		e.NoDataSeconds,
	)
}

// ByTimeSlot implements sort.Interface based on the TimeSlot field.
type ByTimeSlot []DowntimeEpisode

func (a ByTimeSlot) Len() int           { return len(a) }
func (a ByTimeSlot) Less(i, j int) bool { return a[i].TimeSlot < a[j].TimeSlot }
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

// MuteDuration returns the count of seconds between 'from' and 'to'
// that are affected by this incident for particular 'group'.
func (d DowntimeIncident) MuteDuration(rng Range, group string) int64 {
	// Not in range
	if d.Start >= rng.To || d.End < rng.From {
		return 0
	}

	isAffected := false
	for _, affectedGroup := range d.Affected {
		if group == affectedGroup {
			isAffected = true
			break
		}
	}
	if !isAffected {
		return 0
	}

	// Calculate mute duration for range [from; to]
	var (
		start = util.Max(d.Start, rng.From)
		end   = util.Min(d.End, rng.To)
	)

	return end - start
}

func timeInSeconds(d time.Duration, counts, total int64) int64 {
	if total == 0 {
		return 0
	}

	return int64(d.Seconds() * float64(counts) / float64(total))
}

type Stats struct {
	Expected, Up, Down, Unknown int
}

func (s Stats) String() string {
	return fmt.Sprintf("(Σ%d ↑%d ↓%d ?%d)", s.Expected, s.Up, s.Down, s.Unknown)
}
