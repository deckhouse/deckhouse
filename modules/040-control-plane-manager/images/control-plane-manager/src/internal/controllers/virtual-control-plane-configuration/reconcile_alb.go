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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"net"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const albManifestKey = "alb.yaml.tpl"

// exposeHost builds a per-VCP hostname (example: api.<name>.<suffix>) used for ALB SNI routing
func exposeHost(role string, vcp *controlplanev1alpha1.VirtualControlPlane) string {
	return fmt.Sprintf("%s.%s.%s", role, vcp.Name, constants.VirtualExposeDomainSuffix)
}

func apiExposeHost(vcp *controlplanev1alpha1.VirtualControlPlane) string {
	return exposeHost("api", vcp)
}

func konnExposeHost(vcp *controlplanev1alpha1.VirtualControlPlane) string {
	return exposeHost("konn", vcp)
}

func packagesExposeHost(vcp *controlplanev1alpha1.VirtualControlPlane) string {
	return exposeHost("packages", vcp)
}

// reconcileALB applies the per-VCP ALB objects. Only spec/data are reconciled, leaving anything
// the ALB module writes back (status, injected labels) untouched.
func (r *reconciler) reconcileALB(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (reconcile.Result, error) {
	raw, ok := configSecret.Data[albManifestKey]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("config Secret missing %q", albManifestKey)
	}

	objects, err := parseManifestDocs(raw, vcp.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, target := range objects {
		// Cross-namespace objects (e.g. ReferenceGrant in d8-cloud-instance-manager) cannot be
		// owned by a namespaced VirtualControlPlane; apply without controller ownerRef.
		sameNS := target.GetNamespace() == "" || target.GetNamespace() == vcp.Namespace
		mutate := patchSpecData
		if sameNS {
			if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
				return reconcile.Result{}, err
			}
			mutate = patchSpecDataAndOwnerRefs
		}
		if err := applyObject(ctx, r.client, target, mutate); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// patchSpecData reconciles only the spec and data fields, skipping the patch when both already match.
func patchSpecData(current, target *unstructured.Unstructured) (client.Object, bool) {
	if equality.Semantic.DeepEqual(current.Object["spec"], target.Object["spec"]) &&
		equality.Semantic.DeepEqual(current.Object["data"], target.Object["data"]) {
		return nil, false
	}
	if spec, ok := target.Object["spec"]; ok {
		current.Object["spec"] = spec
	}
	if data, ok := target.Object["data"]; ok {
		current.Object["data"] = data
	}
	return current, true
}

// albVIP resolves the external ALB address for this VCP: ALBInstance.status.gateway
// Returns "" (not an error) until the LoadBalancer address is assigned.
//
// The externally-reachable address is read straight off the "d8-alb-<gw>-loadbalancer"
// Service the ALB module provisions alongside the Gateway for that inlet type.
func (r *reconciler) albVIP(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (string, error) {
	albi := &unstructured.Unstructured{}
	albi.SetAPIVersion("network.deckhouse.io/v1alpha1")
	albi.SetKind("ALBInstance")
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: vcp.Namespace, Name: vcp.Name}, albi); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get ALBInstance: %w", err)
	}

	gatewayName, _, _ := unstructured.NestedString(albi.Object, "status", "gateway")
	if gatewayName == "" {
		gatewayName = vcp.Name // spec.gatewayName
	}

	lbService, err := r.getService(ctx, vcp.Namespace, fmt.Sprintf("d8-alb-%s-loadbalancer", gatewayName))
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get ALB LoadBalancer Service: %w", err)
	}

	for _, ingress := range lbService.Status.LoadBalancer.Ingress {
		if ingress.IP != "" && net.ParseIP(ingress.IP) != nil {
			return ingress.IP, nil
		}
		if ingress.Hostname != "" {
			// MVP is intentionally IP-only: the same value is used for apiserver
			// --advertise-address and as an IP SAN, both of which require a concrete IP.
			return "", nil
		}
	}

	return "", nil
}
