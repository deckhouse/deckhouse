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

package packagestatusservice

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// PackageEvent represents an event about a package from PackageOperator.
type PackageEvent struct {
	PackageName string
	Name        string
	Namespace   string
	Version     string
	Type        string
}

// Service processes package events and updates Application status conditions.
//
// The service listens to PackageEvent channel, queries PackageOperator for package statuses,
// maps them to Application conditions, and patches the Application status subresource.
type Service struct {
	log     *log.Logger
	kube    client.Client
	op      applicationpackage.PackageStatusOperator
	events  <-chan PackageEvent
	metrics metricsstorage.Storage

	q          workqueue.RateLimitingInterface
	wg         sync.WaitGroup
	workers    int
	eventMap   map[string]PackageEvent // latest event per key for coalescing
	eventMapMu sync.Mutex              // protects eventMap
}

func NewPackageStatusService(
	log *log.Logger,
	kube client.Client,
	op applicationpackage.PackageStatusOperator,
	events <-chan PackageEvent,
	metrics metricsstorage.Storage,
	workers int,
) *Service {
	if workers <= 0 {
		workers = 2
	}

	return &Service{
		log:      log.Named("packagestatus"),
		kube:     kube,
		op:       op,
		events:   events,
		metrics:  metrics,
		q:        workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		workers:  workers,
		eventMap: make(map[string]PackageEvent),
	}
}

// Run starts the service and blocks until context is cancelled.
func (s *Service) Run(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				s.log.Info("stopping event listener")
				return
			case ev, ok := <-s.events:
				if !ok {
					s.log.Info("events channel closed")
					return
				}
				key := fmt.Sprintf("%s/%s", ev.Namespace, ev.Name)
				// Store latest event for coalescing
				s.eventMapMu.Lock()
				s.eventMap[key] = ev
				s.eventMapMu.Unlock()
				// Add only key to queue (coalescing works on string keys)
				s.q.Add(key)
			}
		}
	}()

	// Start workers
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go func(workerID int) {
			defer s.wg.Done()
			s.log.Debug("worker started", slog.Int("worker", workerID))
			for s.processOne(ctx) {
			}
			s.log.Debug("worker stopped", slog.Int("worker", workerID))
		}(i)
	}

	<-ctx.Done()
	s.log.Info("shutting down service")

	// Shutdown queue and wait for workers
	s.q.ShutDownWithDrain()
	s.wg.Wait()

	s.log.Info("service stopped")
}

func (s *Service) processOne(ctx context.Context) bool {
	item, shutdown := s.q.Get()
	if shutdown {
		return false
	}
	defer s.q.Done(item)

	key := item.(string)

	s.eventMapMu.Lock()
	ev, ok := s.eventMap[key]
	if !ok {
		s.eventMapMu.Unlock()
		s.q.Forget(key)
		return true
	}
	delete(s.eventMap, key)
	s.eventMapMu.Unlock()

	if err := s.sync(ctx, ev); err != nil {
		s.log.Error("sync failed",
			slog.String("key", key),
			slog.String("namespace", ev.Namespace),
			slog.String("name", ev.Name),
			slog.String("package", ev.PackageName),
			log.Err(err))
		s.recordEventMetric("error")
		s.eventMapMu.Lock()
		s.eventMap[key] = ev
		s.eventMapMu.Unlock()
		s.q.AddRateLimited(key)
		return true
	}

	s.recordEventMetric("ok")
	s.q.Forget(key)
	return true
}

