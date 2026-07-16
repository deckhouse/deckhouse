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
	"crypto/x509"
	_ "embed"
	"fmt"
	"maps"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	bashibleapiserver "control-plane-manager/internal/controllers/virtual-control-plane-configuration/bashible-apiserver"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	dhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	certutil "k8s.io/client-go/util/cert"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	bashibleDeckhouseNamespace = "d8-cloud-instance-manager"

	bashibleDeploymentName    = "bashible-apiserver"
	bashibleServiceName       = "bashible-api"
	bashibleAppLabel          = "bashible-apiserver"
	bashibleSecurePort        = 4221
	bashibleNestedServicePort = 443

	bashibleKubeconfigSecretName = "bashible-apiserver-kubeconfig"
	bashibleContextSecretName    = "bashible-apiserver-context"
	bashibleRegistrySecretName   = "deckhouse-registry"
	bashibleRegistrySecretNS     = "d8-system"
	bashibleFilesConfigMapName   = "bashible-apiserver-files"
	bashibleTLSSecretName        = "bashible-apiserver-tls"

	bashibleAPIServiceName    = "v1alpha1.bashible.deckhouse.io"
	bashibleAPIGroup          = "bashible.deckhouse.io"
	bashibleAPIVersion        = "v1alpha1"
	bashibleEndpointSliceName = "bashible-api-manual"
)

func (r *reconciler) reconcileBashibleApiserver(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
	apiserverService *corev1.Service,
	pkiSecret *corev1.Secret,
	adminSecret *corev1.Secret,
	joinToken string,
	albVIP string,
) (reconcile.Result, error) {
	// 1. Parent: Exclusive kubeconfig for bashible-apiserver that provides access to the nested kube-apiserver.
	if _, res, err := r.reconcileBashibleKubeconfigSecret(ctx, vcp, apiserverService, pkiSecret); err != nil || !res.IsZero() {
		return res, err
	}

	// 2. Build a nested client that provides access to the nested cluster.
	nestedClient, err := bashibleapiserver.BuildNestedClient(adminSecret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build nested client: %w", err)
	}

	// 3. Nested: RBAC
	if res, err := r.reconcileBashibleRBAC(ctx, nestedClient); err != nil || !res.IsZero() {
		return res, err
	}

	// 4. Nested: CRDs
	if res, err := r.reconcileBashibleCRDs(ctx, nestedClient); err != nil || !res.IsZero() {
		return res, err
	}

	// 5. Nested: Context Secret
	if res, err := r.reconcileBashibleContext(ctx, nestedClient, vcp, pkiSecret, joinToken, configSecret); err != nil || !res.IsZero() {
		return res, err
	}

	// 6. Nested: Registry Secret
	if res, err := r.reconcileBashibleRegistrySecret(ctx, nestedClient); err != nil || !res.IsZero() {
		return res, err
	}

	// 7. Parent: TLS
	tlsSecret, res, err := r.reconcileBashibleTLSSecret(ctx, vcp, pkiSecret)
	if err != nil || !res.IsZero() {
		return res, err
	}

	// 8. Parent: Files ConfigMap
	if res, err := r.reconcileBashibleFilesConfigMap(ctx, vcp); err != nil || !res.IsZero() {
		return res, err
	}

	// 9. Parent: Service
	_, res, err = r.reconcileBashibleService(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}

	// 10. Parent: Deployment
	if res, err := r.reconcileBashibleDeployment(ctx, vcp); err != nil || !res.IsZero() {
		return res, err
	}

	// 11. Parent: APIService
	if res, err := r.reconcileBashibleAPIService(ctx, nestedClient, tlsSecret, albVIP); err != nil || !res.IsZero() {
		return res, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileBashibleKubeconfigSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	apiserverService *corev1.Service,
	pkiSecret *corev1.Secret,
) (*corev1.Secret, reconcile.Result, error) {
	return r.reconcileKubeconfigSecretFiles(
		ctx,
		vcp,
		apiserverService,
		pkiSecret,
		bashibleKubeconfigSecretName,
		[]kubeconfig.File{kubeconfig.BashibleApiserver},
		fmt.Sprintf("https://%s:6443", apiserverService.Spec.ClusterIP),
	)
}

//go:embed bashible-apiserver/manifests/rbac.yaml
var bashibleRBACYAML string

func (r *reconciler) reconcileBashibleRBAC(ctx context.Context, nestedClient client.Client) (reconcile.Result, error) {
	docs := dhctlyaml.SplitYAML(bashibleRBACYAML)
	for _, doc := range docs {
		if len(doc) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			return reconcile.Result{}, fmt.Errorf("unmarshal rbac manifest: %w", err)
		}

		gvk := obj.GroupVersionKind()
		if gvk.Empty() {
			continue
		}

		key := client.ObjectKeyFromObject(obj)
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(gvk)

		err := nestedClient.Get(ctx, key, current)
		if apierrors.IsNotFound(err) {
			if err := nestedClient.Create(ctx, obj); err != nil {
				return reconcile.Result{}, fmt.Errorf("create %s/%s: %w", gvk.Kind, key, err)
			}
			continue
		}
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("get %s/%s: %w", gvk.Kind, key, err)
		}

		obj.SetResourceVersion(current.GetResourceVersion())
		if err := nestedClient.Patch(ctx, obj, client.Merge); err != nil {
			return reconcile.Result{}, fmt.Errorf("patch %s/%s: %w", gvk.Kind, key, err)
		}
	}

	return reconcile.Result{}, nil
}

