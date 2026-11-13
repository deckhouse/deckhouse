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

// Package repair asks for a Repair when a replica finds broken blobs in its store.
//
// A replica records what it finds broken in its entry of RegistryStorage — the background check of its
// store, a Verify, a Repair that could not finish — and repairs nothing on its own: a repair takes its
// turn in the queue of RegistryOperation like every operation that reads or writes a store, so that one
// replica at a time does. This creates that operation, for the blobs and the replicas the entries name,
// as soon as no other operation runs.
//
// The same broken blobs are asked for again only retryAfter after the last request: a blob nothing
// holds a whole copy of — the only replica, or every replica broken alike, as a bundle pushed broken
// would leave them — is not repaired in a loop. A blob found broken since is a new request at once, and
// so is one found broken again after the findings were all gone.
package repair

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/register"
)

// ControllerName identifies this controller in logs, events and the --disable-controllers flag.
const ControllerName = "registry-repair"

// retryAfter is how long after a request the same broken blobs are asked for again. A request that
// failed failed for a reason — no whole copy anywhere, a replica away — which is rarely gone sooner.
const retryAfter = 4 * time.Hour

// Prefix names the operations created here.
const Prefix = "repair-auto-"

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryStorage{}, &Reconciler{})
}

// Reconciler creates the Repairs the replicas' findings call for.
type Reconciler struct {
	register.Base

	// Now is the clock, so a test can move past the retry without waiting for it.
	Now func() time.Time
}

var _ register.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// SetupWatches follows the operations, whose end lets a waiting request through, and the masters,
// whose going takes their replica's findings with them.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	toSingleton := handler.EnqueueRequestsFromMapFunc(
		func(context.Context, client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}}}
		},
	)
	w.Watches(&registryv1alpha1.RegistryOperation{}, toSingleton)
	w.Watches(&corev1.Node{}, toSingleton, builder.WithPredicates(layout.StorageNodesChanged()))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != registryv1alpha1.SingletonName {
		return ctrl.Result{}, nil
	}

	storage := &registryv1alpha1.RegistryStorage{}
	if err := r.Client.Get(ctx, req.NamespacedName, storage); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	storageNodes, err := layout.ListStorageNodes(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	storage.Status.Replicas = layout.CurrentReplicas(storage.Status.Replicas, storageNodes)

	request := Wanted(storage)
	if request == nil {
		return ctrl.Result{}, r.forgetRequest(ctx, storage)
	}

	operations := &registryv1alpha1.RegistryOperationList{}
	if err := r.Client.List(ctx, operations); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing the registry operations: %w", err)
	}

	// The same findings again: wait out the pause after the last request, unless that request never ran
	// — refused for another operation that slipped in first — and then take the next window of them, so
	// that blobs nothing can repair do not keep the rest out of every request.
	now := r.now()
	offset := 0
	if last, same := lastRequest(storage, request.Fingerprint); same && !refused(operations.Items, last.Operation) {
		if now.Before(last.At.Add(retryAfter)) {
			return ctrl.Result{RequeueAfter: last.At.Add(retryAfter).Sub(now) + time.Second}, nil
		}
		offset = last.Offset + registryv1alpha1.MaxOperationDigests
	} else if same {
		offset = last.Offset
	}
	request.Window(offset)

	if active := activeOperation(operations.Items); active != "" {
		// Its end is an event here, and the request is made then.
		ctrl.LoggerFrom(ctx).Info("broken blobs are waiting for another operation to finish before they are repaired",
			"active", active, "blobs", len(request.Digests))
		return ctrl.Result{}, nil
	}

	operation := &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{GenerateName: Prefix},
		Spec: registryv1alpha1.RegistryOperationSpec{
			Type:  registryv1alpha1.OperationRepair,
			Cause: registryv1alpha1.CorruptionDetectedCause,
			Repair: &registryv1alpha1.RepairSpec{
				Digests: request.Digests,
				Nodes:   request.Nodes,
			},
		},
	}
	if err := r.Client.Create(ctx, operation); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating a repair: %w", err)
	}
	ctrl.LoggerFrom(ctx).Info("created a repair of broken blobs", "operation", operation.Name,
		"nodes", request.Nodes, "blobs", len(request.Digests))

	patch := client.MergeFrom(storage.DeepCopy())
	if storage.Annotations == nil {
		storage.Annotations = map[string]string{}
	}
	storage.Annotations[registryv1alpha1.RepairRequestedAnnotation] = fmt.Sprintf("%s@%s@%d@%s",
		request.Fingerprint, now.UTC().Format(time.RFC3339), request.Offset, operation.Name)
	if err := r.Client.Patch(ctx, storage, patch); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("recording the repair on the storage: %w", err)
	}
	return ctrl.Result{RequeueAfter: retryAfter + time.Second}, nil
}

