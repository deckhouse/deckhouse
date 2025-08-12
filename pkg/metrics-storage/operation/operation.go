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

func ValidateOperations(ops ...MetricOperation) error {
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
		opErrs = multierror.Append(opErrs, fmt.Errorf("one of: 'action', 'set' or 'add' is required: %s", op.Action.String()))
	}

	if op.Group == "" {
		if op.Action == ActionExpireMetrics {
			opErrs = multierror.Append(opErrs, fmt.Errorf("unsupported action '%s'", op.Action.String()))
		}
	}

	if op.Name == "" && op.Group == "" {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required: %s", op.Action.String()))
	}

	if op.Name == "" && op.Group != "" && op.Action != ActionExpireMetrics {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required when action is not 'expire': %s", op.Action.String()))
	}

	if (op.Action == ActionCounterAdd || op.Action == ActionGaugeAdd || op.Action == ActionGaugeSet) && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action '%s'", op.Action.String()))
	}

	if op.Action == ActionHistogramObserve && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action '%s'", op.Action.String()))
	}

	if op.Action == ActionHistogramObserve && op.Buckets == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'buckets' is required for action '%s'", op.Action.String()))
	}

	return opErrs.ErrorOrNil()
}
