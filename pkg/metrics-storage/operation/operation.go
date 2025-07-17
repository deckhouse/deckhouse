// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operation

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

type MetricOperation struct {
	Name    string
	Value   *float64
	Buckets []float64
	Labels  map[string]string
	Group   string
	Action  MetricAction
}

func (m MetricOperation) String() string {
	parts := make([]string, 0)

	if m.Group != "" {
		parts = append(parts, "group="+m.Group)
	}

	if m.Name != "" {
		parts = append(parts, "name="+m.Name)
	}

	if m.Action != "" {
		parts = append(parts, "action="+m.Action.String())
	}

	if m.Value != nil {
		parts = append(parts, fmt.Sprintf("value=%f", *m.Value))
	}

	if m.Buckets != nil {
		parts = append(parts, fmt.Sprintf("buckets=%+v", m.Buckets))
	}

	if m.Labels != nil {
		parts = append(parts, fmt.Sprintf("labels=%+v", m.Labels))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

func ValidateOperations(ops []MetricOperation) error {
	var opsErrs *multierror.Error

	for _, op := range ops {
		err := ValidateMetricOperation(op)
		if err != nil {
			opsErrs = multierror.Append(opsErrs, err)
		}
	}

	return opsErrs.ErrorOrNil()
}

func ValidateMetricOperation(op MetricOperation) error {
	var opErrs *multierror.Error

	if !op.Action.IsValid() {
		opErrs = multierror.Append(opErrs, fmt.Errorf("one of: 'action', 'set' or 'add' is required: %s", op))
	}

	if op.Group == "" {
		if op.Action != ActionOldGaugeSet && op.Action != ActionCounterAdd && op.Action != ActionHistogramObserve {
			opErrs = multierror.Append(opErrs, fmt.Errorf("unsupported action '%s': %s", op.Action, op))
		}
	} else {
		if op.Action != ActionExpireMetrics && op.Action != ActionOldGaugeSet && op.Action != ActionCounterAdd {
			opErrs = multierror.Append(opErrs, fmt.Errorf("unsupported action '%s': %s", op.Action, op))
		}
	}

	if op.Name == "" && op.Group == "" {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required: %s", op))
	}

	if op.Name == "" && op.Group != "" && op.Action != ActionExpireMetrics {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required when action is not 'expire': %s", op))
	}

	if op.Action == ActionOldGaugeSet && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'set': %s", op))
	}

	if op.Action == ActionCounterAdd && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'add': %s", op))
	}

	if op.Action == ActionHistogramObserve && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'observe': %s", op))
	}

	if op.Action == ActionHistogramObserve && op.Buckets == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'buckets' is required for action 'observe': %s", op))
	}

	return opErrs.ErrorOrNil()
}