// forgetRequest drops the last request once nothing is broken: the findings it was for are gone,
// repaired or found whole, and a blob found broken after that is a new finding, asked for at once. Kept,
// it held the same blob broken again on the same replica — the same fingerprint — for retryAfter after
// a Repair that had repaired it.
func (r *Reconciler) forgetRequest(ctx context.Context, storage *registryv1alpha1.RegistryStorage) error {
	if _, requested := storage.Annotations[registryv1alpha1.RepairRequestedAnnotation]; !requested {
		return nil
	}
	patch := client.MergeFrom(storage.DeepCopy())
	delete(storage.Annotations, registryv1alpha1.RepairRequestedAnnotation)
	if err := r.Client.Patch(ctx, storage, patch); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("forgetting the last repair request: %w", err)
	}
	return nil
}

// Request is the Repair the replicas' findings call for.
type Request struct {
	// Digests are the broken blobs asked for, at most MaxOperationDigests of them from Offset in All:
	// the rest wait for the next request.
	Digests []string

	// All are every broken blob the entries name, sorted.
	All []string

	// Offset is where in All the Digests begin.
	Offset int

	// Nodes are the replicas that found them, sorted.
	Nodes []string

	// Fingerprint names this set of findings, so that the same one is not asked for again too soon.
	Fingerprint string
}

// Wanted is the Repair the storage's entries call for, nil when no replica has found anything broken.
func Wanted(storage *registryv1alpha1.RegistryStorage) *Request {
	blobs := map[string]bool{}
	var nodes, findings []string
	for _, replica := range storage.Status.Replicas {
		if len(replica.CorruptedBlobs) == 0 {
			continue
		}
		nodes = append(nodes, replica.Node)
		for _, blob := range replica.CorruptedBlobs {
			blobs[blob] = true
			findings = append(findings, replica.Node+"="+blob)
		}
	}
	if len(nodes) == 0 {
		return nil
	}

	digests := make([]string, 0, len(blobs))
	for blob := range blobs {
		digests = append(digests, blob)
	}
	sort.Strings(digests)
	sort.Strings(nodes)
	sort.Strings(findings)
	sum := sha256.Sum256([]byte(strings.Join(findings, "\n")))
	request := &Request{All: digests, Nodes: nodes, Fingerprint: hex.EncodeToString(sum[:8])}
	request.Window(0)
	return request
}

// Window asks for at most MaxOperationDigests of the blobs, from offset on, wrapping around.
func (r *Request) Window(offset int) {
	if len(r.All) == 0 {
		r.Digests, r.Offset = nil, 0
		return
	}
	r.Offset = offset % len(r.All)
	size := min(len(r.All), registryv1alpha1.MaxOperationDigests)
	r.Digests = make([]string, 0, size)
	for i := range size {
		r.Digests = append(r.Digests, r.All[(r.Offset+i)%len(r.All)])
	}
	sort.Strings(r.Digests)
}

// requested is what RepairRequestedAnnotation says about the last request.
type requested struct {
	At        time.Time
	Offset    int
	Operation string
}

// lastRequest is the last request, if it was for the findings with this fingerprint. Read in both of
// the forms the annotation has had: "<fingerprint>@<time>", and with "@<offset>@<operation>" after it.
func lastRequest(storage *registryv1alpha1.RegistryStorage, fingerprint string) (requested, bool) {
	parts := strings.Split(storage.Annotations[registryv1alpha1.RepairRequestedAnnotation], "@")
	if len(parts) < 2 || parts[0] != fingerprint {
		return requested{}, false
	}
	at, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return requested{}, false
	}
	last := requested{At: at}
	if len(parts) == 4 {
		last.Offset, _ = strconv.Atoi(parts[2])
		last.Operation = parts[3]
	}
	return last, true
}

// refused reports whether the named operation ended without running: refused for another operation
// that was active when it arrived. Such a request repaired nothing and is no reason to wait.
func refused(operations []registryv1alpha1.RegistryOperation, name string) bool {
	if name == "" {
		return false
	}
	for i := range operations {
		if operations[i].Name == name {
			return operations[i].Status.Phase == registryv1alpha1.OperationFailed &&
				operations[i].Status.Reason == registryv1alpha1.ReasonOperationAnotherActive
		}
	}
	return false
}

// activeOperation names an operation that is not finished, or returns "". One canceled before it was
// accepted never will be, and does not count.
func activeOperation(operations []registryv1alpha1.RegistryOperation) string {
	for i := range operations {
		operation := &operations[i]
		if operation.Status.Phase.Finished() {
			continue
		}
		if operation.Status.Phase == "" && operation.Canceled() {
			continue
		}
		return operation.Name
	}
	return ""
}
