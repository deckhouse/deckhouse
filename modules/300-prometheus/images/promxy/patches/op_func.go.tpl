/*
Copyright 2025 Flant JSC

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

package promql

import (
	"math"
	"time"

	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/promql/parser"
)

const (
	opDefined = "op_defined"
)

// RegisterOPDefined registers promql `op_defined` function.
func RegisterOPDefined() {
	parser.Functions[opDefined] = &parser.Function{
		Name:       opDefined,
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix},
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls[opDefined] = funcOPDefined
}

func funcOPDefined(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) Vector {
	vec := vals[0].(Matrix)
	for _, el := range vec {
		var v float64 = 1
		if isNone(takeLast(el), enh) {
			v = 0
		}
		enh.Out = append(enh.Out, Sample{
			Metric: dropMetricName(el.Metric),
			Point:  Point{V: v},
		})
	}
	return enh.Out
}

const (
	opReplaceNaN = "op_replace_nan"
)

// RegisterOPReplaceNaN registers promql `op_replace_nan` function.
func RegisterOPReplaceNaN() {
	parser.Functions[opReplaceNaN] = &parser.Function{
		Name:       opReplaceNaN,
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix, parser.ValueTypeScalar, parser.ValueTypeScalar},
		Variadic:   1,
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls[opReplaceNaN] = funcOPReplaceNaN
}

func funcOPReplaceNaN(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) Vector {
	vec := vals[0].(Matrix)
	val := vals[1].(Vector)[0].V
	var ms int64 = math.MinInt64
	if len(vals) > 2 {
		ms = int64(vals[2].(Vector)[0].V)
	}
	for _, el := range vec {
		p := takeLast(el)
		if p.T < ms {
			continue
		}
		v := p.V
		if isNone(p, enh) || math.IsNaN(v) {
			v = val
		}
		enh.Out = append(enh.Out, Sample{
			Metric: dropMetricName(el.Metric),
			Point:  Point{V: v},
		})
	}
	return enh.Out
}

const (
	OpSmoothie = "op_smoothie"
)

// RegisterOPSmoothie registers promql `op_smoothie` function.
func RegisterOPSmoothie() {
	parser.Functions[OpSmoothie] = &parser.Function{
		Name:       OpSmoothie,
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix},
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls[OpSmoothie] = funcOPSmoothie
}

func funcOPSmoothie(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) Vector {
	return opAggrOverTime(vals, enh, func(values []Point) float64 {
		var mean, count, c float64
		if len(values) == 0 {
			return math.NaN()
		}
		v := values[len(values)-1]
		if math.IsNaN(v.V) {
			return v.V
		}
		if enh.Ts != v.T {
			return math.Float64frombits(value.StaleNaN)
		}
		for _, v := range values {
			count++
			if math.IsInf(mean, 0) {
				if math.IsInf(v.V, 0) && (mean > 0) == (v.V > 0) {
					continue
				}
				if !math.IsInf(v.V, 0) && !math.IsNaN(v.V) {
					continue
				}
			}
			mean, c = kahanSumInc(v.V/count-mean/count, mean, c)
		}

		if math.IsInf(mean, 0) {
			return mean
		}
		return mean + c
	})
}

func opAggrOverTime(vals []parser.Value, enh *EvalNodeHelper, aggrFn func([]Point) float64) Vector {
	el := vals[0].(Matrix)[0]
	v := aggrFn(el.Points)
	if value.IsStaleNaN(v) {
		return enh.Out
	}

	return append(enh.Out, Sample{
		Point: Point{V: v},
	})
}

const (
	opZeroIfNone = "op_zero_if_none"
)

// RegisterOPZeroIfNone registers promql `op_zero_if_none` function.
func RegisterOPZeroIfNone() {
	parser.Functions[opZeroIfNone] = &parser.Function{
		Name:       opZeroIfNone,
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix, parser.ValueTypeScalar},
		Variadic:   1,
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls[opZeroIfNone] = funcOPZeroIfNone
}

func funcOPZeroIfNone(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) Vector {
	vec := vals[0].(Matrix)
	var ms int64 = math.MinInt64
	if len(vals) > 1 {
		ms = int64(vals[1].(Vector)[0].V)
	}
	for _, el := range vec {
		p := takeLast(el)
		if p.T < ms {
			continue
		}
		v := p.V
		if isNone(p, enh) || math.IsNaN(v) {
			v = 0
		}
		enh.Out = append(enh.Out, Sample{
			Metric: dropMetricName(el.Metric),
			Point:  Point{V: v},
		})
	}
	return enh.Out
}

// ActualInterval period in which series is not lost.
const ActualInterval = int64(time.Minute / time.Millisecond)

func takeLast(series Series) Point {
	return series.Points[len(series.Points)-1]
}

func isNone(p Point, enh *EvalNodeHelper) bool {
	return enh.Ts-p.T > ActualInterval
}
