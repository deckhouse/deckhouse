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

// Leaving the pull path without taking the address away from what still names it.
//
// `Managed` -> `Unmanaged` used to be immediate: the moment the mode changed, the storage and the node
// agent went, and the in-cluster address stopped answering. But the address is named by rendered
// manifests all over the cluster, and those move to the upstream registry only as the operator
// re-renders each module's release — one release at a time. Measured on a two-node cloud cluster:
//
//	84s   the platform's own Deployment still named `registry.d8-system.svc:5001` after the storage
//	      and its Service were gone. Nothing failed only because the operator pod happened not to
//	      restart in that window; the one component able to rewrite that reference is the one that
//	      could no longer pull. On an earlier stand it did restart, and the cluster needed the image
//	      in its Deployment patched by hand to come back at all.
//	680s  workloads across d8-monitoring, d8-ingress-nginx, d8-admission-policy-engine,
//	      d8-snapshot-controller and kube-system could not pull, 16 of them at the peak, each still
//	      naming an address that answered nothing.
//
// So the two halves are separated here. Withdrawing the ADDRESS is immediate — the ConfigMap goes at
// once, so every render from that moment on names the upstream registry and the platform starts moving.
// Withdrawing the SERVICE waits until nothing names the in-cluster address any more. In between, the
// module keeps serving from the configuration that was in effect when the user asked to leave, which is
// what this hook persists.
//
// The consequence, and it is deliberate: `Unmanaged` is reached when the drain finishes, not when the
// ModuleConfig is written. A cluster in this state runs the module's components while its ModuleConfig
// says `Unmanaged`, which would be confusing without something saying why — hence the metric and the
// alert. The alternative is a documented outage of minutes on a routine configuration change, and on a
// production cluster that is the worse of the two.
//
// What is NOT covered: disabling the module outright (`enabled: false`) instead of asking for
// `Unmanaged`. Then there is no release to render anything from and helm removes it all in one pass, so
// the window is back. That path is for a cluster being taken off this module for good, and the way
// through it is to reach `Unmanaged` first and let the drain finish.
package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	// DrainSecretName persists a drain in progress.
	//
	// In a secret rather than in values, because values do not survive a restart of the module and this
	// state has to: a drain lasts minutes, an operator restart takes seconds, and forgetting mid-drain
	// would tear the registry down under the workloads still pulling from it — the exact failure this
	// hook exists to prevent. It holds the configuration that was in effect, which includes upstream
	// credentials, so a secret is also the right kind of object for it.
	//
	// Written and removed by this hook, never by helm: helm renders from values, and the whole point of
	// this record is to outlive them.
	DrainSecretName = "registry-drain"

	// DrainReferencesMetric is how many workloads still name the in-cluster registry.
	DrainReferencesMetric = "d8_registry_drain_references"

	// DrainSecondsMetric is how long the drain has been going on.
	DrainSecondsMetric = "d8_registry_drain_seconds"

	drainSnapName          = "drain-record"
	registryConfigSnapName = "registry-config-resource"
	drainMetricGroup       = "registry_drain"

	// referenceExamples is how many names are logged and no more. The count is the decision; the names
	// are there to make an alert actionable, and a list of sixteen in a log line is neither.
	referenceExamples = 5
)

