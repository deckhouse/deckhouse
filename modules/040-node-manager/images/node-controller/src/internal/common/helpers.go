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

package common

import (
	"context"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func GetNodeGroup(ctx context.Context, r client.Reader, name string) (*v1.NodeGroup, error) {
	ng := &v1.NodeGroup{}
	if err := r.Get(ctx, types.NamespacedName{Name: name}, ng); err != nil {
		return nil, err
	}
	return ng, nil
}

// DefaultDrainTimeout bounds an eviction whose group names no
// nodeDrainTimeoutSecond. Shared rather than copied: the draining controller
// runs the eviction and a NodeOperation waits for one, and an operation that
// allows less than the drain it waits for aborts a healthy drain.
const DefaultDrainTimeout = 10 * time.Minute

// maxDrainTimeout guards the seconds-to-Duration multiplication against
// overflowing into a negative deadline (~292 years) for a value stored before
// the CRD bounded nodeDrainTimeoutSecond. Deliberately not a policy cap — that
// belongs in the CRD.
const maxDrainTimeout = time.Duration(math.MaxInt64)

// DrainTimeout is how long the eviction of a node in this group may run. A
// group that cannot be read falls back to the default: the node is waiting on
// that eviction, so a bound that is only usually right beats refusing to start.
func DrainTimeout(ctx context.Context, r client.Reader, ngName string) time.Duration {
	if ngName == "" {
		return DefaultDrainTimeout
	}
	ng, err := GetNodeGroup(ctx, r, ngName)
	if err != nil {
		log.FromContext(ctx).V(1).Info("could not read the group's drain timeout, using the default",
			"nodeGroup", ngName, "default", DefaultDrainTimeout, "error", err.Error())
		return DefaultDrainTimeout
	}
	if ng.Spec.NodeDrainTimeoutSecond == nil {
		return DefaultDrainTimeout
	}
	seconds := int64(*ng.Spec.NodeDrainTimeoutSecond)
	if seconds <= 0 {
		return DefaultDrainTimeout
	}
	if seconds > int64(maxDrainTimeout/time.Second) {
		return maxDrainTimeout
	}
	return time.Duration(seconds) * time.Second
}

// BootstrapTokenNodeGroupLabel names the NodeGroup a bootstrap-token secret was
// issued for. order_bootstrap_token maintains one rotating secret per group.
const BootstrapTokenNodeGroupLabel = "node-manager.deckhouse.io/node-group"

// BootstrapTokens returns the newest unexpired bootstrap token of every
// NodeGroup that has one, keyed by group. Kept in step with
// groupBootstrapToken in dhctl's pkg/operations/bootstrap/steps_immutable_join.go.
func BootstrapTokens(ctx context.Context, r client.Reader) (map[string]string, error) {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets,
		client.InNamespace(KubeSystemNamespace),
		client.HasLabels{BootstrapTokenNodeGroupLabel},
	); err != nil {
		return nil, fmt.Errorf("list bootstrap tokens: %w", err)
	}

	tokens := make(map[string]string, len(secrets.Items))
	newest := make(map[string]time.Time, len(secrets.Items))
	for i := range secrets.Items {
		sec := &secrets.Items[i]
		ng := sec.Labels[BootstrapTokenNodeGroupLabel]
		if sec.Type != corev1.SecretTypeBootstrapToken || ng == "" {
			continue
		}
		if raw, ok := sec.Data["expiration"]; ok {
			expire, err := time.Parse(time.RFC3339, string(raw))
			if err != nil || time.Until(expire) < 0 {
				continue
			}
		}
		id, hasID := sec.Data["token-id"]
		secretPart, hasSecret := sec.Data["token-secret"]
		if !hasID || !hasSecret {
			continue
		}
		if _, seen := tokens[ng]; seen && !sec.CreationTimestamp.After(newest[ng]) {
			continue
		}
		tokens[ng] = string(id) + "." + string(secretPart)
		newest[ng] = sec.CreationTimestamp.Time
	}
	return tokens, nil
}

func GetNodesForNodeGroup(ctx context.Context, r client.Reader, ngName string) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList, client.MatchingLabels{NodeGroupLabel: ngName}); err != nil {
		return nil, fmt.Errorf("failed to list nodes for nodegroup %s: %w", ngName, err)
	}
	return nodeList.Items, nil
}

func GetConfigurationChecksums(ctx context.Context, r client.Reader) (map[string]string, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: MachineNamespace,
		Name:      ConfigurationChecksumsSecretName,
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", MachineNamespace, ConfigurationChecksumsSecretName, err)
	}

	checksums := make(map[string]string, len(secret.Data))
	for k, val := range secret.Data {
		checksums[k] = string(val)
	}
	return checksums, nil
}

func SecretToAllNodeGroups(ctx context.Context, r client.Reader) []reconcile.Request {
	ngList := &v1.NodeGroupList{}
	if err := r.List(ctx, ngList); err != nil {
		log.FromContext(ctx).Error(err, "failed to list nodegroups for secret event")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(ngList.Items))
	for _, ng := range ngList.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ng.Name}})
	}
	return requests
}

func ChecksumSecretPredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == MachineNamespace && obj.GetName() == ConfigurationChecksumsSecretName
	})
}
