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
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

func init() {
	RegisterOPDefined()
	RegisterOPZeroIfNone()
	RegisterOPReplaceNaN()
	RegisterOPSmoothie()
}

// OP_FUNCTIONS

// ExtractOptTop returns a stripped expression and a result modifier when the
// top-level aggregate is op_top. Otherwise the original expression is
// returned unchanged and the modifier is nil.
//
// We deliberately don't traverse the AST: promxy's parser.Walk fans children
// out into goroutines for read-only visits, which would make modifier-state
// updates racy. op_top is only supported on the top level — nested usage is
// left to fail naturally with a generic "no FunctionCall for op_top" error.
func ExtractOptTop(expr parser.Expr, start, end, step int64) (parser.Expr, func(matrix Matrix) Matrix, error) {
	aggExpr, ok := expr.(*parser.AggregateExpr)
	if !ok || aggExpr.Op != parser.OP_TOP {
		return expr, nil, nil
	}

	opTopResultModifier, err := newOPTop(aggExpr, start, end, step)
	if err != nil {
		return nil, nil, err
	}

	return aggExpr.Expr, opTopResultModifier, nil
}

func newOPTop(node *parser.AggregateExpr, start, stop, step int64) (func(Matrix) Matrix, error) {
	var (
		weightFunc   weightFuncKind
		limit        int
		includeOther bool
	)
	limit, _ = strconv.Atoi(node.Param.String())
	switch len(node.Grouping) {
	case 0: // op_top($expr) or op_top($limit, $expr)
	case 1: // op_top($includeOther, $expr) or op_top($limit, $includeOther, $expr)
		includeOther, _ = strconv.ParseBool(strings.ToLower(node.Grouping[0]))
	case 2: // op_top($includeOther, $weightFunc, $expr) or op_top($limit, $includeOther, $weightFunc, $expr)
		includeOther, _ = strconv.ParseBool(strings.ToLower(node.Grouping[0]))
		weightFunc = weightFuncKind(node.Grouping[1])
	}
	params := &OPTopQueryParams{
		weightFunc:   weightFunc,
		includeOther: includeOther,
		limit:        limit,
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit value must be set")
	}
	return func(initial Matrix) Matrix {
		return OPTop(params, initial, start, stop, step)
	}, nil
}

type OPTopQueryParams struct {
	weightFunc   weightFuncKind
	includeOther bool
	limit        int
}

// calculatePointsRequired returns required amount of data-points for current start, stop, and end
// ex:
//
//	step = 10
//	---------
//	[start=0]       1
//	[start+step=10] 2
//	[start+step=20] 3
//	[start+step=30] 4
//	[stop=40]       5
func calculatePointsRequired(start, stop, step int64) int {
	if step == 0 {
		return 1
	}
	req := (stop - start) / step
	return int(req) + 1
}

// calculatePointInd returns point index in series by given timestamp
// ex:
//
//	    timestamp = 30
//	    step      = 10
//		---------
//		[start=0]       1 ind=0
//		[start+step=10] 2 ind=1
//		[start+step=20] 3 ind=2
//		[start+step=30] 4 ind=3 <- at timestamp = 30
//		[stop=40]       5 ind=3
//
// so (30-0)/10 = 3.
func calculatePointInd(start, step, timestamp int64) int {
	if step == 0 {
		return 0
	}
	return int((timestamp - start) / step)
}

func markedAsOtherSeries(ls labels.Labels) bool {
	marked := false
	ls.Range(func(l labels.Label) {
		if l.Value == "~other" {
			marked = true
		}
	})
	return marked
}

type weightFuncKind string

const (
	weightFuncSum weightFuncKind = "sum"
	weightFuncMax weightFuncKind = "max"
	weightFuncExp weightFuncKind = "exp"
	weightFuncEws weightFuncKind = "ews"
	weightFuncEwm weightFuncKind = "ewn"
)