// drainRecord is what survives a restart.
type drainRecord struct {
	// Config is the configuration the module keeps serving from. Captured from the RegistryConfig
	// resource, which is the resolved configuration the controller was already acting on — rather than
	// from the ModuleConfig, which by this point says `Unmanaged` and no longer describes a registry.
	Config RegistryConfig `json:"config"`

	// StartedAt is when the user asked to leave, and what the alert measures from.
	StartedAt time.Time `json:"startedAt"`

	// ZeroScans counts consecutive scans that found nothing. Two are required rather than one: a
	// Deployment re-rendered onto the upstream registry has its old pods for a moment longer, and a
	// scan that lands in that moment sees a clean cluster that is not yet clean.
	ZeroScans int `json:"zeroScans"`

	// Finished records that the withdrawal has been decided and only the render that removes the
	// components is outstanding.
	//
	// Without it the whole thing loops. The configuration a drain serves from is captured from the
	// RegistryConfig resource, and that resource is rendered from the values this hook writes — so it
	// outlives the decision by exactly one render. A hook pass landing in that gap sees `Unmanaged` plus
	// a resource that says `Managed`, reads it as "still serving", and starts the drain over. Measured on
	// the stand: zero references for seven minutes, the storage still up, and the record reappearing with
	// a new `startedAt` every time.
	Finished bool `json:"finished,omitempty"`

	// LastZeroAt is when the last empty scan was counted, and it is what makes "two scans" mean time
	// rather than repetition. This hook runs on a schedule AND before every render of the module, and a
	// drain changes the values on every pass — so two passes can land seconds apart, which is exactly
	// the width of the gap between one module release being rendered and the next. Two zeroes inside
	// that gap would prove nothing at all.
	LastZeroAt time.Time `json:"lastZeroAt,omitempty"`
}

// quietGap is how far apart two empty scans have to be before they count as two.
//
// Half a minute: the gap being guarded against is one render of one module release, and on the stand the
// whole platform moved in about eleven minutes across some forty of them.
const quietGap = 30 * time.Second

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// Before the hook that resolves the configuration, which reads the decision made here.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 9},
		Queue:        "/modules/registry/v2",
		Schedule: []go_hook.ScheduleConfig{
			{
				// Every minute, and the scan itself runs only while a drain is in progress — the check
				// before it is one read of this module's own values. A drain lasts minutes, so a minute
				// of granularity costs at most one extra minute of the module serving an address nobody
				// needs, which is the harmless direction.
				Name:    "registry-drain",
				Crontab: "* * * * *",
			},
		},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       drainSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{DrainSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterDrainSecret,
			},
			{
				// The resolved configuration as the controller has it, which is what a drain serves
				// from. Watched rather than reconstructed: the ModuleConfig it came from has already
				// been changed by the time this hook needs it.
				Name:       registryConfigSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "RegistryConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry"},
				},
				FilterFunc: filterRegistryConfigSpec,
			},
		},
	},
	dependency.WithExternalDependencies(handleDrain),
)

func filterDrainSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the drain secret: %w", err)
	}

	return secret.Data[drainRecordKey], nil
}

// drainRecordKey is the only key in the secret.
const drainRecordKey = "record"

func filterRegistryConfigSpec(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var resource registryv1alpha1.RegistryConfig
	if err := sdk.FromUnstructured(obj, &resource); err != nil {
		return nil, fmt.Errorf("converting the registry configuration: %w", err)
	}

	// Carried as JSON rather than as the resource's own Go type, because what a drain has to serve is
	// this module's resolved configuration and the two are the same document — the resource is rendered
	// from it field by field. Marshalling here keeps the conversion in one place.
	raw, err := json.Marshal(resource.Spec)
	if err != nil {
		return nil, fmt.Errorf("encoding the registry configuration: %w", err)
	}

	return raw, nil
}

