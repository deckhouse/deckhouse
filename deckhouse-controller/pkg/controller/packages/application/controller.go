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

package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controllerName = "d8-application-controller"
)

// PackageEvent represents an event about a package.
type PackageEvent struct {
	PackageName string
	Name        string
	Namespace   string
}

type ApplicationReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Log    *slog.Logger
	events chan PackageEvent
}

func RegisterController(mgr manager.Manager, logger *slog.Logger) error {
	events := make(chan PackageEvent, 1024)

	pkgOpLogger := log.NewLogger().Named("package-operator")
	pkgOp := applicationpackage.NewPackageOperator(pkgOpLogger)

	r := &ApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    logger,
		events: events,
	}

	workerLogger := log.NewLogger().Named("packagestatus")
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		w := newPackageStatusWorker(
			workerLogger,
			r.Client,
			pkgOp,
			events,
			4,
		)
		go w.run(ctx)
		<-ctx.Done()
		return nil
	})); err != nil {
		return fmt.Errorf("add package status worker: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Application{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
		Complete(r)
}

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var app v1alpha1.Application
	if err := r.Client.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ev := PackageEvent{
		PackageName: app.Spec.ApplicationPackageName,
		Name:        app.Name,
		Namespace:   app.Namespace,
	}
	select {
	case r.events <- ev:
	default:
		r.Log.Warn("packagestatus event queue is full; dropping",
			slog.String("namespace", ev.Namespace),
			slog.String("app", ev.Name),
			slog.String("package", ev.PackageName))
	}

	return ctrl.Result{}, nil
}

// packageStatusWorker processes package events and updates Application status conditions.
type packageStatusWorker struct {
	log    *log.Logger
	kube   client.Client
	op     applicationpackage.PackageStatusOperator
	events <-chan PackageEvent

	q       workqueue.RateLimitingInterface
	mu      sync.Mutex
	last    map[string]PackageEvent
	workers int
}

func newPackageStatusWorker(
	log *log.Logger,
	kube client.Client,
	op applicationpackage.PackageStatusOperator,
	events <-chan PackageEvent,
	workers int,
) *packageStatusWorker {
	if workers < 1 {
		workers = 1
	}
	return &packageStatusWorker{
		log:     log,
		kube:    kube,
		op:      op,
		events:  events,
		q:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		workers: workers,
		last:    make(map[string]PackageEvent),
	}
}

func (w *packageStatusWorker) run(ctx context.Context) {
	// listener
	go func() {
		for {
			select {
			case <-ctx.Done():
				w.q.ShutDownWithDrain()
				return
			case ev := <-w.events:
				key := ev.Namespace + "/" + ev.Name
				w.mu.Lock()
				w.last[key] = ev
				w.mu.Unlock()
				w.q.Add(key)
			}
		}
	}()

	// workers
	var wg sync.WaitGroup
	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w.processOne(ctx) {
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
}

func (w *packageStatusWorker) processOne(ctx context.Context) bool {
	item, shutdown := w.q.Get()
	if shutdown {
		return false
	}
	key := item.(string)
	defer w.q.Done(item)

	w.mu.Lock()
	ev := w.last[key]
	delete(w.last, key)
	w.mu.Unlock()

	if err := w.sync(ctx, ev); err != nil {
		if strings.Contains(err.Error(), "not implemented") {
			w.log.Info("package status operator not implemented; skip",
				slog.String("key", key))
			w.q.Forget(item)
			return true
		}

		w.log.Error("sync failed",
			slog.String("err", err.Error()),
			slog.String("key", key))
		w.q.AddRateLimited(key)
	} else {
		w.q.Forget(item)
	}
	return true
}

func (w *packageStatusWorker) sync(ctx context.Context, ev PackageEvent) error {
	var app v1alpha1.Application
	key := client.ObjectKey{Namespace: ev.Namespace, Name: ev.Name}
	if err := w.kube.Get(ctx, key, &app); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get application: %w", err)
	}

	orig := app.DeepCopy()

	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	sts, err := w.op.GetApplicationStatus(statusCtx, ev.PackageName, ev.Name, ev.Namespace)
	if err != nil {
		return fmt.Errorf("get application status: %w", err)
	}

	newConds := mapPackageStatuses(sts, w.log, ev)
	app.Status.Conditions = mergeConditions(app.Status.Conditions, newConds)

	sortConditions(orig.Status.Conditions)
	sortConditions(app.Status.Conditions)

	if conditionsEqual(orig.Status.Conditions, app.Status.Conditions) {
		return nil
	}

	err = w.kube.Status().Patch(ctx, &app, client.MergeFrom(orig))
	if err != nil {
		if apierrors.IsConflict(err) {
			return w.retrySync(ctx, ev)
		}
		return fmt.Errorf("patch application status: %w", err)
	}

	return nil
}

