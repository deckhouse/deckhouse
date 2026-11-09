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

// Package collection decides when the cache is collected, and asks for it by creating a
// RegistryOperation of type GarbageCollection.
//
// Three reasons to collect: the schedule, a new image set, and a store that refuses writes at one of
// its limits — not one whose directory refuses them, which a collection cannot help. The second
// is the moment there is something to collect — every replica holds the new set, so what only the old
// one needed is garbage — and it is a moment no schedule can name. The third is a store that no new
// set can ever be held in, so the second never comes for it: it is collected without waiting, once per
// deployed release, and again refusalRetryAfter after a collection of it failed. All three are turned
// off by `garbageCollection.enabled: false`; an operation somebody creates by hand runs either way.
//
// What this remembers — the last firing it acted on, the set it last collected for, the release a
// store that refuses writes was last collected for — is kept in annotations on RegistryStorage,
// because the operations are deleted a day after they finish and a controller that restarts must not
// take a firing, a set or a refusal it has already handled for a new one.
package collection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
const ControllerName = "registry-garbage-collection"

// missedFiringGrace is how late a firing of the schedule may still be acted on. A controller that was
// down at three in the morning and comes back at four still collects; one that comes back the next
// afternoon does not collect at an hour nobody chose, and waits for the next firing.
const missedFiringGrace = 4 * time.Hour

// maxFirings bounds the walk from the last firing acted on to now. A schedule that fires every minute
// and was not looked at for a long time is not worth walking minute by minute: it is started afresh.
const maxFirings = 10000

// refusalRetryAfter is how long after a failed collection of a store that refuses writes it is tried
// again. Not at once: what failed it — an upstream away, a store that cannot be read — is rarely gone a
// minute later, and every attempt is an operation kept for a day.
const refusalRetryAfter = 4 * time.Hour

// refusalPrefix names the operations created for a store that refuses writes.
const refusalPrefix = "gc-full-"

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryStorage{}, &Reconciler{})
}

// Reconciler creates the garbage collections the module asks for itself.
type Reconciler struct {
	register.Base

	// Reader reads the Deckhouse releases past the cache, only when a collection is about to be
	// created: nothing else here needs them, and an informer over them would be held for nothing.
	Reader client.Reader

	// Now is the clock, so a test can move to the next firing without waiting for it.
	Now func() time.Time

	// The deployed release as last read, and when: a store that refuses writes is reconciled with
	// every report of every replica, and the release does not move that often.
	releaseMu     sync.Mutex
	release       string
	releaseReadAt time.Time
}

// releaseFreshFor is how long a release read stays good for the refusal trigger.
const releaseFreshFor = time.Minute

var _ register.Reconciler = (*Reconciler)(nil)

// InjectAPIReader satisfies register.NeedsAPIReader.
func (r *Reconciler) InjectAPIReader(reader client.Reader) { r.Reader = reader }

func (r *Reconciler) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

