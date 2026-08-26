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

package bootstrapsecrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/bootstrap"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	controllerName = "bootstrap-secrets"

	// manualSecretPrefix names the Secret an operator bootstraps a static node
	// from. The name and its three keys are a contract: dhctl
	// (pkg/kubernetes/actions/entity/node.go:125), CAPS (client/bootstrap.go:485)
	// and the documentation all read them.
	manualSecretPrefix = "manual-bootstrap-for-"

	// resyncInterval is shorter than the three hours of validity at which
	// EnsureToken stops reusing a token, so a rotation always reaches the
	// Secrets while the token they carry is still accepted.
	resyncInterval = 30 * time.Minute

	// invalidRequeueInterval retries a NodeGroup the checks rejected. It is far
	// shorter than the resync because the verdict is usually transient — a group
	// applied before its InstanceClass — and nothing else enqueues the group when
	// the InstanceClass finally appears.
	invalidRequeueInterval = time.Minute

	engineCAPI = "CAPI"

	// The reasons of the Warning events a group gets when a pass leaves it
	// without bootstrap Secrets, so `kubectl describe nodegroup` says why.
	eventReasonSkipped = "BootstrapSecretsSkipped"
	eventReasonFailed  = "BootstrapSecretsFailed"
)

func init() {
	register.RegisterController(controllerName, &deckhousev1.NodeGroup{}, &Reconciler{})
}

// Reconciler writes the bootstrap Secrets a joining node reads: the manual one
// for every non-ephemeral NodeGroup, and the CAPI cloud-init for the ephemeral
// groups a CAPI provider serves.
type Reconciler struct {
	register.Base
	context       *bashiblecontext.Service
	derivedStatus *derived_status.Service
	apiReader     client.Reader
}

func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.context = &bashiblecontext.Service{Client: r.Client, Reader: mgr.GetAPIReader()}
	r.derivedStatus = &derived_status.Service{Client: r.Client}
	r.apiReader = mgr.GetAPIReader()
	return nil
}

// ForPredicates: the rendered Secrets depend on the NodeGroup spec only, so the
// status writes other controllers make must not re-render every group.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)}
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A candi update arrives as a chart upgrade: without this watch the Secrets
	// would keep the old script until the next NodeGroup event.
	w.Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.allNodeGroups),
		builder.WithPredicates(inMachineNamespace(bootstrap.TemplatesConfigMapName)))

	// The token Secret is created empty and filled by kube-controller-manager moments
	// later — cluster install. A node bootstrapping in that window bakes in
	// PACKAGES_PROXY_TOKEN=passthrough, which rpp-get never re-reads (config.go resolveToken).
	w.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.allNodeGroups),
		builder.WithPredicates(inMachineNamespace(bashiblecontext.PackagesProxyTokenSecretName)))
}

func inMachineNamespace(name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == nodecommon.MachineNamespace && obj.GetName() == name
	})
}