func (s *Service) sync(ctx context.Context, ev PackageEvent) error {
	var app v1alpha1.Application
	key := client.ObjectKey{Namespace: ev.Namespace, Name: ev.Name}
	if err := s.kube.Get(ctx, key, &app); err != nil {
		if apierrors.IsNotFound(err) {
			s.log.Debug("application not found, dropping event",
				slog.String("namespace", ev.Namespace),
				slog.String("name", ev.Name))
			return nil
		}
		return fmt.Errorf("get application: %w", err)
	}

	if app.Spec.ApplicationPackageName != ev.PackageName {
		s.log.Warn("package name mismatch",
			slog.String("namespace", ev.Namespace),
			slog.String("name", ev.Name),
			slog.String("eventPackage", ev.PackageName),
			slog.String("appPackage", app.Spec.ApplicationPackageName))
	}

	orig := app.DeepCopy()

	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	startTime := time.Now()
	sts, err := s.op.GetApplicationStatus(statusCtx, ev.PackageName, ev.Name, ev.Namespace)
	duration := time.Since(startTime).Seconds()

	if err != nil {
		s.recordStatusFetchDuration(duration)
		return fmt.Errorf("get application status: %w", err)
	}

	s.recordStatusFetchDuration(duration)

	newConds := mapPackageStatuses(sts, s.log, ev)
	app.Status.Conditions = mergeConditions(app.Status.Conditions, newConds)

	sortConditions(orig.Status.Conditions)
	sortConditions(app.Status.Conditions)

	if conditionsEqual(orig.Status.Conditions, app.Status.Conditions) {
		s.log.Debug("no changes to conditions, skipping patch",
			slog.String("namespace", ev.Namespace),
			slog.String("name", ev.Name))
		return nil
	}

	err = s.kube.Status().Patch(ctx, &app, client.MergeFrom(orig))
	if err != nil {
		if apierrors.IsConflict(err) {
			s.log.Debug("conflict during patch, retrying",
				slog.String("namespace", ev.Namespace),
				slog.String("name", ev.Name))
			s.recordPatchMetric("conflict")
			return s.retrySync(ctx, ev)
		}
		s.recordPatchMetric("error")
		return fmt.Errorf("patch application status: %w", err)
	}

	s.recordPatchMetric("ok")

	s.log.Info("updated application conditions",
		slog.String("namespace", ev.Namespace),
		slog.String("name", ev.Name),
		slog.String("package", ev.PackageName),
		slog.Int("conditions", len(app.Status.Conditions)))

	return nil
}

func (s *Service) retrySync(ctx context.Context, ev PackageEvent) error {
	var app v1alpha1.Application
	key := client.ObjectKey{Namespace: ev.Namespace, Name: ev.Name}
	if err := s.kube.Get(ctx, key, &app); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get application on retry: %w", err)
	}
	orig := app.DeepCopy()

	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	sts, err := s.op.GetApplicationStatus(statusCtx, ev.PackageName, ev.Name, ev.Namespace)
	if err != nil {
		return fmt.Errorf("get application status on retry: %w", err)
	}

	newConds := mapPackageStatuses(sts, s.log, ev)
	app.Status.Conditions = mergeConditions(app.Status.Conditions, newConds)

	sortConditions(orig.Status.Conditions)
	sortConditions(app.Status.Conditions)

	if conditionsEqual(orig.Status.Conditions, app.Status.Conditions) {
		return nil
	}

	err = s.kube.Status().Patch(ctx, &app, client.MergeFrom(orig))
	if err != nil {
		s.recordPatchMetric("error")
		return fmt.Errorf("patch application status on retry: %w", err)
	}

	s.recordPatchMetric("ok")
	return nil
}

// recordEventMetric records an event processing metric.
func (s *Service) recordEventMetric(result string) {
	if s.metrics == nil {
		return
	}
	s.metrics.CounterAdd(metrics.PackageStatusEventsTotal, 1, map[string]string{"result": result})
}

// recordStatusFetchDuration records the duration of a status fetch operation.
func (s *Service) recordStatusFetchDuration(seconds float64) {
	if s.metrics == nil {
		return
	}
	s.metrics.HistogramObserve(metrics.PackageStatusStatusFetchSeconds, seconds, map[string]string{}, nil)
}

// recordPatchMetric records a patch operation metric.
func (s *Service) recordPatchMetric(result string) {
	if s.metrics == nil {
		return
	}
	s.metrics.CounterAdd(metrics.PackageStatusPatchesTotal, 1, map[string]string{"result": result})
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
// Returns empty string for unknown types.
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

	// Merge incoming conditions
	for _, inc := range incoming {
		if i, ok := idx[inc.Type]; ok {
			// Update existing condition
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
			// Add new condition
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
// Compares Type, Status, Reason, and Message
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

type RunnableService struct {
	*Service
}

func (r *RunnableService) Start(ctx context.Context) error {
	r.Run(ctx)
	return nil
}

func RegisterService(
	mgr manager.Manager,
	op applicationpackage.PackageStatusOperator,
	events <-chan PackageEvent,
	metrics metricsstorage.Storage,
	logger *log.Logger,
	workers int,
) error {
	service := NewPackageStatusService(logger, mgr.GetClient(), op, events, metrics, workers)
	runnable := &RunnableService{Service: service}
	return mgr.Add(runnable)
}
