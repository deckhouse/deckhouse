package checks

import (
	"math"
	"strings"
)

// Probe result. Contains results for each probe's checks.
type Result struct {
	ProbeRef        ProbeRef
	CheckResults    map[string]int64
	FailDescription string
}

// MergeChecks updates CheckResults
func (r *Result) MergeChecks(next Result) {
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
func (r *Result) Value() int64 {
	var res int64 = math.MaxInt64
	for _, v := range r.CheckResults {
		if v < res {
			res = v
		}
	}
	return res
}

func NewResult(ref ProbeRef, checkName string, value interface{}) Result {
	var res int64 = 0

	switch v := value.(type) {
	case int:
		res = int64(v)
	case int64:
		res = v
	case Status:
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

	return Result{
		ProbeRef:     ref,
		CheckResults: map[string]int64{checkName: res},
	}
}

type Status int64

const (
	StatusFail Status = iota
	StatusSuccess
	StatusUnknown
)
