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
	"github.com/prometheus/prometheus/util/annotations"
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

// === op_defined(Matrix parser.ValueTypeMatrix) Vector ===
// `op_defined` is a window function that replaces vector component value with 0
// (if value is outdated e.q older than ActualInterval + 1 minute) or 1 otherwise.
func funcOPDefined(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) (Vector, annotations.Annotations) {
	vec := vals[0].(Matrix)
	for _, el := range vec {
		var v float64 = 1
		if isNone(takeLast(el), enh) {
			v = 0
		}
		enh.Out = append(enh.Out, Sample{
			Metric: el.Metric.DropMetricName(),
			F:      v,
		})
	}
	return enh.Out, nil
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

// === op_replace_nan(Matrix parser.ValueTypeMatrix, Value parser.ValueTypeScalar, Ms parser.ValueTypeScalar) Vector ===
// `op_replace_nan` is a window function that replaces vector component value with the second parameter `Value`
// if value is outdated (older than ActualInterval + 1 minute), third parameter is used to cut off values before now-`Ms` (milliseconds).
func funcOPReplaceNaN(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) (Vector, annotations.Annotations) {
	vec := vals[0].(Matrix)
	val := vals[1].(Vector)[0].F
	var ms int64 = math.MinInt64
	if len(vals) > 2 {
		ms = int64(vals[2].(Vector)[0].F)
	}
	for _, el := range vec {
		p := takeLast(el)
		if p.T < ms {
			continue
		}
		v := p.F
		if isNone(p, enh) || math.IsNaN(v) {
			v = val
		}
		enh.Out = append(enh.Out, Sample{
			Metric: el.Metric.DropMetricName(),
			F:      v,
		})
	}
	return enh.Out, nil
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

// === op_smoothie(Matrix parser.ValueTypeMatrix) Vector ===
// `op_smoothie` is a window function that replaces vector component value with the average one over the interval.
func funcOPSmoothie(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) (Vector, annotations.Annotations) {
	return opAggrOverTime(vals, enh, func(values []FPoint) float64 {
		var mean, count, c float64
		if len(values) == 0 {
			return math.NaN()
		}
		v := values[len(values)-1]
		if math.IsNaN(v.F) {
			return v.F
		}
		if enh.Ts != v.T {
			return math.Float64frombits(value.StaleNaN)
		}
		for _, v := range values {
			count++
			if math.IsInf(mean, 0) {
				if math.IsInf(v.F, 0) && (mean > 0) == (v.F > 0) {
					// The `mean` and `v.F` values are `Inf` of the same sign. They
					// can't be subtracted, but the value of `mean` is correct
					// already.
					continue
				}
				if !math.IsInf(v.F, 0) && !math.IsNaN(v.F) {
					// At this stage, the mean is an infinite. If the added
					// value is neither an Inf or a Nan, we can keep that mean
					// value.
					// This is required because our calculation below removes
					// the mean value, which would look like Inf += x - Inf and
					// end up as a NaN.
					continue
				}
			}
			mean, c = kahanSumInc(v.F/count-mean/count, mean, c)
		}

		if math.IsInf(mean, 0) {
			return mean
		}
		return mean + c
	}), nil
}

func opAggrOverTime(vals []parser.Value, enh *EvalNodeHelper, aggrFn func([]FPoint) float64) Vector {
	el := vals[0].(Matrix)[0]
	v := aggrFn(el.Floats)
	if value.IsStaleNaN(v) {
		return enh.Out
	}

	return append(enh.Out, Sample{
		F: v,
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

// === op_zero_if_none(Matrix parser.ValueTypeMatrix, Ms parser.ValueTypeScalar) Vector ===
// `op_zero_if_none` is a window function that replaces vector component value with 0 if empty.
func funcOPZeroIfNone(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) (Vector, annotations.Annotations) {
	vec := vals[0].(Matrix)
	var ms int64 = math.MinInt64
	if len(vals) > 1 {
		ms = int64(vals[1].(Vector)[0].F)
	}
	for _, el := range vec {
		p := takeLast(el)
		if p.T < ms {
			continue
		}
		v := p.F
		if isNone(p, enh) || math.IsNaN(v) {
			v = 0
		}
		enh.Out = append(enh.Out, Sample{
			Metric: el.Metric.DropMetricName(),
			F:      v,
		})
	}
	return enh.Out, nil
}

// ActualInterval period in which series is not lost.
const ActualInterval = int64(time.Minute / time.Millisecond)

// takeLast selects last point in series.
func takeLast(series Series) FPoint {
	return series.Floats[len(series.Floats)-1]
}

// isNone checks absence in current slice.
func isNone(p FPoint, enh *EvalNodeHelper) bool {
	return enh.Ts-p.T > ActualInterval
}