//go:embed bashible-apiserver/manifests/crds.yaml
var bashibleCRDYAML string

func (r *reconciler) reconcileBashibleCRDs(ctx context.Context, nestedClient client.Client) (reconcile.Result, error) {
	docs := dhctlyaml.SplitYAML(bashibleCRDYAML)
	for _, doc := range docs {
		if len(doc) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
			return reconcile.Result{}, err
		}

		key := client.ObjectKeyFromObject(obj)
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(obj.GroupVersionKind())

		err := nestedClient.Get(ctx, key, current)
		if apierrors.IsNotFound(err) {
			if err := nestedClient.Create(ctx, obj); err != nil {
				return reconcile.Result{}, fmt.Errorf("create CRD: %w", err)
			}
			continue
		}
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileBashibleContext(
	ctx context.Context,
	nestedClient client.Client,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
	joinToken string,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	contextInputYAML, err := bashibleapiserver.BuildContextInputYAML(bashibleapiserver.ContextInputParams{
		VCP:          vcp,
		CA:           pkiSecret.Data["ca.crt"],
		JoinToken:    joinToken,
		ClusterUUID:  string(configSecret.Data["cluster-uuid"]),
		APIHost:      apiExposeHost(vcp),
		PackagesHost: packagesExposeHost(vcp),
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build context input: %w", err)
	}

	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleContextSecretName,
			Namespace: bashibleDeckhouseNamespace,
			Labels: map[string]string{
				"app": bashibleAppLabel,
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"input.yaml": contextInputYAML,
		},
	}

	current := &corev1.Secret{}
	key := client.ObjectKeyFromObject(target)
	err = nestedClient.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nestedClient.Create(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	base := current.DeepCopy()
	current.StringData = target.StringData
	current.Labels = target.Labels
	return reconcile.Result{}, nestedClient.Patch(ctx, current, client.MergeFrom(base))
}

func (r *reconciler) reconcileBashibleRegistrySecret(
	ctx context.Context,
	nestedClient client.Client,
) (reconcile.Result, error) {
	parentSecret, err := r.getSecret(ctx, bashibleRegistrySecretNS, bashibleRegistrySecretName)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleRegistrySecretName,
			Namespace: bashibleRegistrySecretNS,
		},
		Type: parentSecret.Type,
		Data: maps.Clone(parentSecret.Data),
	}

	current := &corev1.Secret{}
	key := client.ObjectKeyFromObject(target)
	err = nestedClient.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nestedClient.Create(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	base := current.DeepCopy()
	current.Data = target.Data
	current.Type = target.Type
	return reconcile.Result{}, nestedClient.Patch(ctx, current, client.MergeFrom(base))
}

func (r *reconciler) reconcileBashibleTLSSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
) (*corev1.Secret, reconcile.Result, error) {
	target := buildTargetBashibleTLSSecret(vcp)

	current, err := r.getSecret(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		data, err := buildBashibleTLSSecretData(pkiSecret)
		if err != nil {
			return nil, reconcile.Result{}, fmt.Errorf("generate bashible-apiserver TLS Secret data: %w", err)
		}
		target.Data = data

		return target, reconcile.Result{}, r.createSecret(ctx, target)
	}
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("get bashible-apiserver TLS Secret: %w", err)
	}

	return current, reconcile.Result{}, nil
}

