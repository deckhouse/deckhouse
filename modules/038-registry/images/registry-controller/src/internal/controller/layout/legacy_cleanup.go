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
		if service, isService := obj.(*corev1.Service); isService {
			if err := r.removeLegacyService(ctx, service.Name); err != nil {
				return err
			}
			continue
		}
		if err := r.Client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("removing the previous implementation's %T %s: %w",
				obj, obj.GetName(), err)
		}
	}

	return nil
}

// legacyServiceSelector is what the previous implementation's Service selected: its in-cluster proxy.
const legacyServiceSelector = "registry-service"

// removeLegacyService deletes the Service of that name only if it is the previous implementation's.
//
// The name is shared: this implementation's store is published under `registry` too, a headless
// Service over the storage replicas, because that is the name every image reference is built from.
// Deleted by name, as it was, the store's own Service went on every reconciliation — and with it the
// in-cluster name, for everything that dials it rather than going through a node agent. Measured in
// an air-gapped cluster: the infrastructure converger failing on `lookup registry.d8-system.svc: no
// such host`, and, in the moments Helm had put the Service back, on the certificate behind it.
//
// Told apart by what it selects, which is what makes it the previous implementation's: its in-cluster
// proxy, not the storage. Deleted on that exact object, so one replaced in between is left alone.
func (r *Reconciler) removeLegacyService(ctx context.Context, name string) error {
	service := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: Namespace}, service)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the Service %s: %w", name, err)
	}
	if service.Spec.Selector["component"] != legacyServiceSelector {
		return nil
	}

	uid, version := service.UID, service.ResourceVersion
	err = r.Client.Delete(ctx, service, client.Preconditions{UID: &uid, ResourceVersion: &version})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("removing the previous implementation's Service %s: %w", name, err)
	}
	return nil
}
