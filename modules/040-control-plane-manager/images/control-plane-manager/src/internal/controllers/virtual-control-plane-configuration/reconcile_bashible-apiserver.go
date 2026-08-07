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
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	certutil "k8s.io/client-go/util/cert"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	bashibleContextSecretName  = "bashible-apiserver-context"
	bashibleContextSecretKey   = "input.yaml"
	bashibleRegistrySecretName = "deckhouse-registry"
	bashibleFilesConfigMapName = "bashible-apiserver-files"
	bashibleTLSSecretName      = "bashible-apiserver-tls"

	bashibleAPIServiceName = "v1alpha1.bashible.deckhouse.io"
	bashibleAPIGroup       = "bashible.deckhouse.io"
	bashibleAPIVersion     = "v1alpha1"

	bashibleFirstRunFinishedLabel = "node.deckhouse.io/bashible-first-run-finished"
	bashibleUninitializedTaintKey = "node.deckhouse.io/bashible-uninitialized"
	nodeUninitializedTaintKey     = "node.deckhouse.io/uninitialized"
)

func (r *reconciler) reconcileBashibleApiserver(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
	pkiSecret *corev1.Secret,
	adminSecret *corev1.Secret,
	joinToken string,
) (reconcile.Result, error) {
	// 2. Build a nested client that provides access to the nested cluster.
	nestedClient, err := bashibleapiserver.BuildNestedClient(adminSecret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build nested client: %w", err)
	}

	// 3. Nested: wait until the tenant node-manager module has installed the CRDs
	//    bashible-apiserver serves from.
	if res, err := r.waitForBashibleApiserverCRDs(ctx, nestedClient); err != nil || !res.IsZero() {
		return res, err
	}

	// 4. Nested: external inputs the tenant node-manager renders the context Secret from.
	if res, err := r.reconcileBashibleExternalInputs(ctx, nestedClient, vcp, pkiSecret, joinToken, configSecret); err != nil || !res.IsZero() {
		return res, err
	}

	// 5. Parent: TLS
	tlsSecret, res, err := r.reconcileBashibleTLSSecret(ctx, vcp, pkiSecret)
	if err != nil || !res.IsZero() {
		return res, err
	}

	// 6. Parent: Files ConfigMap
	if res, err := r.reconcileBashibleFilesConfigMap(ctx, vcp); err != nil || !res.IsZero() {
		return res, err
	}

	// 7. Parent: Service
	parentService, res, err := r.reconcileBashibleService(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}

	// 8. Parent: Deployment
	if res, err := r.reconcileBashibleDeployment(ctx, vcp); err != nil || !res.IsZero() {
		return res, err
	}

	// 9. Nested: APIService
	if res, err := r.reconcileBashibleAPIService(ctx, nestedClient, parentService, tlsSecret); err != nil || !res.IsZero() {
		return res, err
	}

	// 10. Nested: node cleanup (no node-manager runs in the nested cluster to do it).
	if res, err := r.reconcileNestedNodeCleanup(ctx, nestedClient); err != nil || !res.IsZero() {
		return res, err
	}

	return reconcile.Result{}, nil
}

var bashibleApiserverCRDs = []string{
	"nodegroupconfigurations.deckhouse.io",
	"nodeusers.deckhouse.io",
}

func (r *reconciler) waitForBashibleApiserverCRDs(ctx context.Context, nestedClient client.Client) (reconcile.Result, error) {
	for _, name := range bashibleApiserverCRDs {
		crd := &unstructured.Unstructured{}
		crd.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "apiextensions.k8s.io",
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		})

		err := nestedClient.Get(ctx, client.ObjectKey{Name: name}, crd)
		if apierrors.IsNotFound(err) {
			log.FromContext(ctx).Info("waiting for the tenant node-manager to install bashible CRDs", "crd", name)
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("get nested CRD %s: %w", name, err)
		}
	}

	return reconcile.Result{}, nil
}

