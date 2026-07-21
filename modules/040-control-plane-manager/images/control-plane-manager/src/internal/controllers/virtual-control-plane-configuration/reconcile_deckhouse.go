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
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	deckhouseSystemNamespace    = "d8-system"
	deckhouseDeploymentName     = "deckhouse"
	deckhouseContainerName      = "deckhouse"
	deckhouseRegistrySecretName = "deckhouse-registry"

	deckhouseClusterConfigurationSecretName = "d8-cluster-configuration"
	deckhouseClusterUUIDConfigMapName       = "d8-cluster-uuid"

	// The ModuleConfig CRD is installed by the running deckhouse pod itself,
	// so its absence right after the Deployment rollout is expected.
	requeueIntervalOnMissingModuleConfigCRD = 10 * time.Second
)

//go:embed deckhouse/manifests/deployment.yaml
var deckhouseDeploymentYAML string

//go:embed deckhouse/manifests/moduleconfigs.yaml
var deckhouseModuleConfigsYAML []byte

// reconcileDeckhouse installs a Deckhouse instance for the tenant cluster.
// The deckhouse-controller pod runs in the parent cluster (vcp-<name>) with
// the tenant admin kubeconfig (not-self-hosted mode), and the tenant cluster
// is seeded with the resources dhctl bootstrap would otherwise create.
func (r *reconciler) reconcileDeckhouse(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	albVIP string,
) (reconcile.Result, error) {
	_, tc, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build tenant clients: %w", err)
	}

	// 1. Tenant: d8-system Namespace. No RBAC is needed: the pod authenticates
	//    via super-admin.conf, and the module renders its own ServiceAccount.
	if err := reconcileTenantNamespace(ctx, tc); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile tenant d8-system namespace: %w", err)
	}

	// 2. Tenant: registry secret (modules reference it for image pulls).
	if err := r.reconcileTenantRegistrySecret(ctx, tc); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile tenant registry secret: %w", err)
	}

	// 3. Tenant: ClusterConfiguration read by the global discovery hooks.
	if err := reconcileTenantClusterConfigurationSecret(ctx, tc, vcp); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile tenant cluster configuration: %w", err)
	}

	// 4. Tenant: stable cluster UUID.
	if err := reconcileTenantClusterUUIDConfigMap(ctx, tc, vcp); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile tenant cluster uuid: %w", err)
	}

	// 5. Tenant: kube-dns Service with the fixed cluster DNS address, so
	//    global discovery works before any DNS module is deployed.
	if err := reconcileTenantKubeDNSService(ctx, tc); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile tenant kube-dns service: %w", err)
	}

	// 6. Parent: registry secret copy for image pulls in vcp-<name>.
	if err := r.reconcileParentRegistrySecret(ctx, vcp); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile parent registry secret: %w", err)
	}

	// 7. Parent: the deckhouse Deployment itself.
	if err := r.reconcileDeckhouseDeployment(ctx, vcp, albVIP); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconcile deckhouse Deployment: %w", err)
	}

	// 8. Tenant: ModuleConfigs; requeue until the pod installs the CRD.
	if res, err := reconcileTenantModuleConfigs(ctx, tc); err != nil || !res.IsZero() {
		return res, err
	}

	return reconcile.Result{}, nil
}

// reconcileTenantNamespace is create-only: the namespace only has to exist
// before the deckhouse pod starts; afterwards deckhouse owns it.
func reconcileTenantNamespace(ctx context.Context, tc client.Client) error {
	target := buildTargetTenantNamespace()

	_, err := getTenantNamespace(ctx, tc, target.Name)
	if apierrors.IsNotFound(err) {
		return createTenantNamespace(ctx, tc, target)
	}

	return err
}

func buildTargetTenantNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: deckhouseSystemNamespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
	}
}