func nonEmptyValue(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

func weightBySum(samples []FPoint) (res float64) {
	for _, sample := range samples {
		res += nonEmptyValue(sample.F)
	}
	return
}

func weightByMax(samples []FPoint) (res float64) {
	for _, sample := range samples {
		if sample.F > res {
			res = nonEmptyValue(sample.F)
		}
	}
	return
}

const magicExp = 1.001

func weightByExp(samples []FPoint) (res float64) {
	for sampleInd := range samples {
		res += nonEmptyValue(samples[sampleInd].F) * math.Pow(magicExp, float64(sampleInd))
	}
	res = nonEmptyValue(res / float64(len(samples)))
	return
}

func weightByEws(samples []FPoint) (res float64) {
	for sampleInd := range samples {
		res += nonEmptyValue(samples[sampleInd].F) * math.Pow(magicExp, float64(sampleInd))
	}
	return
}

func weightByEwm(samples []FPoint) (res float64) {
	for sampleInd := range samples {
		val := nonEmptyValue(samples[sampleInd].F) * math.Pow(magicExp, float64(sampleInd))
		if val > res {
			res = val
		}
	}
	return
}

type member struct {
	index  int
	weight float64
}

type memberData struct {
	members []member
}

func (m memberData) Len() int           { return len(m.members) }
func (m memberData) Less(i, j int) bool { return m.members[i].weight < m.members[j].weight }
func (m memberData) Swap(i, j int)      { m.members[i], m.members[j] = m.members[j], m.members[i] }

func OPTop(params *OPTopQueryParams, initial Matrix, start, stop, step int64) Matrix {
	if len(initial) == 0 {
		return initial
	}
	var weightFunc func([]FPoint) float64
	switch params.weightFunc {
	case weightFuncSum:
		weightFunc = weightBySum
	case weightFuncMax:
		weightFunc = weightByMax
	case weightFuncExp:
		weightFunc = weightByExp
	case weightFuncEws:
		weightFunc = weightByEws
	case weightFuncEwm:
		weightFunc = weightByEwm
	default:
		weightFunc = weightByEws
	}
	m := memberData{members: make([]member, 0, len(initial))}
	for streamInd := range initial {
		m.members = append(m.members, member{
			index:  streamInd,
			weight: weightFunc(initial[streamInd].Floats),
		})
	}
	limit := params.limit
	if limit <= 0 || len(m.members) < limit {
		limit = len(m.members)
	}
	otherSeriesRequired := params.includeOther && limit < len(m.members)
	for memberInd := range m.members {
		initialLookupInd := m.members[memberInd].index
		if markedAsOtherSeries(initial[initialLookupInd].Metric) && otherSeriesRequired {
			m.members[memberInd].weight = 0
		}
	}
	sort.Sort(sort.Reverse(m))
	res := make(Matrix, 0, limit+1)
	mainIndices := m.members[:limit]
	for _, index := range mainIndices {
		res = append(res, initial[index.index])
	}
	if otherSeriesRequired {
		auxiliaryIndices := m.members[limit:]
		otherSamplesCount := calculatePointsRequired(start, stop, step)
		var (
			otherSamples   = make([]FPoint, otherSamplesCount)
			otherLabelsRaw = map[string]string{}
		)
		currentTimestamp := start
		for i := 0; i < otherSamplesCount; i++ {
			otherSamples[i].T = currentTimestamp
			currentTimestamp += step
		}
		var labelsFilled bool
		for _, index := range auxiliaryIndices {
			for _, sample := range initial[index.index].Floats {
				otherSampleInd := calculatePointInd(start, step, sample.T)
				otherSamples[otherSampleInd].F += nonEmptyValue(sample.F)
			}
			if labelsFilled {
				continue
			}

			initial[index.index].Metric.Range(func(l labels.Label) {
				if l.Name == "__name__" {
					otherLabelsRaw[l.Name] = l.Value
				} else {
					otherLabelsRaw[l.Name] = "~other"
				}
			})
			labelsFilled = true
		}

		lsb := labels.NewScratchBuilder(len(otherLabelsRaw))
		for label, labelValue := range otherLabelsRaw {
			lsb.Add(label, labelValue)
		}

		otherSeries := Series{
			Metric: lsb.Labels(),
			Floats: otherSamples,
		}

		res = append(res, otherSeries)
	}
	return res
}