// reconcileBashibleExternalInputs publishes the facts a tenant cannot derive from its own
// cluster into d8-cloud-instance-manager/bashible-external-inputs. The tenant node-manager reads
// it, overlays it on the context it assembles itself and renders bashible-apiserver-context from
// a template: this controller no longer writes that Secret.
func (r *reconciler) reconcileBashibleExternalInputs(
	ctx context.Context,
	nestedClient client.Client,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
	joinToken string,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	publishedInputs, err := getNestedSecret(ctx, nestedClient, bashibleapiserver.ExternalInputsSecretName)
	if err != nil {
		return reconcile.Result{}, err
	}

	publishedContext, err := getNestedSecret(ctx, nestedClient, bashibleContextSecretName)
	if err != nil {
		return reconcile.Result{}, err
	}

	proxyCerts, err := resolveBashibleAPIServerProxyCerts(pkiSecret, publishedInputs, publishedContext)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("resolve apiserverProxyCerts: %w", err)
	}

	rppToken, err := r.registryPackagesProxyToken(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get registry-packages-proxy token: %w", err)
	}

	inputsYAML, err := bashibleapiserver.BuildExternalInputsYAML(bashibleapiserver.ExternalInputsParams{
		VCP:                 vcp,
		CA:                  pkiSecret.Data["ca.crt"],
		JoinToken:           joinToken,
		ClusterUUID:         string(configSecret.Data["cluster-uuid"]),
		APIHost:             apiExposeHost(vcp),
		PackagesHost:        packagesExposeHost(vcp),
		RPPToken:            rppToken,
		APIServerProxyCerts: proxyCerts,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build bashible external inputs: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bashibleapiserver.ExternalInputsSecretName,
			Namespace: bashibleDeckhouseNamespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, nestedClient, secret, func() error {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[constants.HeritageLabelKey] = constants.HeritageLabelValue
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		secret.Annotations[bashibleapiserver.ExternalInputsRevisionAnnotation] = bashibleapiserver.ExternalInputsRevision(inputsYAML)
		secret.Type = corev1.SecretTypeOpaque
		secret.StringData = nil
		secret.Data = map[string][]byte{
			bashibleapiserver.ExternalInputsSecretKey: []byte(inputsYAML),
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("upsert %s/%s: %w", bashibleDeckhouseNamespace, bashibleapiserver.ExternalInputsSecretName, err)
	}

	return reconcile.Result{}, nil
}

func getNestedSecret(ctx context.Context, nestedClient client.Client, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := nestedClient.Get(ctx, client.ObjectKey{Name: name, Namespace: bashibleDeckhouseNamespace}, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get nested Secret %s/%s: %w", bashibleDeckhouseNamespace, name, err)
	}

	return secret, nil
}

// resolveBashibleAPIServerProxyCerts reuses the certificates already handed to the tenant instead
// of signing new ones: every node holds this pair in its api-proxy configuration, so a reissue
// silently invalidates the fleet. bashible-external-inputs is this controller's own object and
// the authoritative source; bashible-apiserver-context is read only to adopt the pair issued
// before the tenant became the writer of that Secret, and stops mattering once the inputs exist.
func resolveBashibleAPIServerProxyCerts(
	pkiSecret *corev1.Secret,
	inputsSecret *corev1.Secret,
	contextSecret *corev1.Secret,
) (bashibleapiserver.ContextAPIServerProxyCerts, error) {
	if certs, ok := publishedAPIServerProxyCerts(inputsSecret, bashibleapiserver.ExternalInputsSecretKey); ok {
		return certs, nil
	}
	if certs, ok := publishedAPIServerProxyCerts(contextSecret, bashibleContextSecretKey); ok {
		return certs, nil
	}

	return generateBashibleAPIServerProxyCerts(pkiSecret)
}

func publishedAPIServerProxyCerts(secret *corev1.Secret, key string) (bashibleapiserver.ContextAPIServerProxyCerts, bool) {
	if secret == nil || len(secret.Data[key]) == 0 {
		return bashibleapiserver.ContextAPIServerProxyCerts{}, false
	}

	var published struct {
		APIServerProxyCerts bashibleapiserver.ContextAPIServerProxyCerts `json:"apiserverProxyCerts"`
	}
	if err := yaml.Unmarshal(secret.Data[key], &published); err != nil ||
		published.APIServerProxyCerts.Crt == "" || published.APIServerProxyCerts.Key == "" {
		return bashibleapiserver.ContextAPIServerProxyCerts{}, false
	}

	return published.APIServerProxyCerts, true
}

// signVCPCert signs a leaf certificate with the VCP cluster CA from pkiSecret
// and returns the certificate and private key in PEM form.
func signVCPCert(pkiSecret *corev1.Secret, cfg pkiutil.CertConfig) (crtPEM, keyPEM []byte, err error) {
	caCert, err := pkiutil.ParseCertificatePEM(pkiSecret.Data["ca.crt"])
	if err != nil {
		return nil, nil, fmt.Errorf("parse VCP CA cert: %w", err)
	}
	caKey, err := pkiutil.ParsePrivateKeyPEM(pkiSecret.Data["ca.key"])
	if err != nil {
		return nil, nil, fmt.Errorf("parse VCP CA key: %w", err)
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("sign %q cert: %w", cfg.CommonName, err)
	}

	keyPEM, err = pkiutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %q key: %w", cfg.CommonName, err)
	}

	return pkiutil.EncodeCertificate(cert), keyPEM, nil
}

func generateBashibleAPIServerProxyCerts(pkiSecret *corev1.Secret) (bashibleapiserver.ContextAPIServerProxyCerts, error) {
	cfg := pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   "kubernetes-api-proxy",
			Organization: []string{"node-manager:kubernetes-api-proxy"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		NotAfter:            time.Now().AddDate(10, 0, 0),
		EncryptionAlgorithm: pkiconstants.EncryptionAlgorithmRSA2048,
	}

	crtPEM, keyPEM, err := signVCPCert(pkiSecret, cfg)
	if err != nil {
		return bashibleapiserver.ContextAPIServerProxyCerts{}, err
	}

	return bashibleapiserver.ContextAPIServerProxyCerts{
		Crt: string(crtPEM),
		Key: string(keyPEM),
	}, nil
}

func (r *reconciler) reconcileBashibleTLSSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
) (*corev1.Secret, reconcile.Result, error) {
	target := buildTargetBashibleTLSSecret(vcp)
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return nil, reconcile.Result{}, err
	}

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

	if !ownerReferencesDiffer(current, target) &&
		equality.Semantic.DeepEqual(current.Labels, target.Labels) {
		return current, reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	current.Labels = target.Labels
	syncOwnerReferences(current, target)

	return current, reconcile.Result{}, r.patchSecret(ctx, base, current)
}