func (r *Reconciler) allNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	ngs := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngs); err != nil {
		log.FromContext(ctx).Error(err, "failed to list NodeGroups after a bootstrap input change")
		return nil
	}
	requests := make([]reconcile.Request, 0, len(ngs.Items))
	for i := range ngs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ngs.Items[i].Name}})
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Both sweeps ride the NodeGroup passes, and both are logged rather than
	// returned: a NodeGroup must still get its Secrets when collecting somebody
	// else's rubbish fails.
	if err := CollectExpiredTokens(ctx, r.Client); err != nil {
		logger.Error(err, "failed to collect expired bootstrap tokens")
	}
	if err := CollectOrphanedSecrets(ctx, r.Client, r.apiReader); err != nil {
		logger.Error(err, "failed to collect orphaned bootstrap secrets")
	}

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !ng.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	resolved, validationErr, err := r.derivedStatus.ResolveNodeGroup(ctx, ng)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("resolve NodeGroup %s: %w", ng.Name, err)
	}
	// A validation error is a statement about the NodeGroup, not a failure of this
	// pass — helm rendered no bootstrap Secret for such a group either — so it is
	// reported and retried rather than returned as an error.
	if validationErr != "" {
		logger.V(1).Info("NodeGroup failed validation, writing no bootstrap secret",
			"nodeGroup", ng.Name, "error", validationErr)
		// Once a minute for as long as the group stays rejected, which client-go's
		// event correlator collapses: burst 25, then one per 300s, identical
		// messages folded into count++. Do not stretch the requeue to protect it.
		r.Recorder.Event(ng, corev1.EventTypeWarning, eventReasonSkipped, validationErr)
		return ctrl.Result{RequeueAfter: invalidRequeueInterval}, nil
	}

	if err := r.writeSecrets(ctx, ng, resolved); err != nil {
		// Without this the reason lives only in the controller log: the NodeGroup
		// itself shows no sign of why its nodes cannot bootstrap.
		r.Recorder.Event(ng, corev1.EventTypeWarning, eventReasonFailed, err.Error())
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

func (r *Reconciler) writeSecrets(ctx context.Context, ng *deckhousev1.NodeGroup, resolved derived_status.ResolvedNodeGroup) error {
	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral {
		return r.writeCAPISecrets(ctx, ng, resolved)
	}
	return r.writeManualSecret(ctx, ng, resolved)
}

// writeManualSecret writes manual-bootstrap-for-<ng>: the Secret an operator
// bootstraps a static node from, and the one the static MachineDeployment points
// its StaticMachines at (capi/machinedeployment.go:387).
func (r *Reconciler) writeManualSecret(ctx context.Context, ng *deckhousev1.NodeGroup, resolved derived_status.ResolvedNodeGroup) error {
	in, err := r.buildInput(ctx, ng, resolved)
	if err != nil {
		return err
	}

	cloudConfig, err := bootstrap.RenderCloudConfig(in)
	if err != nil {
		return fmt.Errorf("render cloud-config for NodeGroup %s: %w", ng.Name, err)
	}
	script, err := bootstrap.RenderStaticScript(in)
	if err != nil {
		return fmt.Errorf("render bootstrap script for NodeGroup %s: %w", ng.Name, err)
	}
	endpoints, err := yaml.Marshal(in.APIServerEndpoints)
	if err != nil {
		return fmt.Errorf("marshal apiserver endpoints for NodeGroup %s: %w", ng.Name, err)
	}

	return r.applySecret(ctx, ng.Name, manualSecretPrefix+ng.Name, map[string][]byte{
		"cloud-config": cloudConfig,
		"bootstrap.sh": script,
		// helm's toYaml dropped the trailing newline; matching it keeps the bytes
		// identical to the Secret the chart rendered before the handover, so the
		// takeover rewrites nothing.
		"apiserverEndpoints": bytes.TrimRight(endpoints, "\n"),
	})
}

// writeCAPISecrets writes the cloud-init a CAPI Machine boots from, under every
// name the group's MachineDeployments reference.
func (r *Reconciler) writeCAPISecrets(ctx context.Context, ng *deckhousev1.NodeGroup, resolved derived_status.ResolvedNodeGroup) error {
	// An immutable node boots from a per-machine NodeBootstrapConfig referenced
	// through bootstrap.configRef, so it has no cloud-init Secret at all. The MCM
	// machine-class Secret is the capi controller's to write, not this one's.
	if resolved.Engine != engineCAPI || ng.Spec.SystemType == deckhousev1.SystemTypeImmutable {
		return nil
	}

	in, err := r.buildInput(ctx, ng, resolved)
	if err != nil {
		return err
	}
	value, err := bootstrap.RenderCAPICloudConfig(in)
	if err != nil {
		return fmt.Errorf("render CAPI cloud-config for NodeGroup %s: %w", ng.Name, err)
	}

	names, err := r.capiSecretNames(ctx, ng, resolved.Zones, in.ClusterUUID)
	if err != nil {
		return err
	}
	data := map[string][]byte{"format": []byte("cloud-config"), "value": value}
	for _, name := range names {
		if err := r.applySecret(ctx, ng.Name, name, data); err != nil {
			return err
		}
	}
	return nil
}

// applySecret writes the Secret server-side. Server-side apply rather than
// create-or-update: these Secrets carry no app label, so they sit outside the
// namespace-scoped Secret informer and a cached read would never find them.
func (r *Reconciler) applySecret(ctx context.Context, ngName, name string, data map[string][]byte) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodecommon.MachineNamespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "node-manager",
				// helm never set it. It is what CollectOrphanedSecrets finds the Secret by
				// once the NodeGroup is deleted: the rendered CAPI name starts with the group
				// name but cannot be parsed back — group names contain dashes, and a migrated
				// cluster keeps whatever legacy name its MachineDeployment carries
				// (sources.go:151-159).
				ngcommon.MachineDeploymentNodeGroupLabel: ngName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := r.Client.Patch(ctx, secret, client.Apply,
		client.FieldOwner(controllerName), client.ForceOwnership); err != nil {
		return fmt.Errorf("write bootstrap secret %s/%s: %w", nodecommon.MachineNamespace, name, err)
	}

	total := 0
	for _, value := range data {
		total += len(value)
	}
	// "applied", not "wrote": a server-side apply that changes nothing is a no-op
	// on the apiserver, and this line fires on every resync all the same.
	log.FromContext(ctx).Info("applied bootstrap secret",
		"secret", nodecommon.MachineNamespace+"/"+name, "bytes", total)
	return nil
}

