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

// Package adoption is what a reconciler needs to own objects the module chart used to render.
//
// Such an object keeps its name, labels and content, so applications and users notice nothing, but
// it carries the ownership metadata of the release engine. The reconciler adopts it: removes that
// metadata, so the release no longer considers the object its own, and from then on writes it
// like any object it created. The module hooks protect the objects from being pruned by the
// release that stops rendering them (see hooks/internal/handover of the module), so the
// controller, deployed by that same release, finds them in place.
//
// Labels and annotations of other writers are preserved. The keys the reconciler sets on top of
// its fixed ones (user-provided metadata) are recorded in ManagedMetadataAnnotation, so a key the
// user removes from the source object is removed from the target too.
package adoption

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Release is the Helm release of the module.
	Release = "user-authn"

	ReleaseNameAnnotation = "meta.helm.sh/release-name"
	// ManagedMetadataAnnotation lists the user-provided label and annotation keys the controller
	// has put on an object.
	ManagedMetadataAnnotation = "user-authn.deckhouse.io/managed-metadata"

	// ManagedByLabel carries ManagedByValue on every object the controller writes; its informers
	// select by it. The chart put Helm there, and nelm leaves an object without it alone.
	ManagedByLabel = "app.kubernetes.io/managed-by"
	ManagedByValue = "user-authn-controller"
)

// ManagedSelector selects the objects the controller writes. One the chart rendered is not
// selected until it is adopted, so a reconciler reads it through Writer.Reader on the create
// conflict.
func ManagedSelector() labels.Selector {
	return labels.SelectorFromSet(labels.Set{ManagedByLabel: ManagedByValue})
}

// releaseAnnotations are the ownership annotations of the release engine (Helm and nelm) and the
// keep policy the migration hook stamps.
var releaseAnnotations = []string{
	ReleaseNameAnnotation,
	"meta.helm.sh/release-namespace",
	"helm.sh/resource-policy",
	"werf.io/fail-mode",
	"werf.io/track-termination-mode",
}

// ErrForeign marks an object that has the name of one of ours but belongs to somebody else.
var ErrForeign = errors.New("refusing to take over a foreign object")

// ForeignRecheckInterval is how often a name held by a foreign object is checked again.
const ForeignRecheckInterval = 5 * time.Minute

// RequeueForeign turns a foreign-object error into a periodic recheck and returns any other error
// as is. The foreign object is outside the label-scoped cache, so its removal brings no event.
func (w Writer) RequeueForeign(err error) (reconcile.Result, error) {
	if errors.Is(err, ErrForeign) && errors.Is(err, reconcile.TerminalError(nil)) {
		w.Log.Error(err, "name taken, checking again later", "after", ForeignRecheckInterval)
		return reconcile.Result{RequeueAfter: ForeignRecheckInterval}, nil
	}
	return reconcile.Result{}, err
}

// RenderedByRelease reports whether obj is still owned by the module release.
func RenderedByRelease(obj client.Object) bool {
	return obj.GetAnnotations()[ReleaseNameAnnotation] == Release
}

// HasManagedMetadata reports whether the controller has written obj.
func HasManagedMetadata(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[ManagedMetadataAnnotation]
	return ok
}

// Metadata is the desired metadata of an object.
type Metadata struct {
	// Fixed labels always win and are not recorded: the controller finds its objects by them.
	Fixed map[string]string
	// Labels and Annotations come from the user and are recorded, so that removed ones go away.
	Labels      map[string]string
	Annotations map[string]string
	// Track enables the bookkeeping annotation. Objects without user metadata do not need it.
	Track bool
}

func (m Metadata) labels() map[string]string {
	out := maps.Clone(m.Labels)
	if out == nil {
		out = make(map[string]string, len(m.Fixed))
	}
	maps.Copy(out, m.Fixed)
	out[ManagedByLabel] = ManagedByValue
	return out
}

