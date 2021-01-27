package dao

type StepRanges struct {
	From   int64
	To     int64
	Step   int64
	Ranges [][]int64 // array of 2 items arrays: 0 is a step range start, 1 is a step range end.
}
