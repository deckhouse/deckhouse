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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

// modelKind is one of the kinds of the grant model the grant webhooks read on every request.
type modelKind int

const (
	modelNamespace modelKind = iota
	modelProject
	modelPolicy
	modelReference
	modelDefinition
)

// modelObjects are the objects whose informers the model reader registers. The types and versions
// are the ones the controllers watch, so on the leader the webhooks share the controllers' informers.
func modelObjects() map[modelKind]client.Object {
	return map[modelKind]client.Object{
		modelNamespace:  &corev1.Namespace{},
		modelProject:    &v1alpha3.Project{},
		modelPolicy:     &grantsv1alpha2.ClusterResourceGrantPolicy{},
		modelReference:  &v1alpha1.GrantableClusterResourceReference{},
		modelDefinition: &v1alpha1.GrantableClusterResourceDefinition{},
	}
}

// modelKindNames name the kinds in the metrics.
var modelKindNames = map[modelKind]string{
	modelNamespace:  "Namespace",
	modelProject:    "Project",
	modelPolicy:     "ClusterResourceGrantPolicy",
	modelReference:  "GrantableClusterResourceReference",
	modelDefinition: "GrantableClusterResourceDefinition",
}

// The reasons a read of the model goes to the API server.
const (
	directNotSynced  = "not_synced"
	directWatchError = "watch_error"
	directNotInCache = "not_in_cache"
)

var (
	modelWatchErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_multitenancy_grant_webhook_model_watch_errors_total",
		Help: "Errors of the list or watch of a kind of the grant model the grant webhooks read from the cache. " +
			"After each one the kind is read from the API server for a while.",
	}, []string{"kind"})
	modelDirectReads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_multitenancy_grant_webhook_model_direct_reads_total",
		Help: "Reads of the grant model by the grant webhooks that went to the API server instead of the cache, by reason: " +
			"not_synced (the informer has not synced yet), watch_error (its list or watch failed lately), " +
			"not_in_cache (the cache did not have the object).",
	}, []string{"kind", "reason"})
)

func init() {
	metrics.Registry.MustRegister(modelWatchErrors, modelDirectReads)
}

func kindOf(obj runtime.Object) (modelKind, bool) {
	switch obj.(type) {
	case *corev1.Namespace, *corev1.NamespaceList:
		return modelNamespace, true
	case *v1alpha3.Project, *v1alpha3.ProjectList:
		return modelProject, true
	case *grantsv1alpha2.ClusterResourceGrantPolicy, *grantsv1alpha2.ClusterResourceGrantPolicyList:
		return modelPolicy, true
	case *v1alpha1.GrantableClusterResourceReference, *v1alpha1.GrantableClusterResourceReferenceList:
		return modelReference, true
	case *v1alpha1.GrantableClusterResourceDefinition, *v1alpha1.GrantableClusterResourceDefinitionList:
		return modelDefinition, true
	}
	return 0, false
}

var _ client.Reader = &ModelReader{}

// ModelReader serves the grant model of the grant webhooks (Namespaces, Projects,
// ClusterResourceGrantPolicies, GrantableClusterResourceReferences and -Definitions) from the
// manager's informers, and everything else from the direct reader.
//
// The webhooks used to read the model straight from the API server: five requests to it per admission
// request. That was chosen because a read of the manager's cache starts the informer of a kind the
// first time and waits for its sync inside the admission request, which could exceed the webhook
// deadline and, under failurePolicy: Fail, pile up into a queue lock. Here the informers are
// registered when the controller starts, on every replica (the controllers watch the same kinds, but
// only on the leader), and a read never waits for a sync: until the informer of a kind has synced the
// kind is read from the API server, so an empty or partial cache is never taken for the truth.
//
// While the list or watch of a kind fails, the kind is read from the API server as well (see
// ModelWatch), so a cache that stopped following the API server is not taken for the truth either.
//
// The informers follow the API server with the lag of the watch, usually well under a second. A Get
// the cache answers with NotFound is asked again from the API server, so a namespace or project
// created a moment ago is found: a missing namespace allows the request and a missing project changes
// the policies that match, and neither may happen just because the watch has not caught up. A change
// of a ClusterResourceGrantPolicy, a reference or a definition, or of the labels of a namespace or a
// project, is enforced once the watch delivers it. The apiserver's own namespaceSelector of the
// webhooks has a lag of the same kind.
type ModelReader struct {
	cache  client.Reader
	direct client.Reader
	synced map[modelKind]func() bool
	watch  *ModelWatch
}

// NewModelReader registers the informers of the grant model on c without waiting for their sync and
// returns the reader. It is called before the manager starts; the cache starts the informers with
// itself, on every replica. watch is the watch error handler of c (see ModelWatch); nil reads the cache
// whatever its watches report, which only tests do.
func NewModelReader(ctx context.Context, c cache.Cache, direct client.Reader, watch *ModelWatch) (*ModelReader, error) {
	synced := map[modelKind]func() bool{}
	for kind, obj := range modelObjects() {
		inf, err := c.GetInformer(ctx, obj, cache.BlockUntilSynced(false))
		if err != nil {
			return nil, fmt.Errorf("register informer of %T: %w", obj, err)
		}
		synced[kind] = inf.HasSynced
	}
	return &ModelReader{cache: c, direct: direct, synced: synced, watch: watch}, nil
}