func buildTargetBashibleTLSSecret(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualResourceName(bashibleTLSSecretName, vcp.Name),
			Namespace: vcp.Namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey:                 constants.HeritageLabelValue,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func buildBashibleTLSSecretData(pkiSecret *corev1.Secret) (map[string][]byte, error) {
	namespace := bashibleDeckhouseNamespace

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

	crtPEM, keyPEM, err := signVCPCert(pkiSecret, cfg)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"ca.crt":        pkiSecret.Data["ca.crt"],
		"apiserver.crt": crtPEM,
		"apiserver.key": keyPEM,
	}, nil
}

//go:embed bashible-apiserver/manifests/version_map.yml
var bashibleVersionMap string

func (r *reconciler) reconcileBashibleFilesConfigMap(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	versionMap, imagesDigestsJSON, err := r.getBashibleFiles(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	target := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualResourceName(bashibleFilesConfigMapName, vcp.Name),
			Namespace: vcp.Namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey:                 constants.HeritageLabelValue,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
			},
		},
		Data: map[string]string{
			"version_map.yml":     versionMap,
			"images_digests.json": imagesDigestsJSON,
		},
	}
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	current, err := r.getConfigMap(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createConfigMap(ctx, target)
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	if equality.Semantic.DeepEqual(current.Data, target.Data) &&
		equality.Semantic.DeepEqual(current.Labels, target.Labels) &&
		!ownerReferencesDiffer(current, target) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	current.Data = target.Data
	current.Labels = target.Labels
	syncOwnerReferences(current, target)
	return reconcile.Result{}, r.patchConfigMap(ctx, base, current)
}