// SetupWatches follows the operations: a new set that arrived while one was running is collected
// once it finishes.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	toSingleton := handler.EnqueueRequestsFromMapFunc(
		func(context.Context, client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName}}}
		},
	)
	w.Watches(&registryv1alpha1.RegistryOperation{}, toSingleton)
	// And the nodes: a master that leaves takes its replica with it and leaves its entry behind, which
	// must stop counting the moment it goes — see below. Only masters coming and going.
	w.Watches(&corev1.Node{}, toSingleton, builder.WithPredicates(layout.StorageNodesChanged()))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != registryv1alpha1.SingletonName {
		return ctrl.Result{}, nil
	}
	log := ctrl.LoggerFrom(ctx)

	storage := &registryv1alpha1.RegistryStorage{}
	if err := r.Client.Get(ctx, req.NamespacedName, storage); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The replicas still expected, and nothing else. An entry outlives its pod, and one left by a
	// master that was replaced holds the set it last had for good: no set would ever be held by
	// "every replica" again, and no new set collected. Never written back.
	storageNodes, err := layout.ListStorageNodes(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	storage.Status.Replicas = layout.CurrentReplicas(storage.Status.Replicas, storageNodes)

	collection := storage.Spec.GarbageCollection
	if collection == nil || !collection.Enabled {
		// Off, the way the syncer reads it: an absent block is a storage the controller has not
		// described yet, not a collection that is on.
		//
		// And the firing last acted on is forgotten, so that turning it on again counts from then: a
		// firing that passed while it was off is not one anybody asked for.
		if storage.Annotations[registryv1alpha1.ScheduledCollectionAnnotation] != "" {
			return ctrl.Result{}, r.annotate(ctx, storage, map[string]string{
				registryv1alpha1.ScheduledCollectionAnnotation: "",
			})
		}
		return ctrl.Result{}, nil
	}

	operations := &registryv1alpha1.RegistryOperationList{}
	if err := r.Client.List(ctx, operations); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing the registry operations: %w", err)
	}
	active := activeOperation(operations.Items)

	now := r.now()
	remember := map[string]string{}
	var result ctrl.Result

	// The set the replicas hold, read once: a collection created for the schedule covers it as well,
	// and must not be followed by another one for the same set.
	set, held := heldSet(storage)

	// The schedule.
	schedule, err := cron.ParseStandard(collection.Schedule)
	if err != nil {
		// Not guessed at: a collection at an hour nobody wrote is worse than none. The layout
		// reconciler resolves the schedule, so this is a defect there or a hand edit.
		log.Error(err, "the garbage collection schedule cannot be read, so no collection is scheduled",
			"schedule", collection.Schedule)
	} else {
		// Counted from the last firing of this same schedule. One that changed — in ModuleConfig, or with
		// the maintenance window it is derived from — starts afresh, and owes nothing.
		last := storage.Annotations[registryv1alpha1.ScheduledCollectionAnnotation]
		if storage.Annotations[registryv1alpha1.ScheduleAnnotation] != collection.Schedule {
			last = ""
		}
		walked := firing(schedule, last, now)
		due := walked.due
		switch {
		case walked.reset:
			// Never acted on, too long ago to walk to, or another schedule: counting starts now, and
			// nothing is owed.
			remember[registryv1alpha1.ScheduledCollectionAnnotation] = now.UTC().Format(time.RFC3339)
			remember[registryv1alpha1.ScheduleAnnotation] = collection.Schedule
		case !due.IsZero():
			remember[registryv1alpha1.ScheduledCollectionAnnotation] = due.UTC().Format(time.RFC3339)
			remember[registryv1alpha1.ScheduleAnnotation] = collection.Schedule
			switch {
			case active != "":
				log.Info("the schedule fired while another operation is not finished, so this firing is skipped",
					"due", due.Format(time.RFC3339), "active", active)
			case now.Sub(due) > missedFiringGrace:
				log.Info("the schedule fired too long ago, so this firing is skipped",
					"due", due.Format(time.RFC3339))
			default:
				name, err := r.create(ctx, "gc-schedule-",
					fmt.Sprintf("The garbage collection schedule %q fired at %s.", collection.Schedule, due.UTC().Format(time.RFC3339)))
				if err != nil {
					return ctrl.Result{}, err
				}
				active = name
				if held {
					r.rememberSet(ctx, remember, set)
				}
			}
		}
		// A schedule that parses and never fires — the 30th of February — has no next firing to wait for,
		// and a requeue a second away from the zero time would spin.
		if !walked.next.IsZero() {
			result.RequeueAfter = max(walked.next.Sub(now), time.Second)
		}
	}

	// A new set, once every replica holds it.
	recorded := storage.Annotations[registryv1alpha1.CollectedSetAnnotation]
	switch {
	case !held, recorded == set, remember[registryv1alpha1.CollectedSetAnnotation] != "":
	case recorded == "":
		// The first set this controller has seen held, on a fresh cache or after an upgrade: there
		// is nothing it replaced, so nothing to collect yet.
		r.rememberSet(ctx, remember, set)
	case active != "":
		// Waited out: the operation's end is an event here, and the set is still new then.
		log.Info("a new image set is held while another operation is not finished, so its collection waits",
			"active", active)
	default:
		release := r.deployedRelease(ctx)
		prefix, cause := "gc-images-", "The cache holds a new image set."
		if previous := storage.Annotations[registryv1alpha1.CollectedReleaseAnnotation]; release != "" && release != previous {
			prefix, cause = "gc-release-", fmt.Sprintf("Deckhouse was updated to %s.", release)
		}
		name, err := r.create(ctx, prefix, cause)
		if err != nil {
			return ctrl.Result{}, err
		}
		active = name
		remember[registryv1alpha1.CollectedSetAnnotation] = set
		remember[registryv1alpha1.CollectedReleaseAnnotation] = release
	}

	// A store that refuses writes, which no new set can ever be held in: the fill fails at its first
	// blob, and the trigger above waits for a set that only a collection could make room for. So it is
	// collected without waiting — once per deployed release, so that a store a collection could not free
	// is not collected over and over. One that failed is tried again, refusalRetryAfter after it ended:
	// the nightly schedule fails the same way on such a store, and the store would stay full for good.
	refusing := refusingReplicas(storage)
	recordedRefusal := storage.Annotations[registryv1alpha1.RefusalCollectedAnnotation]
	switch {
	case len(refusing) == 0:
		if recordedRefusal != "" {
			// Accepting writes again: a later refusal is a new one, and is collected.
			remember[registryv1alpha1.RefusalCollectedAnnotation] = ""
		}
	case active != "":
		// The operation in progress collects this store too.
	default:
		release := r.cachedRelease(ctx, now)
		key := release
		if key == "" {
			key = "unknown"
		}
		if key == recordedRefusal {
			retryAt, retry := refusalRetry(operations.Items)
			if !retry {
				break
			}
			if now.Before(retryAt) {
				result.RequeueAfter = sooner(result.RequeueAfter, retryAt.Sub(now)+time.Second)
				break
			}
		}
		cause := fmt.Sprintf("The store on %s refuses writes, so it is collected without waiting for an image set "+
			"it cannot take in.", strings.Join(refusing, ", "))
		if _, err := r.create(ctx, refusalPrefix, cause); err != nil {
			return ctrl.Result{}, err
		}
		remember[registryv1alpha1.RefusalCollectedAnnotation] = key
	}

	if len(remember) > 0 {
		if err := r.annotate(ctx, storage, remember); err != nil {
			return ctrl.Result{}, err
		}
	}
	return result, nil
}