func (r *reconciler) reconcileTenantRegistrySecret(ctx context.Context, tc client.Client) error {
	parent, err := r.getSecret(ctx, deckhouseSystemNamespace, deckhouseRegistrySecretName)
	if apierrors.IsNotFound(err) {
		// Nothing to copy (e.g. a registry-less dev install).
		return nil
	}
	if err != nil {
		return fmt.Errorf("get parent registry secret: %w", err)
	}

	target := buildTargetRegistrySecret(parent, deckhouseSystemNamespace)
	// The deckhouse module's chart renders this secret too; without helm
	// adoption metadata the release install fails with "invalid ownership
	// metadata" (same pattern as dhctl's DeckhouseRegistrySecret).
	target.Labels["app.kubernetes.io/managed-by"] = "Helm"
	target.Labels["app"] = "registry"
	target.Annotations = map[string]string{
		"meta.helm.sh/release-name":      "deckhouse",
		"meta.helm.sh/release-namespace": deckhouseSystemNamespace,
	}

	current, err := getTenantSecret(ctx, tc, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return createTenantSecret(ctx, tc, target)
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(current.Data, target.Data) &&
		isMetadataSubset(target.Labels, current.Labels) &&
		isMetadataSubset(target.Annotations, current.Annotations) {
		return nil
	}

	base := current.DeepCopy()
	current.Data = target.Data
	current.Labels = mergeMetadata(current.Labels, target.Labels)
	current.Annotations = mergeMetadata(current.Annotations, target.Annotations)

	return patchTenantSecret(ctx, tc, base, current)
}

// isMetadataSubset reports whether every target label/annotation is present
// on the current object with the same value.
func isMetadataSubset(target, current map[string]string) bool {
	for key, value := range target {
		if current[key] != value {
			return false
		}
	}
	return true
}

// buildTargetRegistrySecret builds a copy of the parent cluster's
// deckhouse-registry Secret for the given namespace.
func buildTargetRegistrySecret(parent *corev1.Secret, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deckhouseRegistrySecretName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: parent.Type,
		Data: maps.Clone(parent.Data),
	}
}

func reconcileTenantClusterConfigurationSecret(
	ctx context.Context,
	tc client.Client,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) error {
	target, err := buildTargetTenantClusterConfigurationSecret(vcp)
	if err != nil {
		return fmt.Errorf("build ClusterConfiguration: %w", err)
	}

	current, err := getTenantSecret(ctx, tc, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return createTenantSecret(ctx, tc, target)
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(current.Data, target.Data) {
		return nil
	}

	base := current.DeepCopy()
	current.Data = target.Data

	return patchTenantSecret(ctx, tc, base, current)
}

func buildTargetTenantClusterConfigurationSecret(vcp *controlplanev1alpha1.VirtualControlPlane) (*corev1.Secret, error) {
	data, err := yaml.Marshal(map[string]any{
		"apiVersion":        "deckhouse.io/v1",
		"kind":              "ClusterConfiguration",
		"clusterType":       "Static",
		"kubernetesVersion": vcp.Spec.KubernetesVersion,
		"clusterDomain":     constants.DefaultTenantClusterDomain,
		"serviceSubnetCIDR": constants.DefaultTenantServiceSubnetCIDR,
		"podSubnetCIDR":     constants.DefaultTenantPodSubnetCIDR,
		"defaultCRI":        "Containerd",
	})
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deckhouseClusterConfigurationSecretName,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				"name":                     deckhouseClusterConfigurationSecretName,
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"cluster-configuration.yaml": data,
		},
	}, nil
}

// reconcileTenantClusterUUIDConfigMap is create-only: the UUID identifies the
// tenant cluster for its whole lifetime and must never change.
func reconcileTenantClusterUUIDConfigMap(
	ctx context.Context,
	tc client.Client,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) error {
	target := buildTargetTenantClusterUUIDConfigMap(vcp)

	_, err := getTenantConfigMap(ctx, tc, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return createTenantConfigMap(ctx, tc, target)
	}

	return err
}

func buildTargetTenantClusterUUIDConfigMap(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deckhouseClusterUUIDConfigMapName,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Data: map[string]string{
			"cluster-uuid": string(vcp.UID),
		},
	}
}

// reconcileTenantKubeDNSService is create-only: once the tenant's own DNS
// module takes over the Service, the VCP manager must not fight it.
func reconcileTenantKubeDNSService(ctx context.Context, tc client.Client) error {
	target := buildTargetTenantKubeDNSService()

	_, err := getTenantService(ctx, tc, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return createTenantService(ctx, tc, target)
	}

	return err
}

func buildTargetTenantKubeDNSService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-dns",
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				"k8s-app": "kube-dns",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: constants.DefaultTenantClusterDNS,
			Selector: map[string]string{
				"k8s-app": "kube-dns",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "dns",
					Protocol:   corev1.ProtocolUDP,
					Port:       53,
					TargetPort: intstr.FromInt32(53),
				},
				{
					Name:       "dns-tcp",
					Protocol:   corev1.ProtocolTCP,
					Port:       53,
					TargetPort: intstr.FromInt32(53),
				},
			},
		},
	}
}

func (r *reconciler) reconcileParentRegistrySecret(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) error {
	parent, err := r.getSecret(ctx, deckhouseSystemNamespace, deckhouseRegistrySecretName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get parent registry secret: %w", err)
	}

	target := buildTargetRegistrySecret(parent, constants.VirtualControlPlaneNamespacePrefix+vcp.Name)

	current, err := r.getSecret(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return err
		}

		return r.createSecret(ctx, target)
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(current.Data, target.Data) {
		return nil
	}

	base := current.DeepCopy()
	current.Data = target.Data

	return r.patchSecret(ctx, base, current)
}