// getBashibleFiles reads version_map.yml and images_digests.json from the parent bashible-apiserver-files configmap for consistency on bashible (for images and versions)
func (r *reconciler) getBashibleFiles(ctx context.Context) (versionMap string, imagesDigestsJSON string, err error) {
	configMap, err := r.getConfigMap(ctx, bashibleDeckhouseNamespace, bashibleFilesConfigMapName)
	if err != nil {
		return "", "", fmt.Errorf("get bashible-apiserver-files configmap: %w", err)
	}

	imagesDigestsJSON = configMap.Data["images_digests.json"]
	if imagesDigestsJSON == "" {
		return "", "", fmt.Errorf("images digests JSON is empty")
	}

	versionMap = configMap.Data["version_map.yml"]
	if versionMap == "" {
		versionMap = bashibleVersionMap
	}

	return versionMap, imagesDigestsJSON, nil
}

func (r *reconciler) reconcileBashibleService(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (*corev1.Service, reconcile.Result, error) {
	target := buildTargetBashibleService(vcp)
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return nil, reconcile.Result{}, err
	}

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
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualResourceName(bashibleServiceName, vcp.Name),
			Namespace: vcp.Namespace,
			Labels: map[string]string{
				"app":                      bashibleAppLabel,
				constants.HeritageLabelKey: constants.HeritageLabelValue,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": bashibleAppLabel,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
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
		equality.Semantic.DeepEqual(current.Spec.Ports, target.Spec.Ports) &&
		equality.Semantic.DeepEqual(current.OwnerReferences, target.OwnerReferences)
}

func applyBashibleServiceTarget(current, target *corev1.Service) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}

	maps.Copy(current.Labels, target.Labels)

	current.Spec.Type = target.Spec.Type
	current.Spec.Selector = target.Spec.Selector
	current.Spec.Ports = target.Spec.Ports
	current.OwnerReferences = target.OwnerReferences
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
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	current, err := r.getDeployment(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createDeployment(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get bashible Deployment: %w", err)
	}

	if isBashibleDeploymentInSync(current, target) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	applyBashibleDeploymentTarget(current, target)
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
	rendered := strings.NewReplacer(
		"${NAMESPACE}", vcp.Namespace,
		"${IMAGE_BASHIBLE_APISERVER}", image,
	).Replace(bashibleDeploymentYAML)

	deployment := &appsv1.Deployment{}
	if err := yaml.Unmarshal([]byte(rendered), deployment); err != nil {
		return nil, fmt.Errorf("unmarshal bashible Deployment: %w", err)
	}

	tlsSecret := constants.VirtualResourceName(bashibleTLSSecretName, vcp.Name)
	filesCM := constants.VirtualResourceName(bashibleFilesConfigMapName, vcp.Name)
	kubeconfigSecret := constants.VirtualResourceName(constants.VirtualClientsKubeconfigSecretName, vcp.Name)
	registrySecret := constants.VirtualResourceName(bashibleRegistrySecretName, vcp.Name)

	deployment.Name = constants.VirtualResourceName(bashibleDeploymentName, vcp.Name)
	if deployment.Labels == nil {
		deployment.Labels = map[string]string{}
	}
	deployment.Labels[constants.VirtualControlPlaneScopeLabelKey] = vcp.Name

	if deployment.Spec.Selector == nil {
		deployment.Spec.Selector = &metav1.LabelSelector{}
	}
	if deployment.Spec.Selector.MatchLabels == nil {
		deployment.Spec.Selector.MatchLabels = map[string]string{}
	}
	deployment.Spec.Selector.MatchLabels[constants.VirtualControlPlaneScopeLabelKey] = vcp.Name

	if deployment.Spec.Template.Labels == nil {
		deployment.Spec.Template.Labels = map[string]string{}
	}
	deployment.Spec.Template.Labels[constants.VirtualControlPlaneScopeLabelKey] = vcp.Name

	for i := range deployment.Spec.Template.Spec.ImagePullSecrets {
		if deployment.Spec.Template.Spec.ImagePullSecrets[i].Name == bashibleRegistrySecretName {
			deployment.Spec.Template.Spec.ImagePullSecrets[i].Name = registrySecret
		}
	}

	for i := range deployment.Spec.Template.Spec.Volumes {
		vol := &deployment.Spec.Template.Spec.Volumes[i]
		if vol.Secret != nil {
			switch vol.Secret.SecretName {
			case bashibleTLSSecretName:
				vol.Secret.SecretName = tlsSecret
			case constants.VirtualClientsKubeconfigSecretName:
				vol.Secret.SecretName = kubeconfigSecret
			}
		}
		if vol.ConfigMap != nil && vol.ConfigMap.Name == bashibleFilesConfigMapName {
			vol.ConfigMap.Name = filesCM
		}
	}

	return deployment, nil
}

