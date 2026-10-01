// Copyright 2026 Flant JSC
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

// Package orphans deletes what an application instance leaves beside its Helm release: objects its
// workloads or hooks create at runtime, and the release hooks an uninstall does not delete. The
// package declares the kinds of such objects; the instance's objects among them are the ones in its
// namespace that carry its instance label.
package orphans

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/werf/nelm/pkg/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// serviceTracer identifies tracing spans and log records emitted by the service.
	serviceTracer = "orphans-service"
)

// Resource is a kind of object an application leaves beside its release, as its package declares
// it under orphanResources in its values:
//
//	orphanResources:
//	  - kind: Job
//	    version: v1
type Resource struct {
	// Kind is the kind of the objects, e.g. Job.
	Kind string `json:"kind"`

	// Version is the API version the kind is served at. A bare version ("v1") matches the kind in
	// every group serving it at that version, so Job is declared by "v1" just as Pod is; a
	// group-qualified one ("batch/v1") matches that group only.
	Version string `json:"version"`
}

// kubeClient is the part of the Kubernetes client the service works through.
type kubeClient interface {
	// ToDiscoveryClient returns the cached discovery of the resources the cluster serves.
	ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
	// Metadata lists and deletes objects by their metadata, so a Secret is never read whole.
	Metadata() metadata.Interface
}

// apiResource is an API resource serving a declared kind.
type apiResource struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

// Service deletes what application instances leave beside their releases.
type Service struct {
	client kubeClient

	logger *log.Logger
}

// NewService creates a service that lists and deletes objects through the given client.
func NewService(client kubeClient, logger *log.Logger) *Service {
	return &Service{
		client: client,
		logger: logger.Named(serviceTracer),
	}
}

// Delete deletes the objects of the declared kinds the instance left in its namespace: those that
// carry its instance label. It is meant to run once the instance's release is uninstalled, so that
// nothing of the instance is left to recreate them.
//
// Only what a retry can fix fails the call, since a removal waits on it. A kind the cluster does not
// serve has no objects to delete. A cluster-scoped kind is skipped: the instance label names an
// instance within its namespace alone, so across the cluster it matches the instances of the same
// name in other namespaces too. An object that says it outlives its release is left, as the
// uninstall left it.
func (s *Service) Delete(ctx context.Context, namespace, instance string, resources []Resource) error {
	ctx, span := otel.Tracer(serviceTracer).Start(ctx, "Delete")
	defer span.End()

	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("instance", instance))

	selector, err := labels.ValidatedSelectorFromSet(labels.Set{v1alpha1.PackageLabelInstance: instance})
	if err != nil {
		// no object can carry a label value the API server rejects, and a list by it fails for good
		s.logger.Warn("instance name is not a label value, nothing to delete",
			slog.String("namespace", namespace),
			slog.String("instance", instance),
			log.Err(err))

		return nil
	}

	apiResources, err := s.resolve(resources)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("resolve resources: %w", err)
	}

	var errs error
	for _, res := range apiResources {
		if !res.namespaced {
			s.logger.Warn("skip cluster-scoped orphan resource", slog.String("resource", res.gvr.String()))
			continue
		}

		if err = s.deleteObjects(ctx, res.gvr, namespace, selector.String()); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		span.SetStatus(codes.Error, errs.Error())
	}

	return errs
}

// resolve maps the declared kinds onto the API resources serving them. A kind missing from the
// cached discovery is looked up once more afresh, since a CRD installed after the cache was filled
// may serve it; a kind still missing is not served, so it has no objects to delete.
func (s *Service) resolve(resources []Resource) ([]apiResource, error) {
	disc, err := s.client.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("get discovery client: %w", err)
	}

	resolved, missing, err := s.match(disc, resources)
	if err == nil && len(missing) > 0 {
		disc.Invalidate()
		resolved, missing, err = s.match(disc, resources)
	}

	if err != nil {
		return nil, err
	}

	for _, res := range missing {
		s.logger.Warn("orphan resource kind is not served, skip it",
			slog.String("kind", res.Kind),
			slog.String("version", res.Version))
	}

	return resolved, nil
}