// create creates a GarbageCollection and returns its name.
func (r *Reconciler) create(ctx context.Context, prefix, cause string) (string, error) {
	operation := &registryv1alpha1.RegistryOperation{
		ObjectMeta: metav1.ObjectMeta{GenerateName: prefix},
		Spec: registryv1alpha1.RegistryOperationSpec{
			Type:  registryv1alpha1.OperationGarbageCollection,
			Cause: cause,
		},
	}
	if err := r.Client.Create(ctx, operation); err != nil {
		return "", fmt.Errorf("creating a garbage collection: %w", err)
	}
	ctrl.LoggerFrom(ctx).Info("created a garbage collection", "operation", operation.Name, "cause", cause)
	return operation.Name, nil
}

// rememberSet records the set a collection was created for, with the release deployed at the time.
func (r *Reconciler) rememberSet(ctx context.Context, remember map[string]string, set string) {
	remember[registryv1alpha1.CollectedSetAnnotation] = set
	remember[registryv1alpha1.CollectedReleaseAnnotation] = r.deployedRelease(ctx)
}

// annotate merges the annotations into the storage. A merge patch of a map carries only the keys it
// sets, so it cannot drop what the layout reconciler or anybody else keeps there.
func (r *Reconciler) annotate(
	ctx context.Context, storage *registryv1alpha1.RegistryStorage, values map[string]string,
) error {
	patch := client.MergeFrom(storage.DeepCopy())
	if storage.Annotations == nil {
		storage.Annotations = map[string]string{}
	}
	for key, value := range values {
		storage.Annotations[key] = value
	}
	if err := r.Client.Patch(ctx, storage, patch); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("recording the garbage collection state on the storage: %w", err)
	}
	return nil
}

// refusalRetry says whether the last collection created for a store that refuses writes failed, and
// when it may be tried again. Only a failure: one that went through and left the store full freed what
// it could, and one that was canceled was stopped by somebody who meant it.
func refusalRetry(operations []registryv1alpha1.RegistryOperation) (time.Time, bool) {
	var last *registryv1alpha1.RegistryOperation
	for i := range operations {
		operation := &operations[i]
		if !strings.HasPrefix(operation.Name, refusalPrefix) {
			continue
		}
		if last == nil || last.CreationTimestamp.Before(&operation.CreationTimestamp) {
			last = operation
		}
	}
	if last == nil || last.Status.Phase != registryv1alpha1.OperationFailed || last.Status.FinishedAt == nil {
		return time.Time{}, false
	}
	return last.Status.FinishedAt.Add(refusalRetryAfter), true
}

// sooner is the earlier of two requeue delays, zero being none.
func sooner(current, candidate time.Duration) time.Duration {
	if current == 0 || candidate < current {
		return candidate
	}
	return current
}

// activeOperation names an operation that is not finished, or returns "". One that was canceled
// before it was accepted never will be, and does not count.
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