func handleDrain(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	values := accessor(input)
	current := values.Get()

	if !current.Enabled {
		// The previous implementation owns the cluster: nothing of ours is on the pull path, so there is
		// nothing that could be taken away from anything.
		return finishDrain(input, values, current, "the current implementation is not active")
	}

	parsed, err := readSettings(input)
	if err != nil {
		// The settings are unreadable. Left exactly as it is rather than guessed at: every outcome here
		// is a change to what the cluster pulls through.
		return fmt.Errorf("reading the settings: %w", err)
	}

	mode := parsed.Mode
	if mode == "" {
		mode = string(registryv1alpha1.ModeManaged)
	}

	record, err := readDrainRecord(input)
	if err != nil {
		return err
	}

	if mode == string(registryv1alpha1.ModeManaged) {
		// Managing again — either it never stopped, or a drain was interrupted by the user changing
		// their mind. Nothing has to be torn down in either case.
		return finishDrain(input, values, current, "the module manages the pull path")
	}

	if record == nil {
		// The snapshot says there is no drain. Confirmed against the API before believing it, because
		// the snapshot is an informer and an informer is empty for a moment after the module starts —
		// and a module restarts mid-drain routinely, since the platform moving its own image reference
		// off the in-cluster registry is what restarts it.
		//
		// Measured on the stand without this: the record was rewritten eleven seconds into a drain and
		// its `startedAt` jumped from 13:54:42 to 13:55:33. Nothing broke, because a fresh record still
		// keeps the registry serving — but the clock the alert measures and the count of quiet scans
		// both went back to zero, so a drain that never finishes could restart its way out of being
		// reported.
		record, err = existingDrainRecord(ctx, dc)
		if err != nil {
			return err
		}
	}

	captured, err := capturedConfig(input)
	if err != nil {
		return err
	}
	if captured != nil && captured.Mode != string(registryv1alpha1.ModeManaged) {
		// A resource that says `Unmanaged` describes a module that was not serving anything, so there is
		// nothing to drain. Flattened to "no configuration" here rather than checked again below: one
		// answer to "was this module on the pull path" is easier to keep right than two.
		captured = nil
	}

	// Scanned only when the answer can change anything: a decided withdrawal is waiting for a render,
	// not for the cluster, and there is nothing to count for a drain that has not started.
	count, examples := -1, []string(nil)
	if record != nil && !record.Finished {
		var scanErr error
		count, examples, scanErr = scanReferences(ctx, dc)
		if scanErr != nil {
			// Unknown is not zero. A scan that could not be made says nothing about what still needs the
			// address, and the only harmful action available here is tearing the registry down, so the
			// drain continues on the last thing that was known.
			input.Logger.Warn("cannot tell what still needs the in-cluster registry, so the drain continues",
				"error", scanErr.Error())
			count, examples = -1, nil
		}
	}

	step, next, reason := nextDrainStep(record, captured, count, time.Now().UTC())

	switch step {
	case drainForget:
		return finishDrain(input, values, current, reason)

	case drainWithdrawn:
		// Nothing to do but stay out of the way of the render that finishes this.
		if current.Drain != nil {
			current.Drain = nil
			values.Set(current)
		}
		input.MetricsCollector.Expire(drainMetricGroup)

		return nil

	case drainWithdraw:
		input.Logger.Info("the module leaves the pull path", "reason", reason,
			"draining", drainedFor(record))
		if err := writeDrainRecord(input, next); err != nil {
			return err
		}
		if current.Drain != nil {
			current.Drain = nil
			values.Set(current)
		}
		input.MetricsCollector.Expire(drainMetricGroup)

		return nil
	}

	if record == nil {
		input.Logger.Info(
			"asked to leave the pull path; the in-cluster registry keeps serving until nothing needs it",
			"address", registry_const.Host, "references", count)
	} else if count != 0 {
		input.Logger.Info("still serving the in-cluster registry while the platform moves off it",
			"references", count,
			"examples", strings.Join(examples, ", "),
			"draining", drainedFor(record))
	}

	if err := writeDrainRecord(input, next); err != nil {
		return err
	}

	served := next.Config
	// Serving is what `Managed` means, and the drain is a cluster that has been asked to stop and has
	// not finished stopping. Every template gates on this one value, so saying it here is what keeps the
	// storage, the controller and the node agent in place without a second gate in each of them.
	served.Mode = string(registryv1alpha1.ModeManaged)

	current.Drain = &DrainState{
		Active:     true,
		References: count,
		StartedAt:  next.StartedAt.Format(time.RFC3339),
		Config:     &served,
	}
	values.Set(current)

	publishDrainMetrics(input, count, time.Since(next.StartedAt))

	return nil
}

// drainStep is what has to happen to a drain, and the four of them are the whole state machine.
type drainStep int

const (
	// drainServe keeps the in-cluster registry up and serving from the captured configuration.
	drainServe drainStep = iota

	// drainWithdraw stops serving and remembers that it happened, so the render that removes the
	// components cannot be mistaken for the module still managing.
	drainWithdraw

	// drainWithdrawn is the wait for that render. Nothing is served and nothing is decided again.
	drainWithdrawn

	// drainForget removes the record: either it was never needed, or its work is done.
	drainForget
)

