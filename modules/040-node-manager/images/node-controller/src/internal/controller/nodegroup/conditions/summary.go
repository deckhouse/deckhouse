/*
Copyright 2026 Flant JSC

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

package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func CalculateConditionSummary(_ []metav1.Condition, statusMsg string) *v1.ConditionSummary {
	ready := "True"
	if len(statusMsg) > 0 {
		ready = "False"
	}

	return &v1.ConditionSummary{
		Ready:         ready,
		StatusMessage: statusMsg,
	}
}