// match matches the declared kinds against the resources the cluster serves, returning the API
// resources serving them and the kinds none serves.
func (s *Service) match(disc discovery.DiscoveryInterface, resources []Resource) ([]apiResource, []Resource, error) {
	_, lists, err := disc.ServerGroupsAndResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return nil, nil, fmt.Errorf("discover served resources: %w", err)
		}

		// a group whose discovery failed hides its own kinds only, the rest are matched still
		s.logger.Warn("discover served resources partially", log.Err(err))
	}

	resolved := make([]apiResource, 0, len(resources))
	missing := make([]Resource, 0, len(resources))

	for _, res := range resources {
		found := serving(lists, res)
		if len(found) == 0 {
			missing = append(missing, res)
			continue
		}

		resolved = append(resolved, found...)
	}

	return resolved, missing, nil
}

// serving returns the API resources in lists that serve the declared kind at its version, in the
// group the version names or, for a bare version, in any group. A subresource carries the kind of
// its parent and does not count, nor does a resource that cannot be listed and deleted.
func serving(lists []*metav1.APIResourceList, res Resource) []apiResource {
	want, err := schema.ParseGroupVersion(res.Version)
	if err != nil || want.Version == "" || res.Kind == "" {
		return nil
	}

	var found []apiResource

	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil || gv.Version != want.Version || (want.Group != "" && gv.Group != want.Group) {
			continue
		}

		for _, served := range list.APIResources {
			if served.Kind != res.Kind || strings.Contains(served.Name, "/") {
				continue
			}

			if !slices.Contains(served.Verbs, "list") || !slices.Contains(served.Verbs, "delete") {
				continue
			}

			found = append(found, apiResource{
				gvr:        gv.WithResource(served.Name),
				namespaced: served.Namespaced,
			})
		}
	}

	return found
}

// deleteObjects deletes the objects of the resource in the namespace that the selector matches. An
// object failing to delete does not stop the others; every failure is returned.
func (s *Service) deleteObjects(ctx context.Context, gvr schema.GroupVersionResource, namespace, selector string) error {
	client := s.client.Metadata().Resource(gvr).Namespace(namespace)

	list, err := client.List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		// the resource went away since discovery was read, and its objects with it
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("list %s: %w", gvr, err)
	}

	var errs error
	for i := range list.Items {
		obj := &list.Items[i]

		// already being deleted: a finalizer holds it now, which another delete does not release
		if obj.GetDeletionTimestamp() != nil {
			continue
		}

		if outlivesRelease(obj.GetAnnotations()) {
			s.logger.Debug("keep orphan resource, it outlives its release",
				slog.String("resource", gvr.String()),
				slog.String("namespace", namespace),
				slog.String("name", obj.GetName()))

			continue
		}

		err = client.Delete(ctx, obj.GetName(), metav1.DeleteOptions{
			// batch/v1 still orphans the Pods of a Job deleted without a policy
			PropagationPolicy: ptr.To(metav1.DeletePropagationBackground),
			// pinned to the object listed, not to one recreated under its name since
			Preconditions: metav1.NewUIDPreconditions(string(obj.GetUID())),
		})

		switch {
		case err == nil:
			s.logger.Info("delete orphan resource",
				slog.String("resource", gvr.String()),
				slog.String("namespace", namespace),
				slog.String("name", obj.GetName()))

		case apierrors.IsNotFound(err), apierrors.IsConflict(err):
			// gone already, or replaced by an object that was not listed

		default:
			errs = errors.Join(errs, fmt.Errorf("delete %s '%s': %w", gvr, obj.GetName(), err))
		}
	}

	return errs
}

// outlivesRelease reports whether the annotations say the object outlives the release that rendered
// it, which is why the uninstall left it: a keep or skip-delete resource policy, or ownership shared
// with anyone. Nelm counts release hooks as shared too, but by its own default rather than by an
// annotation, and that default is what leaves hooks behind — so a hook is not kept here.
func outlivesRelease(annotations map[string]string) bool {
	if common.Ownership(strings.TrimSpace(annotations[common.AnnotationKeyHumanOwnership])) == common.OwnershipAnyone {
		return true
	}

	// the werf policy takes precedence over the Helm one, as nelm reads them
	policies, ok := annotations[common.AnnotationKeyHumanWerfResourcePolicy]
	if !ok {
		policies = annotations[common.AnnotationKeyHumanResourcePolicy]
	}

	for policy := range strings.SplitSeq(policies, ",") {
		if p := common.ResourcePolicy(strings.TrimSpace(policy)); p == common.ResourcePolicyKeep || p == common.ResourcePolicySkipDelete {
			return true
		}
	}

	return false
}