// refusingReplicas names the replicas whose store refuses writes at one of its two limits, sorted.
//
// The limits only: a store the registry cannot write to — NotWritable — gets nothing from a collection,
// which can only free room. Counted, it started a collection once per release, and again
// refusalRetryAfter after each that failed — as one in a directory the registry cannot write to may
// well do — each pausing the replica's writes for nothing until the operator fixed the directory.
func refusingReplicas(storage *registryv1alpha1.RegistryStorage) []string {
	var nodes []string
	for _, replica := range storage.Status.Replicas {
		if replica.Store == nil || replica.Store.Writable {
			continue
		}
		switch replica.Store.Reason {
		case registryv1alpha1.StoreBudgetExhausted, registryv1alpha1.StoreReserveExhausted:
			nodes = append(nodes, replica.Node)
		}
	}
	sort.Strings(nodes)
	return nodes
}

// heldSet returns the image set every replica holds, if they all hold the same one.
//
// Every replica rather than the leader: a follower that has not replicated the new set yet would only
// wait for it at the start of the collection, and an hour may not be enough for it.
func heldSet(storage *registryv1alpha1.RegistryStorage) (string, bool) {
	replicas := storage.Status.Replicas
	if len(replicas) == 0 {
		return "", false
	}
	set := replicas[0].VerifiedSet
	for _, replica := range replicas {
		if !replica.Full || replica.VerifiedSet == "" || replica.VerifiedSet != set {
			return "", false
		}
	}
	return set, true
}

// walk is where the schedule stands now.
type walk struct {
	// due is the latest firing since the last one acted on, zero if none came.
	due time.Time

	// next is the firing after now.
	next time.Time

	// reset means there is nothing to walk from — never acted on, unreadable, or too far back — and
	// counting starts now.
	reset bool
}

// firing walks the schedule from the last firing acted on to now.
func firing(schedule cron.Schedule, last string, now time.Time) walk {
	from, err := time.Parse(time.RFC3339, last)
	if err != nil || from.After(now) {
		return walk{next: schedule.Next(now), reset: true}
	}

	var due time.Time
	for range maxFirings {
		upcoming := schedule.Next(from)
		if upcoming.IsZero() || upcoming.After(now) {
			return walk{due: due, next: upcoming}
		}
		due, from = upcoming, upcoming
	}
	return walk{next: schedule.Next(now), reset: true}
}

// cachedRelease is deployedRelease read at most once in releaseFreshFor.
func (r *Reconciler) cachedRelease(ctx context.Context, now time.Time) string {
	r.releaseMu.Lock()
	defer r.releaseMu.Unlock()
	if !r.releaseReadAt.IsZero() && now.Sub(r.releaseReadAt) < releaseFreshFor {
		return r.release
	}
	r.release, r.releaseReadAt = r.deployedRelease(ctx), now
	return r.release
}

var releaseGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseReleaseList"}

// deployedRelease is the version of the deployed Deckhouse release, or "" when it cannot be read. Only
// what the cause of a collection says depends on it.
func (r *Reconciler) deployedRelease(ctx context.Context) string {
	reader := r.Reader
	if reader == nil {
		reader = r.Client
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(releaseGVK)
	if err := reader.List(ctx, list); err != nil {
		ctrl.LoggerFrom(ctx).Info("cannot read the Deckhouse releases, so the cause names no release",
			"error", err.Error())
		return ""
	}
	for i := range list.Items {
		phase, _, _ := unstructured.NestedString(list.Items[i].Object, "status", "phase")
		if phase != "Deployed" {
			continue
		}
		if version, _, _ := unstructured.NestedString(list.Items[i].Object, "spec", "version"); version != "" {
			return version
		}
	}

	// No release object says what the cluster runs — a cluster installed from a development image
	// never deploys one — so the image it runs is asked, as the syncer asks it.
	return runningVersion(ctx, reader)
}

var deploymentGVK = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

// runningVersion is the tag of the image the deckhouse container runs, or "".
func runningVersion(ctx context.Context, reader client.Reader) string {
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(deploymentGVK)
	if err := reader.Get(ctx, types.NamespacedName{Namespace: "d8-system", Name: "deckhouse"}, deployment); err != nil {
		ctrl.LoggerFrom(ctx).Info("cannot read the deckhouse deployment, so the cause names no release",
			"error", err.Error())
		return ""
	}

	containers, _, _ := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	for _, entry := range containers {
		container, ok := entry.(map[string]any)
		if !ok || container["name"] != "deckhouse" {
			continue
		}
		image, _ := container["image"].(string)
		return imageTag(image)
	}
	return ""
}

// imageTag is the tag of an image reference, "" when it names only a digest or nothing.
func imageTag(image string) string {
	image = strings.SplitN(image, "@", 2)[0]
	name := image[strings.LastIndex(image, "/")+1:]
	if colon := strings.LastIndex(name, ":"); colon >= 0 {
		return name[colon+1:]
	}
	return ""
}
