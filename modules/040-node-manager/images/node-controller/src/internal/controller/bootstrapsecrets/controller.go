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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
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

	engineCAPI = "CAPI"

	// maxSecretSize is the apiserver's own cap on the sum of a Secret's values
	// (k8s.io/kubernetes/pkg/apis/core.MaxSecretSize). The script inlines the
	// minget binary as base64 twice over — 4KiB today, so the margin is wide —
	// and the apiserver's own rejection would name no cause.
	maxSecretSize = 1 << 20
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
}

func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.context = &bashiblecontext.Service{Client: r.Client, Reader: mgr.GetAPIReader()}
	r.derivedStatus = &derived_status.Service{Client: r.Client}
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
		builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == nodecommon.MachineNamespace &&
				obj.GetName() == bootstrap.TemplatesConfigMapName
		})))
}

func (r *Reconciler) allNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	ngs := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngs); err != nil {
		log.FromContext(ctx).Error(err, "failed to list NodeGroups after a bootstrap templates change")
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

	// The sweep rides the NodeGroup passes; expired token Secrets are the only
	// rubbish this controller leaves behind and nothing else collects them.
	if err := CollectExpiredTokens(ctx, r.Client); err != nil {
		logger.Error(err, "failed to collect expired bootstrap tokens")
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
	// A validation error is a statement about the NodeGroup, not about this pass:
	// helm rendered no bootstrap Secret for such a group either, and retrying
	// would only repeat the same verdict.
	if validationErr != "" {
		logger.Info("NodeGroup failed validation, writing no bootstrap secret",
			"nodeGroup", ng.Name, "error", validationErr)
		return ctrl.Result{RequeueAfter: resyncInterval}, nil
	}

	if err := r.writeSecrets(ctx, ng, resolved); err != nil {
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

	return r.applySecret(ctx, manualSecretPrefix+ng.Name, map[string][]byte{
		"cloud-config": cloudConfig,
		"bootstrap.sh": script,
		// helm's toYaml drops the trailing newline; matching it keeps the bytes
		// identical to the Secret helm still renders during the handover.
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
		if err := r.applySecret(ctx, name, data); err != nil {
			return err
		}
	}
	return nil
}

// applySecret writes the Secret server-side. Server-side apply rather than
// create-or-update: these Secrets carry no app label, so they sit outside the
// namespace-scoped Secret informer and a cached read would never find them.
func (r *Reconciler) applySecret(ctx context.Context, name string, data map[string][]byte) error {
	total := 0
	for _, value := range data {
		total += len(value)
	}
	if total > maxSecretSize {
		return fmt.Errorf("bootstrap secret %s/%s is %d bytes, over the %d the apiserver accepts: "+
			"the bootstrap script inlines the minget binary as base64",
			nodecommon.MachineNamespace, name, total, maxSecretSize)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodecommon.MachineNamespace,
			Labels:    map[string]string{"heritage": "deckhouse", "module": "node-manager"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := r.Client.Patch(ctx, secret, client.Apply,
		client.FieldOwner(controllerName), client.ForceOwnership); err != nil {
		return fmt.Errorf("write bootstrap secret %s/%s: %w", nodecommon.MachineNamespace, name, err)
	}

	log.FromContext(ctx).Info("wrote bootstrap secret",
		"secret", nodecommon.MachineNamespace+"/"+name, "bytes", total)
	return nil
}