// modelWatchErrorHold is how long a kind is read from the API server after an error of its list or
// watch. It outlasts the longest wait of a reflector between two attempts (30 s backoff, with jitter
// up to twice that), so a kind whose watch keeps failing is never read from the cache in between.
const modelWatchErrorHold = 2 * time.Minute

// ModelWatch is the watch error handler of the manager's cache. HasSynced of an informer never goes
// back to false, so after a list or watch of a kind of the grant model starts failing (the permission
// to watch it was revoked, the API server answers 429 or 5xx on the relist) the cache would go on
// answering with what it last saw, for as long as the failure lasts. Instead the kind is read from the
// API server until modelWatchErrorHold has passed since its last error: a failure of the API server
// then fails the request (failurePolicy: Fail) rather than letting it be decided on stale data.
type ModelWatch struct {
	mu            sync.Mutex
	degradedUntil map[modelKind]time.Time
	now           func() time.Time
}

// NewModelWatch returns the handler; its WatchError goes into cache.Options.DefaultWatchErrorHandler.
func NewModelWatch() *ModelWatch {
	return &ModelWatch{degradedUntil: map[modelKind]time.Time{}, now: time.Now}
}

// modelTypeDescriptions maps the type description of a reflector (the Go type of the objects it
// lists, for a typed informer) to the kind of the model.
var modelTypeDescriptions = func() map[string]modelKind {
	out := map[string]modelKind{}
	for kind, obj := range modelObjects() {
		out[fmt.Sprintf("%T", obj)] = kind
	}
	return out
}()

// WatchError is the watch error handler of every informer of the manager's cache. It logs as the
// default handler does and, for a kind of the model, holds its reads off the cache. A watch that only
// expired or was closed by the API server is not a failure: the reflector resumes it at once.
func (w *ModelWatch) WatchError(ctx context.Context, r *toolscache.Reflector, err error) {
	toolscache.DefaultWatchErrorHandler(ctx, r, err)
	if benignWatchError(err) {
		return
	}
	kind, ok := modelTypeDescriptions[r.TypeDescription()]
	if !ok {
		return
	}
	w.markDegraded(kind)
}

func (w *ModelWatch) markDegraded(kind modelKind) {
	modelWatchErrors.WithLabelValues(modelKindNames[kind]).Inc()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.degradedUntil[kind] = w.now().Add(modelWatchErrorHold)
}

func (w *ModelWatch) degraded(kind modelKind) bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	until, ok := w.degradedUntil[kind]
	return ok && w.now().Before(until)
}

// benignWatchError reports the errors the reflector recovers from by itself: an expired resource
// version (it lists again), a watch closed by the API server, and the shutdown of the informer.
func benignWatchError(err error) bool {
	return k8serrors.IsResourceExpired(err) || k8serrors.IsGone(err) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.Canceled)
}

// source tells where a read of obj goes: the cache (true), or the API server for the reason given. It
// returns the kind of the model obj is of too.
// The informer is checked before every read of the cache: the manager's cache waits for an unsynced
// informer, which a webhook must never do.
func (r *ModelReader) source(obj runtime.Object) (modelKind, bool, string) {
	kind, ok := kindOf(obj)
	if !ok {
		return kind, false, ""
	}
	if synced := r.synced[kind]; synced == nil || !synced() {
		return kind, false, directNotSynced
	}
	if r.watch.degraded(kind) {
		return kind, false, directWatchError
	}
	return kind, true, ""
}

func countDirect(kind modelKind, reason string) {
	if reason != "" {
		modelDirectReads.WithLabelValues(modelKindNames[kind], reason).Inc()
	}
}

// Get reads the object from the cache once its informer has synced and its watch is healthy, and from
// the API server otherwise or when the cache does not have it.
func (r *ModelReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	kind, cached, reason := r.source(obj)
	if !cached {
		countDirect(kind, reason)
		return r.direct.Get(ctx, key, obj, opts...)
	}
	err := r.cache.Get(ctx, key, obj, opts...)
	if k8serrors.IsNotFound(err) {
		countDirect(kind, directNotInCache)
		return r.direct.Get(ctx, key, obj, opts...)
	}
	return err
}

// List lists from the cache once the informer of the kind has synced and its watch is healthy, and
// from the API server otherwise.
//
// A list from the cache is not deep-copied: the items share their fields with the cache, so a caller
// must not modify them. The webhooks and resolve only read the grant model; copying every reference,
// definition and policy on every admission request is what this spares.
func (r *ModelReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	kind, cached, reason := r.source(list)
	if !cached {
		countDirect(kind, reason)
		return r.direct.List(ctx, list, opts...)
	}
	return r.cache.List(ctx, list, append(opts, client.UnsafeDisableDeepCopy)...)
}
