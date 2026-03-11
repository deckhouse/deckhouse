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

package reconcilers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/ctrlname"
	"github.com/deckhouse/node-controller/internal/dynctrl"
)

func init() {
	dynctrl.RegisterController(ctrlname.NodeGroupStatus, &deckhousev1.NodeGroup{}, &Status{})
}

var _ dynctrl.Reconciler = (*Status)(nil)

type Status struct {
	dynctrl.Base
}

func (r *Status) SetupWatches(w dynctrl.Watcher) {
	w.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
			return []reconcile.Request{}
		}),
	)
}

func (r *Status) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