func (r *reconciler) reconcileDeckhouseDeployment(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	albVIP string,
) error {
	image, err := r.getParentDeckhouseImage(ctx)
	if err != nil {
		return fmt.Errorf("get parent deckhouse image: %w", err)
	}

	target, err := buildTargetDeckhouseDeployment(vcp, image, albVIP)
	if err != nil {
		return err
	}
	if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
		return err
	}

	current, err := r.getDeployment(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return r.createDeployment(ctx, target)
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(current.Spec, target.Spec) {
		return nil
	}

	base := current.DeepCopy()
	current.Spec = target.Spec

	return r.patchDeployment(ctx, base, current)
}

func buildTargetDeckhouseDeployment(
	vcp *controlplanev1alpha1.VirtualControlPlane,
	image string,
	albVIP string,
) (*appsv1.Deployment, error) {
	namespace := vcpNamespace(vcp)

	rendered := strings.NewReplacer(
		"${NAMESPACE}", namespace,
		"${IMAGE_DECKHOUSE}", image,
		"${VCP_API_VIP}", albVIP,
	).Replace(deckhouseDeploymentYAML)

	deployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(rendered), deployment); err != nil {
		return nil, fmt.Errorf("unmarshal deckhouse Deployment: %w", err)
	}

	return deployment, nil
}

func reconcileTenantModuleConfigs(ctx context.Context, tc client.Client) (reconcile.Result, error) {
	objects, err := parseManifestDocs(deckhouseModuleConfigsYAML, "")
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, target := range objects {
		if err := applyObject(ctx, tc, target, patchTenantObject); err != nil {
			if isMissingModuleConfigCRD(err) {
				log.FromContext(ctx).Info("ModuleConfig CRD is not installed by the tenant deckhouse yet, requeueing")
				return reconcile.Result{RequeueAfter: requeueIntervalOnMissingModuleConfigCRD}, nil
			}

			return reconcile.Result{}, fmt.Errorf("apply tenant ModuleConfig: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

// isMissingModuleConfigCRD reports whether the error means the ModuleConfig
// CRD has not been installed by the tenant deckhouse yet.
func isMissingModuleConfigCRD(err error) bool {
	var noKind *meta.NoKindMatchError
	var noResource *meta.NoResourceMatchError

	return errors.As(err, &noKind) || errors.As(err, &noResource)
}

// Kubernetes I/O helpers (tenant cluster).
// The tenant client is built per-VCP (see tenantClients), so unlike the
// parent-cluster helpers on *reconciler these take it as an argument.

// getParentDeckhouseImage reads the image from the parent cluster's own
// deckhouse Deployment, so the tenant instance follows the parent's releases.
func (r *reconciler) getParentDeckhouseImage(ctx context.Context) (string, error) {
	deployment, err := r.getDeployment(ctx, deckhouseSystemNamespace, deckhouseDeploymentName)
	if err != nil {
		return "", err
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == deckhouseContainerName {
			return container.Image, nil
		}
	}

	return "", fmt.Errorf("no %q container", deckhouseContainerName)
}

// Namespace
func getTenantNamespace(ctx context.Context, tc client.Client, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	err := tc.Get(ctx, client.ObjectKey{Name: name}, ns)
	return ns, err
}

func createTenantNamespace(ctx context.Context, tc client.Client, ns *corev1.Namespace) error {
	return tc.Create(ctx, ns)
}

// Secret
func getTenantSecret(ctx context.Context, tc client.Client, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := tc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	return secret, err
}

func createTenantSecret(ctx context.Context, tc client.Client, secret *corev1.Secret) error {
	return tc.Create(ctx, secret)
}

// patchTenantSecret patches only .data, which has a single writer (this
// controller), so a merge patch without optimistic lock is safe.
func patchTenantSecret(ctx context.Context, tc client.Client, base, secret *corev1.Secret) error {
	return tc.Patch(ctx, secret, client.MergeFrom(base))
}

// ConfigMap
func getTenantConfigMap(ctx context.Context, tc client.Client, namespace, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := tc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap)
	return configMap, err
}

func createTenantConfigMap(ctx context.Context, tc client.Client, configMap *corev1.ConfigMap) error {
	return tc.Create(ctx, configMap)
}

// Service
func getTenantService(ctx context.Context, tc client.Client, namespace, name string) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := tc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, service)
	return service, err
}

func createTenantService(ctx context.Context, tc client.Client, service *corev1.Service) error {
	return tc.Create(ctx, service)
}
