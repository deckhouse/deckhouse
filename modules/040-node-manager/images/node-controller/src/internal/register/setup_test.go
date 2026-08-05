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

package register

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
)

// plainReconciler takes whatever concurrency it is given.
type plainReconciler struct{}

func (plainReconciler) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
func (plainReconciler) SetupWatches(Watcher) {}

// cappedReconciler is only correct below a number of workers it holds itself.
type cappedReconciler struct {
	cap int
}

func (cappedReconciler) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
func (cappedReconciler) SetupWatches(Watcher)           {}
func (r cappedReconciler) MaxConcurrentReconciles() int { return r.cap }

// A controller whose correctness depends on the number of workers holds that
// number in code. Left to the deployment argument, one typo in any segment of
// --max-concurrent-reconciles discards the whole per-controller map and the
// controller silently runs with the shared default.
func TestEffectiveMaxConcurrentReconciles(t *testing.T) {
	tests := []struct {
		name       string
		reconciler Reconciler
		requested  int
		expWorkers int
	}{
		{
			name:       "a reconciler with no opinion takes what it is given",
			reconciler: plainReconciler{},
			requested:  10,
			expWorkers: 10,
		},
		{
			name:       "a reconciler that caps itself wins over the flag",
			reconciler: cappedReconciler{cap: 1},
			requested:  10,
			expWorkers: 1,
		},
		{
			name:       "the cap does not raise a lower request",
			reconciler: cappedReconciler{cap: 4},
			requested:  2,
			expWorkers: 2,
		},
		{
			name:       "a meaningless cap is ignored",
			reconciler: cappedReconciler{cap: 0},
			requested:  10,
			expWorkers: 10,
		},
		{
			name:       "a meaningless request still runs one worker",
			reconciler: plainReconciler{},
			requested:  0,
			expWorkers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expWorkers, effectiveMaxConcurrentReconciles(tt.reconciler, tt.requested))
		})
	}
}
