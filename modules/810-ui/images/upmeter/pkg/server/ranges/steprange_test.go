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

package ranges

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func Test_CalculateAdjustedStepRanges(t *testing.T) {
	type args struct {
		from, to, step int64
	}

	tests := []struct {
		name string
		args args
		want []Range
	}{
		{
			name: "Single range",
			args: args{from: 0, to: 300, step: 300},
			want: []Range{{From: 0, To: 300}},
		}, {
			name: "Adjusts",
			args: args{from: 21, to: 663, step: 321},
			want: []Range{
				{From: 300, To: 600},
				{From: 600, To: 900},
			},
		}, {
			name: "Bigger step (1h)",
			args: args{from: 21, to: 10000, step: 3600},
			want: []Range{
				{From: 3600, To: 7200},
				{From: 7200, To: 10800},
			},
		}, {
			name: "Step ranges used in status_test.go",
			args: args{from: 0, to: 900, step: 300},
			want: []Range{
				{From: 0, To: 300},
				{From: 300, To: 600},
				{From: 600, To: 900},
			},
		}, {
			name: "Step ranges used in status_test.go",
			args: args{from: 0, to: 1200, step: 600},
			want: []Range{
				{From: 0, To: 600},
				{From: 600, To: 1200},
			},
		}, {
			name: "Big step 3000",
			args: args{from: 3500, to: 10000, step: 3000},
			want: []Range{
				{From: 6000, To: 9000},
				{From: 9000, To: 12000},
			},
		}, {
			name: "Big step 7200",
			args: args{from: 10000, to: 70000, step: 7200},
			want: []Range{
				{From: 14400, To: 21600},
				{From: 21600, To: 28800},
				{From: 28800, To: 36000},
				{From: 36000, To: 43200},
				{From: 43200, To: 50400},
				{From: 50400, To: 57600},
				{From: 57600, To: 64800},
				{From: 64800, To: 72000},
			},
		}, {
			name: "Real timestamps",
			args: args{from: 1603180029, to: 1603784829, step: 86400},
			want: []Range{
				{From: 1603238400, To: 1603324800},
				{From: 1603324800, To: 1603411200},
				{From: 1603411200, To: 1603497600},
				{From: 1603497600, To: 1603584000},
				{From: 1603584000, To: 1603670400},
				{From: 1603670400, To: 1603756800},
				{From: 1603756800, To: 1603843200},
			},
		}, {
			name: "30s aligned range is foced to 5m",
			args: args{from: 1603784700, to: 1603784730, step: 30},
			want: []Range{
				{From: 1603784700, To: 1603785000},
			},
		}, {
			name: "30s not aligned range is foced to 5m",
			args: args{from: 1603784693, to: 1603784732, step: 30},
			want: []Range{
				{From: 1603784700, To: 1603785000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New5MinStepRange(tt.args.from, tt.args.to, tt.args.step).Subranges

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalculateAdjustedStepRanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignStep(t *testing.T) {
	tests := []struct {
		name string
		arg  int64
		want int64
	}{
		{name: "-1 is 300", arg: -1, want: 300},
		{name: "0 is 300", arg: 0, want: 300},
		{name: "299 is 300", arg: 256, want: 300},

		{name: "300 is 300", arg: 300, want: 300},
		{name: "301 is 300", arg: 300, want: 300},
		{name: "599 is 300", arg: 300, want: 300},

		{name: "600 is 600", arg: 600, want: 600},
		{name: "601 is 600", arg: 601, want: 600},
		{name: "899 is 600", arg: 899, want: 600},

		{name: "900 is 900", arg: 900, want: 900},
		{name: "901 is 900", arg: 901, want: 900},
		{name: "1199 is 900", arg: 1199, want: 900},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alignStep(tt.arg, 300); got != tt.want {
				t.Errorf("alignStep() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignEdge(t *testing.T) {
	type args struct {
		to   int64
		step int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "1, 1",
			args: args{to: 1, step: 1},
			want: 1,
		},

		// 300
		{
			name: "1, 300",
			args: args{to: 1, step: 300},
			want: 300,
		},
		{
			name: "300, 300",
			args: args{to: 300, step: 300},
			want: 300,
		},

		// 600
		{
			name: "301, 300",
			args: args{to: 301, step: 300},
			want: 600,
		},
		{
			name: "599, 300",
			args: args{to: 599, step: 300},
			want: 600,
		},
		{
			name: "600, 300",
			args: args{to: 600, step: 300},
			want: 600,
		},

		// 900
		{
			name: "601, 300",
			args: args{to: 601, step: 300},
			want: 900,
		},
		{
			name: "899, 300",
			args: args{to: 899, step: 300},
			want: 900,
		},
		{
			name: "900, 300",
			args: args{to: 900, step: 300},
			want: 900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alignEdgeForward(tt.args.to, tt.args.step); got != tt.want {
				t.Errorf("alignEdge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readjusting(t *testing.T) {
	for i := 0; i < 10; i++ {
		from := rand.Int63n(time.Now().Unix())
		to := from + rand.Int63n(30000)
		step := 1 + rand.Int63n(30000)

		rng1 := New5MinStepRange(from, to, step)
		rng2 := New5MinStepRange(rng1.From, rng1.To, rng1.Step)

		if !reflect.DeepEqual(rng1, rng2) {
			t.Errorf("step ranges must be equal: initial=%v, secondary=%v", rng1, rng2)
		}
	}
}