func isBashibleDeploymentInSync(current, target *appsv1.Deployment) bool {
	for key, value := range target.Labels {
		if current.Labels[key] != value {
			return false
		}
	}
	return equality.Semantic.DeepEqual(current.Spec.Replicas, target.Spec.Replicas) &&
		equality.Semantic.DeepEqual(current.Spec.Selector, target.Spec.Selector) &&
		equality.Semantic.DeepDerivative(target.Spec.Template, current.Spec.Template) &&
		equality.Semantic.DeepEqual(current.OwnerReferences, target.OwnerReferences)
}

func applyBashibleDeploymentTarget(current, target *appsv1.Deployment) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	maps.Copy(current.Labels, target.Labels)
	current.Spec.Replicas = target.Spec.Replicas
	current.Spec.Selector = target.Spec.Selector
	current.Spec.Template = target.Spec.Template
	current.OwnerReferences = target.OwnerReferences
}

func (r *reconciler) reconcileBashibleAPIService(
	ctx context.Context,
	nested client.Client,
	parentService *corev1.Service,
	tlsSecret *corev1.Secret,
) (reconcile.Result, error) {
	namespace := bashibleDeckhouseNamespace
	address := parentService.Spec.ClusterIP
	if address == "" || address == corev1.ClusterIPNone {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	nestedService := buildNestedBashibleService(namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, nested, nestedService, func() error {
		applyNestedBashibleService(nestedService)
		return nil
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("nested bashible service: %w", err)
	}

	// Legacy Endpoints for kube-aggregator (< 1.34)
	ep := buildNestedBashibleEndpoints(namespace, address)
	if _, err := controllerutil.CreateOrUpdate(ctx, nested, ep, func() error {
		// TODO: migrate to endpointslice
		applyNestedBashibleEndpoints(ep, namespace, address)
		return nil
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("nested bashible endpoints: %w", err)
	}

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

// reconcileNestedNodeCleanup ports node-controller bashiblecleanup for the nested cluster
// once a node reports bashible-first-run-finished, drop that label and the uninitialized taints in one patch so it becomes schedulable and bashible does not re-apply the label.
func (r *reconciler) reconcileNestedNodeCleanup(ctx context.Context, nestedClient client.Client) (reconcile.Result, error) {
	nodes := &corev1.NodeList{}
	if err := nestedClient.List(ctx, nodes, client.HasLabels{bashibleFirstRunFinishedLabel}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list nested nodes: %w", err)
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		base := node.DeepCopy()

		delete(node.Labels, bashibleFirstRunFinishedLabel)
		node.Spec.Taints = filterTaints(node.Spec.Taints, nodeUninitializedTaintKey, bashibleUninitializedTaintKey)

		if equality.Semantic.DeepEqual(base.Labels, node.Labels) &&
			equality.Semantic.DeepEqual(base.Spec.Taints, node.Spec.Taints) {
			continue
		}
		if err := nestedClient.Patch(ctx, node, client.MergeFrom(base)); err != nil {
			return reconcile.Result{}, fmt.Errorf("cleanup nested node %s: %w", node.Name, err)
		}
	}

	return reconcile.Result{}, nil
}

func filterTaints(taints []corev1.Taint, dropKeys ...string) []corev1.Taint {
	drop := make(map[string]struct{}, len(dropKeys))
	for _, k := range dropKeys {
		drop[k] = struct{}{}
	}
	out := make([]corev1.Taint, 0, len(taints))
	for _, t := range taints {
		if _, ok := drop[t.Key]; !ok {
			out = append(out, t)
		}
	}
	return out
}
