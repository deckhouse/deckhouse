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
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
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

// reconcileALB applies the per-VCP ALB objects
func (r *reconciler) reconcileALB(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, configSecret *corev1.Secret) (reconcile.Result, error) {
	objects, err := albManifests(configSecret, vcp)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, target := range objects {
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(target.GroupVersionKind())

		err := r.client.Get(ctx, client.ObjectKeyFromObject(target), current)
		if apierrors.IsNotFound(err) {
			if err := r.client.Create(ctx, target); err != nil {
				return reconcile.Result{}, fmt.Errorf("create %s %s: %w", target.GetKind(), target.GetName(), err)
			}
			continue
		}
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("get %s %s: %w", target.GetKind(), target.GetName(), err)
		}
	}

	return reconcile.Result{}, nil
}

func albManifests(configSecret *corev1.Secret, vcp *controlplanev1alpha1.VirtualControlPlane) ([]*unstructured.Unstructured, error) {
	raw, ok := configSecret.Data[albManifestKey]
	if !ok {
		return nil, fmt.Errorf("config Secret missing %q", albManifestKey)
	}

	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	var objects []*unstructured.Unstructured
	for _, doc := range strings.Split(string(raw), "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			return nil, fmt.Errorf("decode alb manifest: %w", err)
		}
		if len(obj.Object) == 0 {
			continue
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

// albVIP resolves the external ALB address for this VCP: ALBInstance.status.gateway
// Returns "" (not an error) until the LoadBalancer address is assigned.
func (r *reconciler) albVIP(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (string, error) {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	albi := &unstructured.Unstructured{}
	albi.SetAPIVersion("network.deckhouse.io/v1alpha1")
	albi.SetKind("ALBInstance")
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: vcp.Name}, albi); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get ALBInstance: %w", err)
	}

	gatewayName, _, _ := unstructured.NestedString(albi.Object, "status", "gateway")
	if gatewayName == "" {
		gatewayName = vcp.Name // spec.gatewayName
	}

	gateway := &unstructured.Unstructured{}
	gateway.SetAPIVersion("gateway.networking.k8s.io/v1")
	gateway.SetKind("Gateway")
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: gatewayName}, gateway); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get Gateway %s: %w", gatewayName, err)
	}

	addresses, _, _ := unstructured.NestedSlice(gateway.Object, "status", "addresses")
	for _, a := range addresses {
		addr, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		value, _ := addr["value"].(string)
		if value == "" {
			continue
		}
		if t, _ := addr["type"].(string); t == "" || t == "IPAddress" {
			return value, nil
		}
	}

	return "", nil
}
