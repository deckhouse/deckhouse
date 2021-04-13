package check

import "time"

type StepRanges struct {
	From   int64
	To     int64
	Step   int64
	Ranges []Range
}

type Range struct {
	From int64
	To   int64
}

func (r Range) Diff() time.Duration {
	return time.Duration(r.To-r.From) * time.Second
}
