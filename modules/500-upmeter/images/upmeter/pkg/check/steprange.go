package check

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

func (r Range) Diff() int64 {
	return r.To - r.From
}