// nextDrainStep is the whole decision, kept away from the reads and writes around it.
//
// `count` is how many workloads still need the in-cluster registry, or -1 when that could not be
// established — and -1 is not zero: an unknown must never end a drain, because the only harmful action
// available here is taking a registry away from something that is still pulling from it.
func nextDrainStep(
	record *drainRecord,
	captured *RegistryConfig,
	count int,
	now time.Time,
) (drainStep, *drainRecord, string) {
	if record == nil {
		if captured == nil {
			return drainForget, nil, "the module was not serving the pull path"
		}
		// Started even when the scan found nothing, and finished on the next pass if it stays that way.
		// Starting costs one secret and one extra minute of serving; not starting because a single scan
		// came back empty is how a cluster mid-render loses the address under it.
		return drainServe, &drainRecord{Config: *captured, StartedAt: now}, ""
	}

	if record.Finished {
		if captured == nil {
			// The release has removed the resource, so there is nothing left to be confused by.
			return drainForget, nil, "the withdrawal is complete"
		}
		return drainWithdrawn, record, "waiting for the release to remove the components"
	}

	next := *record
	switch {
	case count > 0:
		// Progress, or a workload that has just appeared. Either way the count of consecutive empty
		// scans is no longer consecutive.
		next.ZeroScans = 0
		next.LastZeroAt = time.Time{}

	case count == 0 && (next.LastZeroAt.IsZero() || now.Sub(next.LastZeroAt) >= quietGap):
		next.ZeroScans++
		next.LastZeroAt = now

		// A count of -1 falls through both: an unknown is neither, and the counter stands still.
	}

	if next.ZeroScans >= zeroScansToFinish {
		next.Finished = true
		return drainWithdraw, &next, "nothing needs the in-cluster registry any more"
	}

	return drainServe, &next, ""
}

// drainedFor is how long a drain has been going on, for a log line. Empty for one that starts now.
func drainedFor(record *drainRecord) string {
	if record == nil {
		return "0s"
	}

	return time.Since(record.StartedAt).Round(time.Second).String()
}

// zeroScansToFinish is how many consecutive empty scans end a drain. See drainRecord.ZeroScans.
const zeroScansToFinish = 2

// finishDrain removes the record and the state, and says in the log why.
//
// Idempotent on purpose: it is the answer to every path that is not "still draining", and those paths
// are the common case — a cluster that never left `Managed` reaches this on every reconciliation.
func finishDrain(
	input *go_hook.HookInput,
	values helpers.ValuesAccessor[Values],
	current Values,
	because string,
) error {
	if current.Drain != nil {
		current.Drain = nil
		values.Set(current)
	}

	input.MetricsCollector.Expire(drainMetricGroup)

	record, err := readDrainRecord(input)
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}

	input.Logger.Info("removing the drain record", "reason", because)
	input.PatchCollector.Delete("v1", "Secret", "d8-system", DrainSecretName)

	return nil
}

func readDrainRecord(input *go_hook.HookInput) (*drainRecord, error) {
	raw, err := helpers.SnapshotToSingle[[]byte](input, drainSnapName)
	if err != nil {
		return nil, nil //nolint:nilerr // Absent is the ordinary case, not a failure.
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var record drainRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("decoding the drain record: %w", err)
	}

	return &record, nil
}

// existingDrainRecord reads the record straight from the API, for the moment when the informer behind
// the snapshot has not caught up. Absent is not an error: on almost every cluster there is no drain.
func existingDrainRecord(ctx context.Context, dc dependency.Container) (*drainRecord, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("getting the kubernetes client: %w", err)
	}

	secret, err := client.CoreV1().Secrets("d8-system").Get(ctx, DrainSecretName, v1meta.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the drain record: %w", err)
	}

	raw := secret.Data[drainRecordKey]
	if len(raw) == 0 {
		return nil, nil
	}

	var record drainRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("decoding the drain record: %w", err)
	}

	return &record, nil
}