func buildTargetBashibleTLSSecret(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Secret {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleTLSSecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func buildBashibleTLSSecretData(pkiSecret *corev1.Secret) (map[string][]byte, error) {
	namespace := bashibleDeckhouseNamespace

	caCert, err := pkiutil.ParseCertificatePEM(pkiSecret.Data["ca.crt"])
	if err != nil {
		return nil, fmt.Errorf("parse VCP CA cert: %w", err)
	}
	caKey, err := pkiutil.ParsePrivateKeyPEM(pkiSecret.Data["ca.key"])
	if err != nil {
		return nil, fmt.Errorf("parse VCP CA key: %w", err)
	}

	cfg := pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "bashible-api." + namespace + ".svc",
			AltNames: certutil.AltNames{
				DNSNames: []string{
					"bashible-api." + namespace + ".svc",
					"bashible-api." + namespace + ".svc.cluster.local",
				},
			},
			Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		NotAfter:            time.Now().AddDate(10, 0, 0),
		EncryptionAlgorithm: pkiconstants.EncryptionAlgorithmRSA2048,
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("sign bashible-apiserver serving cert: %w", err)
	}

	keyPEM, err := pkiutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("marshal bashible-apiserver key: %w", err)
	}

	return map[string][]byte{
		"ca.crt":        pkiSecret.Data["ca.crt"],
		"apiserver.crt": pkiutil.EncodeCertificate(cert),
		"apiserver.key": keyPEM,
	}, nil
}

//go:embed bashible-apiserver/manifests/version_map.yml
var bashibleVersionMap string

func (r *reconciler) reconcileBashibleFilesConfigMap(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	bashibleImagesDigestsJSON, err := r.getImagesDigestsJSON(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get images digests JSON: %w", err)
	}

	target := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleFilesConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"version_map.yml":     bashibleVersionMap,
			"images_digests.json": bashibleImagesDigestsJSON,
		},
	}

	current, err := r.getConfigMap(ctx, namespace, bashibleFilesConfigMapName)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createConfigMap(ctx, target)
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	if equality.Semantic.DeepEqual(current.Data, target.Data) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	current.Data = target.Data
	return reconcile.Result{}, r.patchConfigMap(ctx, base, current)
}

func (r *reconciler) getImagesDigestsJSON(ctx context.Context) (string, error) {
	configMap, err := r.getConfigMap(ctx, bashibleDeckhouseNamespace, bashibleFilesConfigMapName)
	if err != nil {
		return "", fmt.Errorf("get images digests JSON: %w", err)
	}

	if configMap.Data["images_digests.json"] == "" {
		return "", fmt.Errorf("images digests JSON is empty")
	}

	return configMap.Data["images_digests.json"], nil
}

func (r *reconciler) reconcileBashibleService(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (*corev1.Service, reconcile.Result, error) {
	target := buildTargetBashibleService(vcp)

	current, err := r.getService(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		if err := r.createService(ctx, target); err != nil {
			return nil, reconcile.Result{}, fmt.Errorf("create bashible Service: %w", err)
		}
		return target, reconcile.Result{}, nil
	}
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("get bashible Service: %w", err)
	}

	if isBashibleServiceInSync(current, target) {
		return current, reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	applyBashibleServiceTarget(current, target)

	if err := r.patchService(ctx, base, current); err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("patch bashible Service: %w", err)
	}

	return current, reconcile.Result{}, nil
}

func buildTargetBashibleService(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Service {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleServiceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                      bashibleAppLabel,
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": bashibleAppLabel,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt32(bashibleSecurePort),
				},
			},
		},
	}
}

func isBashibleServiceInSync(current, target *corev1.Service) bool {
	for key, value := range target.Labels {
		if current.Labels[key] != value {
			return false
		}
	}

	return current.Spec.Type == target.Spec.Type &&
		equality.Semantic.DeepEqual(current.Spec.Selector, target.Spec.Selector) &&
		equality.Semantic.DeepEqual(current.Spec.Ports, target.Spec.Ports)
}

func applyBashibleServiceTarget(current, target *corev1.Service) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}

	maps.Copy(current.Labels, target.Labels)

	current.Spec.Type = target.Spec.Type
	current.Spec.Selector = target.Spec.Selector
	current.Spec.Ports = target.Spec.Ports
}

