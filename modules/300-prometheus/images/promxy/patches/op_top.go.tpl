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

// ExtractOptTop checks op_top if it is top level aggregate expr and returns new expression without op_top and result modifier.
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
	case 0:
	case 1:
		includeOther, _ = strconv.ParseBool(strings.ToLower(node.Grouping[0]))
	case 2:
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

func calculatePointsRequired(start, stop, step int64) int {
	if step == 0 {
		return 1
	}
	req := (stop - start) / step
	return int(req) + 1
}

func calculatePointInd(start, step, timestamp int64) int {
	if step == 0 {
		return 0
	}
	return int((timestamp - start) / step)
}

func markedAsOtherSeries(ls labels.Labels) bool {
	marked := false
	for _, l := range ls {
		if l.Value == "~other" {
			marked = true
		}
	}
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

func weightBySum(samples []Point) (res float64) {
	for _, sample := range samples {
		res += nonEmptyValue(sample.V)
	}
	return
}

func weightByMax(samples []Point) (res float64) {
	for _, sample := range samples {
		if sample.V > res {
			res = nonEmptyValue(sample.V)
		}
	}
	return
}

const magicExp = 1.001

func weightByExp(samples []Point) (res float64) {
	for sampleInd := range samples {
		res += nonEmptyValue(samples[sampleInd].V) * math.Pow(magicExp, float64(sampleInd))
	}
	res = nonEmptyValue(res / float64(len(samples)))
	return
}

func weightByEws(samples []Point) (res float64) {
	for sampleInd := range samples {
		res += nonEmptyValue(samples[sampleInd].V) * math.Pow(magicExp, float64(sampleInd))
	}
	return
}

func weightByEwm(samples []Point) (res float64) {
	for sampleInd := range samples {
		val := nonEmptyValue(samples[sampleInd].V) * math.Pow(magicExp, float64(sampleInd))
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
	var weightFunc func([]Point) float64
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
			weight: weightFunc(initial[streamInd].Points),
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
			otherSamples   = make([]Point, otherSamplesCount)
			otherLabelsRaw = map[string]string{}
		)
		currentTimestamp := start
		for i := 0; i < otherSamplesCount; i++ {
			otherSamples[i].T = currentTimestamp
			currentTimestamp += step
		}
		var labelsFilled bool
		for _, index := range auxiliaryIndices {
			for _, sample := range initial[index.index].Points {
				otherSampleInd := calculatePointInd(start, step, sample.T)
				otherSamples[otherSampleInd].V += nonEmptyValue(sample.V)
			}
			if labelsFilled {
				continue
			}
			for _, l := range initial[index.index].Metric {
				if l.Name == "__name__" {
					otherLabelsRaw[l.Name] = l.Value
				} else {
					otherLabelsRaw[l.Name] = "~other"
				}
			}
			labelsFilled = true
		}

		otherLabelsList := make(labels.Labels, 0, len(otherLabelsRaw))
		for label, labelValue := range otherLabelsRaw {
			otherLabelsList = append(otherLabelsList, labels.Label{Name: label, Value: labelValue})
		}
		sort.Sort(otherLabelsList)

		otherSeries := Series{
			Metric: otherLabelsList,
			Points: otherSamples,
		}
		res = append(res, otherSeries)
	}
	return res
}
