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

package readiness

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExistsResourceWithoutChecker
// use for resources without special checks
// resources was get before run check and we do not need check another
type ExistsResourceWithoutChecker struct {
	loggerProvider LoggerProvider
}

func NewExistsResourceWithoutChecker(loggerProvider LoggerProvider) *ExistsResourceWithoutChecker {
	return &ExistsResourceWithoutChecker{
		loggerProvider: loggerProvider,
	}
}

func (e *ExistsResourceWithoutChecker) WaitAttemptsBeforeCheck() int {
	// prevent api server associations
	return 1
}

func (e *ExistsResourceWithoutChecker) IsReady(_ context.Context, _ *unstructured.Unstructured, resourceName string) (bool, error) {
	return debugLogAndReturnReady(safeLoggerProvider(e.loggerProvider), resourceName, "is exists and ready, because do not need special checks")
}