// CollectOrphanedSecrets deletes the bootstrap Secrets written for NodeGroups that
// no longer exist. Nothing else collects them: the keep annotation stamped ahead of
// the handover (hooks/set_keep_policy_on_capi_resources.go) relieved helm of pruning
// this namespace.
//
// reader must be uncached on both counts. These Secrets carry no app label, so the
// namespace-scoped Secret informer holds none of them (common/cache.go), and a
// NodeGroup missing from a cold cache is not a deleted NodeGroup.
//
// Metadata-only, because this runs on every pass and the bodies are the rendered
// cloud-init and bootstrap.sh — tens of KB each that the sweep never reads.
func CollectOrphanedSecrets(ctx context.Context, c client.Client, reader client.Reader) error {
	secrets := &metav1.PartialObjectMetadataList{}
	secrets.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
	if err := reader.List(ctx, secrets,
		client.InNamespace(nodecommon.MachineNamespace),
		// Both labels: a Secret an operator wrote by hand carries neither, and is
		// not this controller's to delete.
		client.MatchingLabels{"heritage": "deckhouse", "module": "node-manager"},
		client.HasLabels{ngcommon.MachineDeploymentNodeGroupLabel},
	); err != nil {
		return fmt.Errorf("list bootstrap secrets: %w", err)
	}

	alive := make(map[string]bool, len(secrets.Items))
	var deletions []error
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		ngName := secret.Labels[ngcommon.MachineDeploymentNodeGroupLabel]
		if ngName == "" {
			continue
		}
		if _, checked := alive[ngName]; !checked {
			exists, err := nodeGroupExists(ctx, reader, ngName)
			if err != nil {
				return err
			}
			alive[ngName] = exists
		}
		if alive[ngName] {
			continue
		}
		gone := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace}}
		if err := c.Delete(ctx, gone); err != nil && !apierrors.IsNotFound(err) {
			deletions = append(deletions, fmt.Errorf("delete orphaned bootstrap secret %s: %w", secret.Name, err))
			continue
		}
		log.FromContext(ctx).Info("deleted orphaned bootstrap secret",
			"secret", nodecommon.MachineNamespace+"/"+secret.Name, "nodeGroup", ngName)
	}
	return errors.Join(deletions...)
}

func nodeGroupExists(ctx context.Context, reader client.Reader, name string) (bool, error) {
	ng := &deckhousev1.NodeGroup{}
	err := reader.Get(ctx, types.NamespacedName{Name: name}, ng)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read NodeGroup %s: %w", name, err)
	}
	return true, nil
}
