/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

// effectiveMinReplicas returns the minimum replica count for the waypoint instance
// based on replicasManagement spec. This is used for:
// - Setting Deployment replicas (Static mode)
// - Deciding whether to create a PDB (>= 2)
func effectiveMinReplicas(spec *networkv1alpha1.WaypointInstanceSpec) int32 {
	if spec.ReplicasManagement == nil {
		return 1
	}
	switch spec.ReplicasManagement.Mode {
	case "Static":
		if spec.ReplicasManagement.Static != nil {
			return spec.ReplicasManagement.Static.Replicas
		}
		return 1
	case "HPA":
		if spec.ReplicasManagement.HPA != nil {
			return spec.ReplicasManagement.HPA.MinReplicas
		}
		return 1
	default:
		return 1
	}
}

func (r *WaypointController) ensureWaypointDeployment(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	saName, err := r.ensureWaypointServiceAccount(ctx, instance)
	if err != nil {
		return err
	}

	resources, err := resourcesFromResourcesManagement(instance.Spec.ResourcesManagement)
	if err != nil {
		return err
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, deploy, func() error {
		return r.mutateDeployment(deploy, instance, saName, resources)
	})

	return err
}

func (r *WaypointController) mutateDeployment(deploy *appsv1.Deployment, instance *networkv1alpha1.WaypointInstance, saName string, resources corev1.ResourceRequirements) error {
	manageReplicas := true
	replicas := effectiveMinReplicas(&instance.Spec)

	// When HPA manages replicas, leave deploy.Spec.Replicas as-is so that
	// CreateOrPatch produces no diff for this field and doesn't fight the HPA.
	if instance.Spec.ReplicasManagement != nil && instance.Spec.ReplicasManagement.Mode == "HPA" {
		manageReplicas = false
	}

	if manageReplicas {
		deploy.Spec.Replicas = &replicas
	}

	deploy.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			AppLabelKey:                              AppLabelValue,
			"gateway.networking.k8s.io/gateway-name": resourceBaseName(instance.Name),
		},
	}

	podLabels := podTemplateLabels(instance, r.istioRevision, r.istioNetworkName)

	deploy.Labels = make(map[string]string)
	for k, v := range instanceLabels(instance) {
		deploy.Labels[k] = v
	}
	for k, v := range istioLabels(instance, r.istioRevision, r.istioNetworkName) {
		deploy.Labels[k] = v
	}
	deploy.Labels["gateway.networking.k8s.io/gateway-name"] = resourceBaseName(instance.Name)

	deploy.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: podLabels,
			Annotations: map[string]string{
				"istio.io/rev":         r.istioRevision,
				"prometheus.io/path":   "/stats/prometheus",
				"prometheus.io/port":   "15020",
				"prometheus.io/scrape": "true",
			},
			Namespace: instance.Namespace,
		},
	}

	podSpec, err := waypointPodSpec(waypointPodSpecConfig{
		InstanceName:          instance.Name,
		Namespace:             instance.Namespace,
		ClusterDomain:         r.clusterDomain,
		ProxyImage:            r.proxyImage,
		Resources:             resources,
		ServiceAccount:        saName,
		NodeSelector:          instance.Spec.NodeSelector,
		Tolerations:           instance.Spec.Tolerations,
		IstioRevision:         r.istioRevision,
		IstioNetworkName:      r.istioNetworkName,
		IstioCloudPlatform:    r.istioCloudPlatform,
		IstioClusterID:        r.istioClusterID,
		EnablePodAntiAffinity: replicas >= 2 || (instance.Spec.ReplicasManagement != nil && instance.Spec.ReplicasManagement.Mode == "HPA"),
	})
	if err != nil {
		return err
	}
	deploy.Spec.Template.Spec = podSpec

	// Use a predictable one-pod-at-a-time rolling update strategy.
	deploy.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxSurge:       ptr.To(intstr.FromInt32(1)),
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
		},
	}

	if err := controllerutil.SetControllerReference(instance, deploy, r.scheme); err != nil {
		return err
	}

	klog.V(4).InfoS(
		"Deployment spec set",
		"name", deploy.Name,
		"namespace", deploy.Namespace,
	)

	return nil
}