func writeDrainRecord(input *go_hook.HookInput, record *drainRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding the drain record: %w", err)
	}

	secret := &v1core.Secret{
		TypeMeta: v1meta.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: v1meta.ObjectMeta{
			Name:      DrainSecretName,
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "registry",
			},
		},
		Data: map[string][]byte{drainRecordKey: raw},
	}

	object, err := sdk.ToUnstructured(secret)
	if err != nil {
		return fmt.Errorf("converting the drain secret: %w", err)
	}

	input.PatchCollector.CreateOrUpdate(object)
	return nil
}

func capturedConfig(input *go_hook.HookInput) (*RegistryConfig, error) {
	raw, err := helpers.SnapshotToSingle[[]byte](input, registryConfigSnapName)
	if err != nil {
		return nil, nil //nolint:nilerr // No resource means nothing was being served.
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var config RegistryConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("decoding the registry configuration: %w", err)
	}

	return &config, nil
}

func publishDrainMetrics(input *go_hook.HookInput, references int, elapsed time.Duration) {
	input.MetricsCollector.Expire(drainMetricGroup)

	// A negative count means the scan failed, and a gauge cannot say "unknown". Reported as the
	// duration alone: the alert is about a drain that does not end, and a drain whose scans keep failing
	// is exactly that.
	if references >= 0 {
		input.MetricsCollector.Set(DrainReferencesMetric, float64(references), nil,
			metrics.WithGroup(drainMetricGroup))
	}

	input.MetricsCollector.Set(DrainSecondsMetric, elapsed.Seconds(), nil,
		metrics.WithGroup(drainMetricGroup))
}

// scanReferences counts the workloads that still name the in-cluster registry.
//
// Listed on demand rather than watched. A watch on every pod in the cluster would be paid for
// permanently, by every cluster, to answer a question that only matters during the minutes after a
// mode change — and this hook checks its own values before scanning, so a cluster that is not draining
// pays nothing at all.
//
// Both running pods and the templates of what creates them are counted, and the templates are the more
// important half: a Deployment that has not been re-rendered will create a pod naming the old address
// the moment the current one restarts, long after the pod list looks clean.
//
// Only the platform's own namespaces. Those references come from Deckhouse renders, so those are the
// namespaces they are in; a workload of the user's that hardcodes the in-cluster address would keep the
// module serving forever, which is what the alert is for rather than something to wait on silently.
func scanReferences(ctx context.Context, dc dependency.Container) (int, []string, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return 0, nil, fmt.Errorf("getting the kubernetes client: %w", err)
	}

	var found []string
	note := func(kind, namespace, name string) {
		found = append(found, fmt.Sprintf("%s %s/%s", kind, namespace, name))
	}

	pods, err := client.CoreV1().Pods("").List(ctx, v1meta.ListOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("listing pods: %w", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if platformNamespace(pod.Namespace) && podCouldStillPull(pod) {
			note("pod", pod.Namespace, pod.Name)
		}
	}

	deployments, err := client.AppsV1().Deployments("").List(ctx, v1meta.ListOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("listing deployments: %w", err)
	}
	for i := range deployments.Items {
		item := &deployments.Items[i]
		if platformNamespace(item.Namespace) && namesInClusterRegistry(&item.Spec.Template.Spec) {
			note("deployment", item.Namespace, item.Name)
		}
	}

	daemonSets, err := client.AppsV1().DaemonSets("").List(ctx, v1meta.ListOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("listing daemonsets: %w", err)
	}
	for i := range daemonSets.Items {
		item := &daemonSets.Items[i]
		if platformNamespace(item.Namespace) && namesInClusterRegistry(&item.Spec.Template.Spec) {
			note("daemonset", item.Namespace, item.Name)
		}
	}

	statefulSets, err := client.AppsV1().StatefulSets("").List(ctx, v1meta.ListOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("listing statefulsets: %w", err)
	}
	for i := range statefulSets.Items {
		item := &statefulSets.Items[i]
		if platformNamespace(item.Namespace) && namesInClusterRegistry(&item.Spec.Template.Spec) {
			note("statefulset", item.Namespace, item.Name)
		}
	}

	cronJobs, err := client.BatchV1().CronJobs("").List(ctx, v1meta.ListOptions{})
	if err != nil {
		return 0, nil, fmt.Errorf("listing cronjobs: %w", err)
	}
	for i := range cronJobs.Items {
		item := &cronJobs.Items[i]
		spec := &item.Spec.JobTemplate.Spec.Template.Spec
		if platformNamespace(item.Namespace) && namesInClusterRegistry(spec) {
			note("cronjob", item.Namespace, item.Name)
		}
	}

	examples := found
	if len(examples) > referenceExamples {
		examples = examples[:referenceExamples]
	}

	return len(found), examples, nil
}

