package check

type Status int64

const (
	StatusFail Status = iota
	StatusSuccess
	StatusUnknown
)

// Runner represents probe result. It contains results for each probe's checkss.
type Result struct {
	ProbeRef        ProbeRef
	CheckResults    map[string]Status
	FailDescription string
}

// SetCheckStatus updates check results
func (r *Result) SetCheckStatus(next Result) {
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

// Value deduces the probe result. It does it by minimizing check results: Fail < Success < Unknown
func (r *Result) Value() Status {
	var res = StatusUnknown
	for _, v := range r.CheckResults {
		if v < res {
			res = v
		}
	}
	return res
}

// NewResult creates result struct for a check
func NewResult(ref ProbeRef, checkName string, status Status) Result {
	return Result{
		ProbeRef:     ref,
		CheckResults: map[string]Status{checkName: status},
	}
}
