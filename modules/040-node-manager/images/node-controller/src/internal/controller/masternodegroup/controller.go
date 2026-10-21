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

package masternodegroup

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("master-node-group", &deckhousev1.NodeGroup{}, &Reconciler{})
}

const (
	masterNodeGroupName = "master"
	// masterNodeGroupFinalizer holds a deleted master NodeGroup until its spec is copied into the
	// group that replaces it.
	masterNodeGroupFinalizer = "node-manager.deckhouse.io/master-node-group"

	clusterConfigSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigKey             = "cluster-configuration.yaml"

	clusterTypeStatic = "Static"
)

type Reconciler struct {
	register.Base
}

// ForPredicates passes the events of the master NodeGroup only. Nothing deletes that group
// legitimately, dhctl destroy included, so a deleted one is restored with its own spec.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == masterNodeGroupName
	})}
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WatchesRawSource(source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: masterNodeGroupName}})
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	existing := &deckhousev1.NodeGroup{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: masterNodeGroupName}, existing)
	if err == nil {
		return r.reconcileExisting(ctx, existing)
	}
	if !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get NodeGroup %s: %w", masterNodeGroupName, err)
	}

	// The node type follows the cluster type, so guessing it would give a static cluster a
	// CloudPermanent master group — and nodeType is immutable. Fail and retry instead.
	clusterType, err := r.readClusterType(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("creating the master NodeGroup", "clusterType", clusterType)

	// The NodeGroup validating webhook is served by node-controller itself, so a create issued
	// before the webhook server listens is rejected; the workqueue backoff covers that window.
	if err := r.Client.Create(ctx, withFinalizer(masterNodeGroup(clusterType))); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("create NodeGroup %s: %w", masterNodeGroupName, err)
	}

	logger.Info("master NodeGroup created")
	return ctrl.Result{}, nil
}

// reconcileExisting keeps the finalizer on the master NodeGroup and restores the group once it is
// deleted. dhctl creates the group at bootstrap without the finalizer.
func (r *Reconciler) reconcileExisting(ctx context.Context, ng *deckhousev1.NodeGroup) (ctrl.Result, error) {
	if !ng.DeletionTimestamp.IsZero() {
		return r.restore(ctx, ng)
	}
	if controllerutil.ContainsFinalizer(ng, masterNodeGroupFinalizer) {
		return ctrl.Result{}, nil
	}

	patch := client.MergeFromWithOptions(ng.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.AddFinalizer(ng, masterNodeGroupFinalizer)
	if err := r.Client.Patch(ctx, ng, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("add finalizer to NodeGroup %s: %w", masterNodeGroupName, err)
	}

	log.FromContext(ctx).Info("master NodeGroup finalizer added")
	return ctrl.Result{}, nil
}

// restore re-creates the deleted master NodeGroup with its spec, labels and annotations. The new
// group takes the same name, so it is created only after the finalizer lets the old object go.
func (r *Reconciler) restore(ctx context.Context, deleted *deckhousev1.NodeGroup) (ctrl.Result, error) {
	// Not held by this controller: once the group is gone, it is created from the template.
	if !controllerutil.ContainsFinalizer(deleted, masterNodeGroupFinalizer) {
		return ctrl.Result{}, nil
	}
	// Another finalizer, foregroundDeletion of a cascading delete for one, keeps the old object after
	// ours is gone, and the create would collide with it. Its removal updates the group and brings us back.
	if len(deleted.Finalizers) > 1 {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)
	saved := deleted.DeepCopy()

	// The lock turns a stale cached object into a conflict: a plain patch would strip the
	// finalizer from a group already restored under the same name.
	controllerutil.RemoveFinalizer(deleted, masterNodeGroupFinalizer)
	if err := r.Client.Patch(ctx, deleted, client.MergeFromWithOptions(saved, client.MergeFromWithOptimisticLock{})); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer from NodeGroup %s: %w", masterNodeGroupName, err)
	}

	restored := withFinalizer(&deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        masterNodeGroupName,
			Labels:      saved.Labels,
			Annotations: saved.Annotations,
		},
		Spec: saved.Spec,
	})
	// ponytail: a create failing here is retried by the template branch, which loses the saved
	// spec. Keep the saved object across retries if this window ever shows up in practice.
	if err := r.Client.Create(ctx, restored); err != nil {
		return ctrl.Result{}, fmt.Errorf("re-create NodeGroup %s: %w", masterNodeGroupName, err)
	}

	logger.Info("master NodeGroup restored after deletion", "deletedUID", saved.UID, "uid", restored.UID)
	r.Recorder.Event(restored, corev1.EventTypeWarning, "RestoredAfterDeletion",
		"NodeGroup master was deleted and has been re-created with the same spec, labels and annotations")
	return ctrl.Result{}, nil
}

// withFinalizer protects a master NodeGroup from the moment it is created.
func withFinalizer(ng *deckhousev1.NodeGroup) *deckhousev1.NodeGroup {
	controllerutil.AddFinalizer(ng, masterNodeGroupFinalizer)
	return ng
}

// readClusterType reads clusterType out of the cluster configuration. Every failure is reported:
// clusterType is a required field of ClusterConfiguration, so an absent or empty one means the
// configuration is not readable yet, never that the cluster is a cloud one.
func (r *Reconciler) readClusterType(ctx context.Context) (string, error) {
	key := types.NamespacedName{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf("get secret %s: %w", key, err)
	}

	raw, ok := secret.Data[clusterConfigKey]
	if !ok {
		return "", fmt.Errorf("read key %s of secret %s: key is absent", clusterConfigKey, key)
	}

	var cfg struct {
		ClusterType string `json:"clusterType"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", clusterConfigKey, err)
	}
	if cfg.ClusterType == "" {
		return "", fmt.Errorf("parse %s: clusterType is empty", clusterConfigKey)
	}

	return cfg.ClusterType, nil
}

// masterNodeGroup builds the metadata-only NodeGroup the master nodes are registered into. The
// master Node itself is created by kubeadm through bashible, so this object carries no cloud
// instance settings — on a cloud cluster the masters are CloudPermanent.
func masterNodeGroup(clusterType string) *deckhousev1.NodeGroup {
	nodeType := deckhousev1.NodeTypeCloudPermanent
	if clusterType == clusterTypeStatic {
		nodeType = deckhousev1.NodeTypeStatic
	}

	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: masterNodeGroupName},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: nodeType,
			Disruptions: &deckhousev1.DisruptionsSpec{
				ApprovalMode: deckhousev1.DisruptionApprovalModeManual,
			},
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					// Legacy node role, kept for backward compatibility with user software.
					"node-role.kubernetes.io/master": "",
				},
				Taints: []corev1.Taint{{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				}},
			},
		},
	}
}