// podCouldStillPull reports whether this pod could still have to fetch from the in-cluster registry.
//
// Naming the address is not the same as needing it. A container that has already started holds its image
// in the node's content store, and a restart of it does not go back to a registry — unless its pull
// policy is `Always`, which is the one case where every start is a fetch. So what has to be waited on is
// a container that names the address AND either has not started yet, or will fetch again when it does.
//
// This is the difference between a withdrawal that completes and one that hangs. Measured on the stand:
// a `trickster` rollout had been stuck for two hours for reasons of its own — the new pod, already on
// the upstream registry, never became ready, so the superseded ReplicaSet kept its old pod alive. That
// pod named the in-cluster address, had long since pulled it, and served nothing; waiting on it would
// have held the module on the pull path indefinitely, and any stuck rollout anywhere in the cluster
// would do the same.
//
// What is still waited on, deliberately: anything not yet started. A pod pulling right now is the case
// the whole mechanism exists for.
func podCouldStillPull(pod *v1core.Pod) bool {
	started := make(map[string]bool, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	for _, statuses := range [][]v1core.ContainerStatus{
		pod.Status.ContainerStatuses,
		pod.Status.InitContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	} {
		for i := range statuses {
			status := &statuses[i]
			started[status.Name] = status.State.Running != nil || status.State.Terminated != nil
		}
	}

	needs := func(container *v1core.Container) bool {
		if !strings.HasPrefix(container.Image, registry_const.Host+"/") {
			return false
		}
		if container.ImagePullPolicy == v1core.PullAlways {
			return true
		}

		return !started[container.Name]
	}

	for i := range pod.Spec.Containers {
		if needs(&pod.Spec.Containers[i]) {
			return true
		}
	}
	for i := range pod.Spec.InitContainers {
		if needs(&pod.Spec.InitContainers[i]) {
			return true
		}
	}
	for i := range pod.Spec.EphemeralContainers {
		common := v1core.Container(pod.Spec.EphemeralContainers[i].EphemeralContainerCommon)
		if needs(&common) {
			return true
		}
	}

	return false
}

// platformNamespace answers whether a namespace is one Deckhouse renders into.
func platformNamespace(namespace string) bool {
	return namespace == "kube-system" || strings.HasPrefix(namespace, "d8-")
}

// namesInClusterRegistry answers whether anything in a pod specification pulls from the in-cluster
// registry.
//
// The storage and the controller of this module are excluded by their namespace being asked about
// somewhere else — they pull from the upstream registry, not from themselves. Everything a pod can pull
// is checked, including init containers: the platform's own Deployment names the in-cluster address in
// one of those, and it was the reference that stranded a cluster on the stand.
func namesInClusterRegistry(spec *v1core.PodSpec) bool {
	prefix := registry_const.Host + "/"

	for i := range spec.Containers {
		if strings.HasPrefix(spec.Containers[i].Image, prefix) {
			return true
		}
	}
	for i := range spec.InitContainers {
		if strings.HasPrefix(spec.InitContainers[i].Image, prefix) {
			return true
		}
	}
	for i := range spec.EphemeralContainers {
		if strings.HasPrefix(spec.EphemeralContainers[i].Image, prefix) {
			return true
		}
	}

	return false
}