type managedKeys struct {
	Labels      []string `json:"labels,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
}

// annotations are the desired annotations without the release ones: those belong to the release
// engine, and an object of the controller that carried them would be taken for an object of the
// chart. Apply removes them from an existing object, so New must not set them either.
func (m Metadata) annotations() map[string]string {
	out := maps.Clone(m.Annotations)
	for _, key := range releaseAnnotations {
		delete(out, key)
	}
	return out
}

func (m Metadata) keys() managedKeys {
	var k managedKeys
	for key := range m.Labels {
		if _, fixed := m.Fixed[key]; !fixed {
			k.Labels = append(k.Labels, key)
		}
	}
	k.Annotations = slices.Collect(maps.Keys(m.annotations()))
	slices.Sort(k.Labels)
	slices.Sort(k.Annotations)
	return k
}

// recordedKeys reads the bookkeeping annotation. A missing or unreadable one means nothing is known
// to be ours: the controller then only adds, which is what adopting a chart object needs, since
// the chart rendered exactly the desired metadata.
func recordedKeys(annotations map[string]string) managedKeys {
	var k managedKeys
	if raw, ok := annotations[ManagedMetadataAnnotation]; ok {
		_ = json.Unmarshal([]byte(raw), &k)
	}
	return k
}

// New sets the desired metadata on an object about to be created.
func (m Metadata) New(obj client.Object) {
	obj.SetLabels(m.labels())
	annotations := m.annotations()
	if m.Track {
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		raw, _ := json.Marshal(m.keys())
		annotations[ManagedMetadataAnnotation] = string(raw)
	}
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
}

// Apply rewrites the metadata of an existing object: the release metadata goes, recorded keys no
// longer wanted go, the desired ones are set, foreign ones stay.
func (m Metadata) Apply(obj client.Object) {
	current := obj.GetAnnotations()
	recorded := recordedKeys(current)

	obj.SetLabels(merge(obj.GetLabels(), m.labels(), recorded.Labels))

	annotations := merge(current, m.annotations(), recorded.Annotations)
	for _, key := range releaseAnnotations {
		delete(annotations, key)
	}
	delete(annotations, ManagedMetadataAnnotation)
	if m.Track {
		raw, _ := json.Marshal(m.keys())
		annotations[ManagedMetadataAnnotation] = string(raw)
	}
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
}

func merge(have, want map[string]string, recorded []string) map[string]string {
	out := maps.Clone(have)
	if out == nil {
		out = make(map[string]string, len(want))
	}
	for _, key := range recorded {
		if _, wanted := want[key]; !wanted {
			delete(out, key)
		}
	}
	maps.Copy(out, want)
	return out
}

// Writer writes owned objects. The client reads from the cache; reader bypasses it, to look at an
// object a label-scoped cache does not hold.
type Writer struct {
	Client client.Client
	Reader client.Reader
	Log    logr.Logger
}

// Ensure makes the object match want. current is the cached object or nil. owns decides whether
// an existing object may be taken over; mutate turns a copy of the existing object into the
// desired one. A create that conflicts with an object missing from the cache falls back to the
// live object, so a lagging or label-scoped cache does not stop the reconcile.
//
// The returned object is what is in the cluster now.
func (w Writer) Ensure(ctx context.Context, current, want client.Object, owns func(client.Object) bool, mutate func(client.Object)) (client.Object, error) {
	if current == nil {
		err := w.Client.Create(ctx, want)
		if err == nil {
			w.Log.Info("created", "kind", kindOf(want), "namespace", want.GetNamespace(), "name", want.GetName())
			return want, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("create %s %s: %w", kindOf(want), client.ObjectKeyFromObject(want), err)
		}
		live, err := emptyLike(want)
		if err != nil {
			return nil, err
		}
		if err := w.Reader.Get(ctx, client.ObjectKeyFromObject(want), live); err != nil {
			return nil, fmt.Errorf("read existing %s %s: %w", kindOf(want), client.ObjectKeyFromObject(want), err)
		}
		current = live
	}

	if !owns(current) {
		return nil, reconcile.TerminalError(fmt.Errorf("%s %s: %w", kindOf(want), client.ObjectKeyFromObject(current), ErrForeign))
	}

	updated, ok := current.DeepCopyObject().(client.Object)
	if !ok {
		return nil, fmt.Errorf("copy %T", current)
	}
	mutate(updated)
	if equality.Semantic.DeepEqual(current, updated) {
		return current, nil
	}
	if err := w.Client.Update(ctx, updated); err != nil {
		return nil, fmt.Errorf("update %s %s: %w", kindOf(want), client.ObjectKeyFromObject(updated), err)
	}
	w.Log.Info("updated", "kind", kindOf(want), "namespace", updated.GetNamespace(), "name", updated.GetName())
	return updated, nil
}

// Get reads the object at key into obj from the API. The cache is not read: the cached
// OAuth2Clients carry no secret (revocationfeed.TransformClient), and an update built from one
// would drop it. It reports whether the object exists.
func (w Writer) Get(ctx context.Context, key client.ObjectKey, obj client.Object) (bool, error) {
	err := w.Reader.Get(ctx, key, obj)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get %s %s: %w", kindOf(obj), key, err)
	}
	return true, nil
}

// Delete removes obj; the UID precondition keeps an object recreated in the meantime.
func (w Writer) Delete(ctx context.Context, obj client.Object, reason string) error {
	var opts []client.DeleteOption
	if uid := obj.GetUID(); uid != "" {
		opts = append(opts, client.Preconditions{UID: &uid})
	}
	err := w.Client.Delete(ctx, obj, opts...)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s %s: %w", kindOf(obj), client.ObjectKeyFromObject(obj), err)
	}
	w.Log.Info("deleted", "kind", kindOf(obj), "namespace", obj.GetNamespace(), "name", obj.GetName(), "reason", reason)
	return nil
}

// emptyLike returns a zero object of the type of obj. The live object is read into it rather than
// into a copy of the desired one: decoding merges into maps, so labels of the desired object would
// survive on a foreign one and make it pass as ours.
func emptyLike(obj client.Object) (client.Object, error) {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		out := &unstructured.Unstructured{}
		out.SetGroupVersionKind(u.GroupVersionKind())
		return out, nil
	}
	out, ok := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(client.Object)
	if !ok {
		return nil, fmt.Errorf("new %T", obj)
	}
	return out, nil
}

// kindOf names the kind for messages; typed objects from the client carry no GVK.
func kindOf(obj client.Object) string {
	if kind := obj.GetObjectKind().GroupVersionKind().Kind; kind != "" {
		return kind
	}
	return reflect.Indirect(reflect.ValueOf(obj)).Type().Name()
}
