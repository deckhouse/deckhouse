/*
Copyright 2024 Flant JSC

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

package project

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	"controller/internal/impersonate"
	projectmanager "controller/internal/manager/project"
	"controller/internal/startup"
)

const controllerName = "d8-project-controller"

func Register(runtimeManager manager.Manager, helmClient *helm.Client, logger logr.Logger, migration *startup.Migration) error {
	// spec.manifests of a template is applied as the user of the project, not as the controller
	manifestsClients := impersonate.NewClients(runtimeManager.GetConfig(), runtimeManager.GetScheme(), runtimeManager.GetRESTMapper())
	r := &reconciler{
		init:    new(sync.WaitGroup),
		logger:  logger.WithName(controllerName),
		client:  runtimeManager.GetClient(),
		manager: projectmanager.New(runtimeManager.GetClient(), helmClient, logger, projectmanager.WithManifestsClients(manifestsClients)),
	}

	r.init.Add(1)

	// init project manager, it has to ensure default templates
	if err := runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return retry.OnError(
			wait.Backoff{
				Steps:    10,
				Duration: 100 * time.Millisecond,
				Factor:   2.0,
				Jitter:   0.1,
			},
			func(e error) bool {
				logger.Info("failed to init project manager - try to retry", "error", e.Error())
				return true
			},
			func() error {
				return r.manager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init, migration)
			},
		)
	})); err != nil {
		return fmt.Errorf("init project manager: %w", err)
	}

	projectController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("create project controller: %w", err)
	}

	r.logger.Info("initialize project controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha3.Project{}, builder.WithPredicates(predicate.Or(
			predicate.AnnotationChangedPredicate{},
			predicate.GenerationChangedPredicate{},
			customPredicate[client.Object]{logger: logger},
			predicate.Funcs{UpdateFunc: ownLabelsDrifted},
		))).
		Watches(&corev1.Namespace{}, namespaceProjectHandler{},
			builder.WithPredicates(namespaceWatchPredicate{})).
		Watches(&corev1.ResourceQuota{}, standardFieldHandler(v1alpha3.ProjectQuotaName)).
		// a handover of a Helm text release waits for the administrators binding to be Ready, and the
		// manifests of a template wait for the binding of the user they are applied as
		Watches(&v1alpha3.ProjectRoleBinding{}, standardFieldHandler(v1alpha3.ProjectAdministratorsBinding, v1alpha3.ProjectManifestsBinding)).
		Complete(projectController)
}

// standardFieldHandler wakes the project of a controller-managed object of one of the given names.
func standardFieldHandler(names ...string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
		return standardFieldRequests(object, names...)
	})
}

// standardFieldRequests is the request for the project an object of one of the given names belongs to
// by its project label, and nothing for any other object.
func standardFieldRequests(object client.Object, names ...string) []reconcile.Request {
	if !slices.Contains(names, object.GetName()) {
		return nil
	}
	project, ok := object.GetLabels()[v1alpha3.ResourceLabelProject]
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: project}}}
}

// namespaceProjectHandler wakes the owning real project and the virtual project
// that inventories unowned namespaces. Update maps both the old and the new
// object: after adopt the new ns is owned, so only the old side still names
// virtual default/deckhouse.
type namespaceProjectHandler struct{}

var _ handler.EventHandler = namespaceProjectHandler{}

func (namespaceProjectHandler) Create(_ context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	enqueueNamespaceProjects(q, e.Object)
}

func (namespaceProjectHandler) Update(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	enqueueNamespaceProjects(q, e.ObjectOld, e.ObjectNew)
}

func (namespaceProjectHandler) Delete(_ context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	enqueueNamespaceProjects(q, e.Object)
}

func (namespaceProjectHandler) Generic(_ context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	enqueueNamespaceProjects(q, e.Object)
}

func enqueueNamespaceProjects(q workqueue.TypedRateLimitingInterface[reconcile.Request], objects ...client.Object) {
	for _, req := range namespaceProjectRequests(objects...) {
		q.Add(req)
	}
}

func namespaceProjectRequests(objects ...client.Object) []reconcile.Request {
	seen := make(map[string]struct{})
	var reqs []reconcile.Request
	for _, object := range objects {
		if object == nil {
			continue
		}
		for _, req := range requestsForNamespace(object) {
			if _, ok := seen[req.Name]; ok {
				continue
			}
			seen[req.Name] = struct{}{}
			reqs = append(reqs, req)
		}
	}
	return reqs
}

func requestsForNamespace(object client.Object) []reconcile.Request {
	var reqs []reconcile.Request
	if proj, ok := object.GetLabels()[v1alpha3.ResourceLabelProject]; ok {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKey{Name: proj}})
	}
	if virtual := projectmanager.VirtualProjectName(object); virtual != "" {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKey{Name: virtual}})
	}
	// Adopt creates the same-name Project before Helm can label the namespace.
	// Without this, dropping foreign meta.helm.sh/* never wakes that Error Project.
	if name := object.GetName(); name != "" {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKey{Name: name}})
	}
	return reqs
}

type namespaceWatchPredicate struct {
	predicate.Funcs
}

func (namespaceWatchPredicate) Create(event.CreateEvent) bool { return true }

func (namespaceWatchPredicate) Delete(event.DeleteEvent) bool { return true }

func (namespaceWatchPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	if !e.ObjectNew.GetDeletionTimestamp().IsZero() {
		return true
	}
	oldProject := e.ObjectOld.GetLabels()[v1alpha3.ResourceLabelProject]
	newProject := e.ObjectNew.GetLabels()[v1alpha3.ResourceLabelProject]
	oldHeritage := e.ObjectOld.GetLabels()[v1alpha3.ResourceLabelHeritage]
	newHeritage := e.ObjectNew.GetLabels()[v1alpha3.ResourceLabelHeritage]
	if oldProject != newProject || oldHeritage != newHeritage {
		return true
	}
	return helmOwnershipChanged(e.ObjectOld, e.ObjectNew)
}

func helmOwnershipChanged(oldObj, newObj client.Object) bool {
	oldA, newA := oldObj.GetAnnotations(), newObj.GetAnnotations()
	return oldA[helm.ResourceAnnotationReleaseName] != newA[helm.ResourceAnnotationReleaseName] ||
		oldA[helm.ResourceAnnotationReleaseNamespace] != newA[helm.ResourceAnnotationReleaseNamespace]
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	init    *sync.WaitGroup
	manager *projectmanager.Manager
	client  client.Client
	logger  logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Info("reconcile the project", "project", req.Name)
	project := new(v1alpha3.Project)
	if err := r.client.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("the project not found", "project", req.Name)
			r.manager.Forget(req.Name)
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "failed to get the project", "project", req.Name)
		return reconcile.Result{}, err
	}

	// handle virtual projects
	if project.Spec.ProjectTemplateName == projectmanager.VirtualTemplate {
		r.logger.Info("handle the virtual project", "project", req.Name)
		return r.manager.HandleVirtual(ctx, project)
	}

	// handle the project deletion
	if !project.DeletionTimestamp.IsZero() {
		r.logger.Info("delete the project", "project", project.Name)
		return r.manager.Delete(ctx, project)
	}

	// ensure the project
	r.logger.Info("ensure the project", "project", project.Name)
	return r.manager.Handle(ctx, project)
}

// ownLabelsDrifted wakes a project whose projects.deckhouse.io/project or project-template label no
// longer says what the controller stamps (the project name, spec.projectTemplateName). Grant policies
// select projects by these labels, and a label-only edit changes neither the generation nor an
// annotation, so the other predicates miss it. The Project webhook refuses another value from anyone but
// the platform and lets a removal through with a warning; this puts the labels back after either. Virtual
// projects are stamped once, when they are created, and are left out.
func ownLabelsDrifted(e event.UpdateEvent) bool {
	project, ok := e.ObjectNew.(*v1alpha3.Project)
	if !ok || project.IsVirtual() {
		return false
	}
	return project.Labels[v1alpha3.ResourceLabelProject] != project.Name ||
		project.Labels[v1alpha3.ResourceLabelTemplate] != project.Spec.ProjectTemplateName
}

type customPredicate[T metav1.Object] struct {
	predicate.TypedFuncs[T]
	logger logr.Logger
}

func (p customPredicate[T]) Update(e event.TypedUpdateEvent[T]) bool {
	if isNil(e.ObjectOld) {
		p.logger.Error(nil, "update event has no old object to update", "event", e)
		return false
	}
	if isNil(e.ObjectNew) {
		p.logger.Error(nil, "update event has no new object for update", "event", e)
		return false
	}

	// skip projects that do not require sync
	if val, ok := e.ObjectNew.GetAnnotations()[v1alpha3.ProjectAnnotationRequireSync]; ok && val == "true" {
		return true
	}

	return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
}

func (p customPredicate[T]) Delete(_ event.TypedDeleteEvent[T]) bool {
	return true
}

func isNil(arg any) bool {
	if v := reflect.ValueOf(arg); !v.IsValid() || ((v.Kind() == reflect.Pointer ||
		v.Kind() == reflect.Interface ||
		v.Kind() == reflect.Slice ||
		v.Kind() == reflect.Map ||
		v.Kind() == reflect.Chan ||
		v.Kind() == reflect.Func) && v.IsNil()) {
		return true
	}
	return false
}
