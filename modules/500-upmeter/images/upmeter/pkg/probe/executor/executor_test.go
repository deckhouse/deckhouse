package executor

import (
	"context"
	"testing"
)

func Test_IsMark(t *testing.T) {
	tests := []struct {
		name           string
		prevTime       int64
		startTime      int64
		results        []bool
		lastExportTime []int64
	}{
		{
			"slightly early than mark at start",
			0,
			28,
			[]bool{false, false, true, false},
			[]int64{0, 0, 30, 30},
		},
		{
			"granular mark at start",
			0,
			30,
			[]bool{true, false, false, false},
			[]int64{30, 30, 30, 30},
		},
		{
			"slightly later than 30 mark at start",
			0,
			32,
			[]bool{false, false, false, false},
			[]int64{30, 30, 30, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.results) != len(tt.lastExportTime) {
				t.Fatalf("bad test definition: should have equal length of results and lastExportTime arrays")
			}

			exp := NewProbeExecutor(context.Background())
			nowTime := tt.startTime

			for i := range tt.results {
				result := exp.CheckAndUpdateLastExportTime(nowTime)
				expectedResult := tt.results[i]

				if result != expectedResult {
					t.Fatalf("result[%d]: should return %v instead of '%v'", i, expectedResult, result)
				}

				expectedLastExportTime := tt.lastExportTime[i]
				if exp.LastExportTimestamp != expectedLastExportTime {
					t.Fatalf("lastExportTime[%d]: should %v instead of '%v'", i, expectedLastExportTime, exp.LastExportTimestamp)
				}

				nowTime++
			}
		})
	}
}
