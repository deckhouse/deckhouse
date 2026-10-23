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

// Package revocation follows the Revocation objects user-authn writes when it revokes the
// credentials of a user, and reports the revoked pairs.
package revocation

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/coreos/pkg/capnslog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	// Namespace holds the Revocation objects.
	Namespace = "d8-user-authn"

	// typePair marks the revocation of everything a pair of user and provider holds below a
	// generation. The other types, session and grant, end what basic auth never has.
	typePair = "pair"

	subjectIndex = "subject"
)

// GVR is the Revocation type of the Dex storage.
var GVR = schema.GroupVersionResource{Group: "dex.coreos.com", Version: "v1", Resource: "revocations"}

// RevokeFunc is called for every pair revocation the watcher sees: everything issued to subject with
// a generation below generation is revoked.
type RevokeFunc func(subject string, generation uint64)

// Watcher follows the Revocation objects with an informer.
type Watcher struct {
	informer cache.SharedIndexInformer
	logger   *capnslog.PackageLogger
}

// pair is a pair revocation as the informer keeps it.
type pair struct {
	subject    string
	generation uint64
}

// New creates a watcher that calls revoke for every pair revocation it lists or sees created.
func New(client dynamic.Interface, revoke RevokeFunc) (*Watcher, error) {
	logger := capnslog.NewPackageLogger("basic-auth-proxy", "revocation")

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 0, Namespace, nil)
	informer := factory.ForResource(GVR).Informer()

	if err := informer.SetTransform(trim); err != nil {
		return nil, fmt.Errorf("set revocation transform: %w", err)
	}
	if err := informer.AddIndexers(cache.Indexers{subjectIndex: indexBySubject}); err != nil {
		return nil, fmt.Errorf("add revocation indexer: %w", err)
	}

	onObject := func(obj any) {
		p, ok, err := pairOf(obj)
		if err != nil {
			logger.Warningf("skip a revocation: %v", err)
			return
		}
		if ok {
			revoke(p.subject, p.generation)
		}
	}
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onObject,
		UpdateFunc: func(_, obj any) { onObject(obj) },
	}); err != nil {
		return nil, fmt.Errorf("add revocation event handler: %w", err)
	}

	return &Watcher{informer: informer, logger: logger}, nil
}

// Run follows the revocations until ctx is done. Until the first list completes the watcher knows
// of no revocation, and nothing is evicted earlier than the cache TTL.
func (w *Watcher) Run(ctx context.Context) {
	go func() {
		if cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced) {
			w.logger.Printf("revocations synced: %d objects", len(w.informer.GetStore().ListKeys()))
		}
	}()
	w.informer.RunWithContext(ctx)
}

// Generation returns the highest generation revoked for subject: everything issued to it below that
// generation is revoked.
func (w *Watcher) Generation(subject string) uint64 {
	objs, err := w.informer.GetIndexer().ByIndex(subjectIndex, subject)
	if err != nil {
		return 0
	}

	var highest uint64
	for _, obj := range objs {
		if p, ok, err := pairOf(obj); err == nil && ok && p.generation > highest {
			highest = p.generation
		}
	}
	return highest
}

func indexBySubject(obj any) ([]string, error) {
	p, ok, err := pairOf(obj)
	if err != nil || !ok {
		// An object that is not a valid pair revocation is not indexed; the event handler logs it.
		return nil, nil
	}
	return []string{p.subject}, nil
}

// pairOf reads a pair revocation. It reports false for the revocations of other types.
func pairOf(obj any) (pair, bool, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return pair{}, false, fmt.Errorf("unexpected object %T", obj)
	}

	revocationType, _, err := unstructured.NestedString(u.Object, "type")
	if err != nil {
		return pair{}, false, fmt.Errorf("revocation %s: type: %w", u.GetName(), err)
	}
	if revocationType != typePair {
		return pair{}, false, nil
	}

	subject, _, err := unstructured.NestedString(u.Object, "subject")
	if err != nil {
		return pair{}, false, fmt.Errorf("revocation %s: subject: %w", u.GetName(), err)
	}
	if subject == "" {
		return pair{}, false, fmt.Errorf("revocation %s has no subject", u.GetName())
	}

	generation, err := generationOf(u.Object["generation"])
	if err != nil {
		return pair{}, false, fmt.Errorf("revocation %s: generation: %w", u.GetName(), err)
	}

	return pair{subject: subject, generation: generation}, true, nil
}

// generationOf reads a JSON number the way the dynamic client decodes it: int64 for an integer,
// float64 for anything else.
func generationOf(value any) (uint64, error) {
	switch v := value.(type) {
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("negative value %d", v)
		}
		return uint64(v), nil
	case float64:
		if v < 0 || v != math.Trunc(v) || v >= math.MaxUint64 {
			return 0, fmt.Errorf("value %v is not an unsigned integer", v)
		}
		return uint64(v), nil
	case nil:
		return 0, errors.New("missing")
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}

// trim keeps only what the watcher reads: the type, subject and generation of a revocation.
func trim(obj any) (any, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}

	trimmed := &unstructured.Unstructured{Object: map[string]any{}}
	trimmed.SetAPIVersion(u.GetAPIVersion())
	trimmed.SetKind(u.GetKind())
	trimmed.SetNamespace(u.GetNamespace())
	trimmed.SetName(u.GetName())
	trimmed.SetUID(u.GetUID())
	trimmed.SetResourceVersion(u.GetResourceVersion())
	for _, field := range []string{"type", "subject", "generation"} {
		if value, found := u.Object[field]; found {
			trimmed.Object[field] = value
		}
	}

	return trimmed, nil
}