func (r *reconciler) reconcileBashibleDeployment(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (reconcile.Result, error) {
	image, err := r.getBashibleApiserverImage(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get bashible apiserver image: %w", err)
	}

	target, err := buildTargetBashibleDeployment(vcp, image)
	if err != nil {
		return reconcile.Result{}, err
	}

	current, err := r.getDeployment(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createDeployment(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get bashible Deployment: %w", err)
	}

	if equality.Semantic.DeepEqual(current.Spec, target.Spec) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	current.Spec = target.Spec
	return reconcile.Result{}, r.patchDeployment(ctx, base, current)
}

func (r *reconciler) getBashibleApiserverImage(ctx context.Context) (string, error) {
	global, err := r.getSecret(ctx, constants.KubeSystemNamespace, constants.VirtualControlPlaneConfigSecretName)
	if err != nil {
		return "", err
	}

	return bashibleApiserverImageFromConfig(global)
}

//go:embed bashible-apiserver/manifests/deployment.yaml
var bashibleDeploymentYAML string

func buildTargetBashibleDeployment(vcp *controlplanev1alpha1.VirtualControlPlane, image string) (*appsv1.Deployment, error) {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	rendered := strings.NewReplacer(
		"${NAMESPACE}", namespace,
		"${IMAGE_BASHIBLE_APISERVER}", image,
	).Replace(bashibleDeploymentYAML)

	deployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(rendered), deployment); err != nil {
		return nil, fmt.Errorf("unmarshal bashible Deployment: %w", err)
	}

	return deployment, nil
}

func (r *reconciler) reconcileBashibleAPIService(
	ctx context.Context,
	nested client.Client,
	tlsSecret *corev1.Secret,
	albVIP string,
) (reconcile.Result, error) {
	namespace := bashibleDeckhouseNamespace
	bashibleAddress := albVIP
	if bashibleAddress == "" {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	svc := buildNestedBashibleService(namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, nested, svc, func() error {
		applyNestedBashibleService(svc)
		return nil
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("nested bashible service: %w", err)
	}

	// Legacy Endpoints for kube-aggregator (< 1.34)
	ep := buildNestedBashibleEndpoints(namespace, bashibleAddress)
	if _, err := controllerutil.CreateOrUpdate(ctx, nested, ep, func() error {
		applyNestedBashibleEndpoints(ep, namespace, bashibleAddress)
		return nil
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("nested bashible endpoints: %w", err)
	}

	// es := buildNestedBashibleEndpointSlice(namespace, bashibleAddress)
	// if _, err := controllerutil.CreateOrUpdate(ctx, nested, es, func() error {
	// 	applyNestedBashibleEndpointSlice(es, namespace, bashibleAddress)
	// 	return nil
	// }); err != nil {
	// 	return reconcile.Result{}, fmt.Errorf("nested bashible endpointslice: %w", err)
	// }

	caBundle := tlsSecret.Data["ca.crt"]
	apiservice := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: bashibleAPIServiceName},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:                bashibleAPIGroup,
			Version:              bashibleAPIVersion,
			GroupPriorityMinimum: 1000,
			VersionPriority:      15,
			Service: &apiregistrationv1.ServiceReference{
				Name:      bashibleServiceName,
				Namespace: namespace,
			},
			CABundle: caBundle,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, nested, apiservice, func() error {
		apiservice.Spec.CABundle = caBundle
		apiservice.Spec.Service = &apiregistrationv1.ServiceReference{
			Name: bashibleServiceName, Namespace: namespace,
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("nested APIService: %w", err)
	}

	return reconcile.Result{}, nil
}

func buildNestedBashibleService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleServiceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolTCP,
			}},
		},
	}
}

func applyNestedBashibleService(svc *corev1.Service) {
	svc.Spec.Selector = nil
	svc.Spec.Ports = []corev1.ServicePort{{
		Name:     "https",
		Port:     bashibleNestedServicePort,
		Protocol: corev1.ProtocolTCP,
	}}
}

func buildNestedBashibleEndpoints(namespace, address string) *corev1.Endpoints {
	ep := &corev1.Endpoints{}
	applyNestedBashibleEndpoints(ep, namespace, address)
	ep.Name = bashibleServiceName
	return ep
}

func applyNestedBashibleEndpoints(ep *corev1.Endpoints, namespace, address string) {
	ep.Namespace = namespace
	ep.Subsets = []corev1.EndpointSubset{{
		Addresses: []corev1.EndpointAddress{{IP: address}},
		Ports: []corev1.EndpointPort{{
			Name:     "https",
			Port:     bashibleNestedServicePort,
			Protocol: corev1.ProtocolTCP,
		}},
	}}
}

func buildNestedBashibleEndpointSlice(namespace, bashibleAddress string) *discoveryv1.EndpointSlice {
	slice := &discoveryv1.EndpointSlice{}
	applyNestedBashibleEndpointSlice(slice, namespace, bashibleAddress)
	slice.Name = bashibleEndpointSliceName
	return slice
}

func applyNestedBashibleEndpointSlice(slice *discoveryv1.EndpointSlice, namespace, bashibleAddress string) {
	portName := "https"
	protocol := corev1.ProtocolTCP
	port := int32(bashibleNestedServicePort)

	if slice.Labels == nil {
		slice.Labels = map[string]string{}
	}
	slice.Namespace = namespace
	slice.Labels[discoveryv1.LabelServiceName] = bashibleServiceName

	slice.AddressType = discoveryv1.AddressTypeIPv4
	slice.Endpoints = []discoveryv1.Endpoint{{
		Addresses: []string{bashibleAddress},
	}}
	slice.Ports = []discoveryv1.EndpointPort{{
		Name:     &portName,
		Protocol: &protocol,
		Port:     &port,
	}}
}
