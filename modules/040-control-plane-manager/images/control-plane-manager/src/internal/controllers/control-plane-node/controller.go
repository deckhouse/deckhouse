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

package controlplanenode

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpnplanner"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 5 * time.Minute
)

func Register(mgr manager.Manager, metricsStorage metricsstorage.Storage) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	metricHandlers, err := newMetrics(metricsStorage)
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}

	r := &reconciler{
		client:           mgr.GetClient(),
		apiReader:        mgr.GetAPIReader(),
		scheme:           mgr.GetScheme(),
		operationBuilder: cpnplanner.NormalOperationBuilder{},
		metrics:          metricHandlers,
		log:              log.Default().With(slog.String("controller", constants.CpnControllerName)),
	}

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: nodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	// Ignore Create, Delete. React only to Update when Status changed (ignore spec-only updates).
	operationPredicate := predicate.And(
		nodeLabelPredicate,
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
				newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
				if !okOld || !okNew {
					return false
				}
				return !reflect.DeepEqual(oldOp.Status, newOp.Status)
			},
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpnControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &controlplanev1alpha1.ControlPlaneNode{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(operationPredicate),
		).
		Watches(
			&controlplanev1alpha1.ControlPlaneNode{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(nodeLabelPredicate, predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}
