package checks

import (
	"strings"
)

// Probe result. Contains results for each probe's checks.
type Result struct {
	ProbeRef        ProbeRef
	CheckResults    map[string]Status
	FailDescription string
}

// MergeChecks updates CheckResults
func (r *Result) MergeChecks(next Result) {
	if next.CheckResults == nil {
		return
	}
	if len(r.CheckResults) == 0 {
		r.CheckResults = make(map[string]Status)
	}
	for k, v := range next.CheckResults {
		r.CheckResults[k] = v
	}
}

// CalcResult returns min Status in CheckResult.
//
// Fail < Success < Unknown
func (r *Result) Value() Status {
	var res = StatusUnknown
	for _, v := range r.CheckResults {
		if v < res {
			res = v
		}
	}
	return res
}

func NewResult(ref ProbeRef, checkName string, value interface{}) Result {
	var res = StatusFail

	switch v := value.(type) {
	case int:
		res = Status(v)
	case int64:
		res = Status(v)
	case Status:
		res = v
	case bool:
		if v {
			res = StatusSuccess
		}
	case string:
		switch strings.ToLower(v) {
		case "ok", "success":
			res = StatusSuccess
		}
	}

	return Result{
		ProbeRef:     ref,
		CheckResults: map[string]Status{checkName: res},
	}
}

type Status int64

const (
	StatusFail Status = iota
	StatusSuccess
	StatusUnknown
)