func (w *packageStatusWorker) retrySync(ctx context.Context, ev PackageEvent) error {
	var app v1alpha1.Application
	key := client.ObjectKey{Namespace: ev.Namespace, Name: ev.Name}
	if err := w.kube.Get(ctx, key, &app); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get application on retry: %w", err)
	}
	orig := app.DeepCopy()

	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	sts, err := w.op.GetApplicationStatus(statusCtx, ev.PackageName, ev.Name, ev.Namespace)
	if err != nil {
		return fmt.Errorf("get application status on retry: %w", err)
	}

	newConds := mapPackageStatuses(sts, w.log, ev)
	app.Status.Conditions = mergeConditions(app.Status.Conditions, newConds)

	sortConditions(orig.Status.Conditions)
	sortConditions(app.Status.Conditions)

	if conditionsEqual(orig.Status.Conditions, app.Status.Conditions) {
		return nil
	}

	err = w.kube.Status().Patch(ctx, &app, client.MergeFrom(orig))
	if err != nil {
		return fmt.Errorf("patch application status on retry: %w", err)
	}

	return nil
}

// mapPackageStatuses converts PackageStatus slice to ApplicationStatusCondition slice.
func mapPackageStatuses(src []applicationpackage.PackageStatus, logger *log.Logger, ev PackageEvent) []v1alpha1.ApplicationStatusCondition {
	out := make([]v1alpha1.ApplicationStatusCondition, 0, len(src))
	for _, s := range src {
		t := normalizeType(s.Type)
		if t == "" {
			logger.Warn("unknown package status type, skipping",
				slog.String("type", s.Type),
				slog.String("namespace", ev.Namespace),
				slog.String("name", ev.Name),
				slog.String("package", ev.PackageName))
			continue
		}

		cond := v1alpha1.ApplicationStatusCondition{
			Type:    t,
			Status:  boolToCondStatus(s.Status),
			Reason:  s.Reason,
			Message: s.Message,
		}
		out = append(out, cond)
	}
	return out
}

// normalizeType converts package status type to Application condition type.
func normalizeType(pkgType string) string {
	mapping := map[string]string{
		"requirementsMet":        v1alpha1.ApplicationConditionRequirementsMet,
		"startupHooksSuccessful": v1alpha1.ApplicationConditionStartupHooksSuccessful,
		"manifestsDeployed":      v1alpha1.ApplicationConditionManifestsDeployed,
		"replicasAvailable":      v1alpha1.ApplicationConditionReplicasAvailable,
	}

	if mapped, ok := mapping[pkgType]; ok {
		return mapped
	}

	return ""
}

// boolToCondStatus converts bool to corev1.ConditionStatus.
func boolToCondStatus(b bool) corev1.ConditionStatus {
	if b {
		return corev1.ConditionTrue
	}
	return corev1.ConditionFalse
}

// mergeConditions merges incoming conditions into existing ones.
func mergeConditions(existing, incoming []v1alpha1.ApplicationStatusCondition) []v1alpha1.ApplicationStatusCondition {
	idx := make(map[string]int)
	for i, c := range existing {
		idx[c.Type] = i
	}

	now := metav1.Now()
	res := make([]v1alpha1.ApplicationStatusCondition, len(existing))
	copy(res, existing)

	for _, inc := range incoming {
		if i, ok := idx[inc.Type]; ok {
			cur := res[i]
			statusChanged := cur.Status != inc.Status

			if statusChanged {
				cur.Status = inc.Status
				cur.LastTransitionTime = now
			}
			if cur.Reason != inc.Reason {
				cur.Reason = inc.Reason
			}
			if cur.Message != inc.Message {
				cur.Message = inc.Message
			}
			cur.LastProbeTime = now

			res[i] = cur
		} else {
			inc.LastTransitionTime = now
			inc.LastProbeTime = now
			res = append(res, inc)
		}
	}

	sortConditions(res)

	return res
}

// sortConditions sorts conditions by Type.
func sortConditions(conds []v1alpha1.ApplicationStatusCondition) {
	sort.Slice(conds, func(i, j int) bool {
		return conds[i].Type < conds[j].Type
	})
}

// conditionsEqual performs semantic comparison of conditions after sorting.
func conditionsEqual(a, b []v1alpha1.ApplicationStatusCondition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type ||
			a[i].Status != b[i].Status ||
			a[i].Reason != b[i].Reason ||
			a[i].Message != b[i].Message {
			return false
		}
	}
	return true
}
