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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	clusterConfigSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigKey             = "cluster-configuration.yaml"

	clusterTypeStatic = "Static"

	requeueOnFailure = 10 * time.Second
)

type Reconciler struct {
	register.Base
}

// ForPredicates drops every NodeGroup event: the group is ensured once at startup, the way the
// create_master_node_group hook did on OnStartup. Reacting to events would also re-create a group
// that an administrator removed on purpose.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(client.Object) bool { return false })}
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
		logger.V(1).Info("skipping: master NodeGroup already exists")
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get NodeGroup %s: %w", masterNodeGroupName, err)
	}

	clusterType, err := r.readClusterType(ctx)
	if err != nil {
		// The node type follows the cluster type, so guessing it would give a static cluster a
		// CloudPermanent master group. Wait for the configuration instead of creating anything.
		logger.Info("cannot determine the cluster type, will retry", "error", err.Error())
		return ctrl.Result{RequeueAfter: requeueOnFailure}, nil
	}

	logger.Info("creating the master NodeGroup", "clusterType", clusterType)

	if err := r.Client.Create(ctx, masterNodeGroup(clusterType)); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		// The NodeGroup validating webhook is served by node-controller itself, so a create
		// issued before the webhook server accepts connections is rejected — retry instead of
		// failing, exactly as the hook did on its own queue.
		logger.Info("creating the master NodeGroup failed, will retry", "error", err.Error())
		return ctrl.Result{RequeueAfter: requeueOnFailure}, nil
	}

	logger.Info("master NodeGroup created")
	return ctrl.Result{}, nil
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
