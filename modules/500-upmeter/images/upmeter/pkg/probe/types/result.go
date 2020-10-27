package types

import (
	"fmt"
	"math"
	"strings"
)

type ProbeRef struct {
	Group string `json:"group"`
	Probe string `json:"probe"`
}

func (p ProbeRef) ProbeId() string {
	return fmt.Sprintf("%s/%s", p.Group, p.Probe)
}

// Probe result. Contains results for each probe's checks.
type ProbeResult struct {
	ProbeRef        ProbeRef
	CheckResults    map[string]int64
	FailDescription string
}

// MergeChecks updates CheckResults or set Result
func (r *ProbeResult) MergeChecks(next ProbeResult) {
	if next.CheckResults == nil {
		return
	}
	if len(r.CheckResults) == 0 {
		r.CheckResults = make(map[string]int64)
	}
	for k, v := range next.CheckResults {
		r.CheckResults[k] = v
	}
}

// CalcResult returns min value in CheckResult.
//
// Probe result is 1 if all check results are 1.
func (r *ProbeResult) Value() int64 {
	var res int64 = math.MaxInt64
	for _, v := range r.CheckResults {
		if v < res {
			res = v
		}
	}
	return res
}

func NewProbeResult(ref ProbeRef, checkName string, value interface{}) ProbeResult {
	var res int64 = 0

	switch v := value.(type) {
	case int:
		res = int64(v)
	case int64:
		res = v
	case ProbeResultValue:
		res = int64(v)
	case bool:
		if v {
			res = 1
		}
	case string:
		switch strings.ToLower(v) {
		case "ok", "success":
			res = 1
		}
	}

	return ProbeResult{
		ProbeRef:     ref,
		CheckResults: map[string]int64{checkName: res},
	}
}

type ProbeResultValue int64

const ProbeFailed ProbeResultValue = 0
const ProbeSuccess ProbeResultValue = 1

// Downtime with start and end timestamps aligned to 30s or 5m
type DowntimeEpisode struct {
	ProbeRef       ProbeRef `json:"probeRef"`
	TimeSlot       int64    `json:"ts"`      // timestamp: 30s or 5m time slot (timestamp that is a multiple of 30 seconds or 5 min)
	FailSeconds    int64    `json:"fail"`    // seconds of fail state during the slot range [timeslot;timeslot+30)
	SuccessSeconds int64    `json:"success"` // seconds of success state during the slot range [timeslot;timeslot+30)
	Unknown        int64    `json:"unknown"` // timeslot length - fail - success
}

func (e DowntimeEpisode) IsInRange(from int64, to int64) bool {
	return e.TimeSlot >= from && e.TimeSlot < to
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
	DowntimeName string   // a name of a Downtime custom resource
}

func (d DowntimeIncident) MuteDuration(from, to int64, group string) int64 {
	// Not in range
	if d.Start >= to || d.End < from {
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
	start := d.Start
	if d.Start < from {
		start = from
	}
	end := d.End
	if d.End > to {
		end = to
	}
	return end - start
}
