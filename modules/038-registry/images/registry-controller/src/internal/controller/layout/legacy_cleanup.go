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

package layout

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// The previous implementation's pull path, removed by this controller and not by Helm.
//
// Those objects — a Service under the name every image reference is built from, the in-cluster
// proxy that answers it, its configuration and its PKI — are marked `helm.sh/resource-policy: keep`
// in the release before this one. That is deliberate: on a `Direct` cluster they ARE the pull path,
// and the release that stops rendering them would otherwise delete them at the instant of the
// handover, before this implementation's node agent has taken over the container runtime
// configuration.
//
// A cluster in that state cannot repair itself: its nodes resolve the in-cluster name to nothing, the
// next control-plane manifest re-rendered onto a new digest cannot be pulled, and with the API gone
// bashible — the thing that would have installed the agent — has nothing to talk to.
//
// So the removal belongs here, where the state that decides it already lives: this controller owns
// the RegistryNode objects and writes the very statuses the decision reads.
var legacyPullPathObjects = []client.Object{
	&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: Namespace}},
	&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "registry-incluster-proxy", Namespace: Namespace}},
	&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry-incluster-proxy-config", Namespace: Namespace}},
	&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry-pki", Namespace: Namespace}},
}

// legacyPullPathMayBeRemoved answers whether the node agent serves the pull path on EVERY node.
//
// Every one, and that is the whole subtlety. A node whose bashible has not run yet still resolves
// the in-cluster address to the old proxy, so removing it while one node lags cuts off exactly that
// node — and the symptom appears not at removal but at the next pull, which may be hours later and
// looks like nothing to do with this.
//
// `reconciled` alone is not enough either: it says the node applied the configuration, while
// `proxyListening` says the agent is actually answering. Between those two the address resolves to
// a socket nobody holds.
//
// An empty list is not consent. It means no layout has been written yet, which is the state right
// after the handover — precisely when the old objects are still the only pull path there is.
func legacyPullPathMayBeRemoved(nodes []registryv1alpha1.RegistryNode) bool {
	if len(nodes) == 0 {
		return false
	}

	for i := range nodes {
		if !nodes[i].Status.Reconciled || !nodes[i].Status.ProxyListening {
			return false
		}
	}

	return true
}

// removeLegacyPullPath deletes what the previous implementation left serving the in-cluster
// address, once nothing depends on it any more.
//
// Deleting an object that is already gone is not an error and not worth reporting: this runs on
// every reconciliation, and after the first successful pass every call is a no-op. What IS worth
// reporting is a failure to delete, because the objects then keep answering an address this
// implementation also serves, and two answers to one address is the state this whole gate exists
// to avoid.
func (r *Reconciler) removeLegacyPullPath(ctx context.Context, nodes []registryv1alpha1.RegistryNode) error {
	if !legacyPullPathMayBeRemoved(nodes) {
		return nil
	}

	for _, obj := range legacyPullPathObjects {
		if err := r.Client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("removing the previous implementation's %T %s: %w",
				obj, obj.GetName(), err)
		}
	}

	return nil
}
