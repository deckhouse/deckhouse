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
	"maps"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	requeueInterval                   = 5 * time.Minute
	requeueIntervalOnReadingClusterIP = 5 * time.Second
)

var _ reconcile.Reconciler = (*reconciler)(nil)

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	vcp, err := r.getVirtualControlPlane(ctx, req.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get VirtualControlPlane: %w", err)
	}

	if res, err := r.reconcileNamespace(ctx, vcp); err != nil || !res.IsZero() {
		return res, err
	}

	apiserverService, res, err := r.reconcileAPIServerService(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}

	pkiSecret, res, err := r.reconcilePKISecret(ctx, vcp, apiserverService)
	if err != nil || !res.IsZero() {
		return res, err
	}

	if res, err := r.reconcileKubeconfigSecret(ctx, vcp, apiserverService, pkiSecret); err != nil || !res.IsZero() {
		return res, err
	}

	configSecret, res, err := r.reconcileConfigSecret(ctx)
	if err != nil || !res.IsZero() {
		return res, err
	}

	if res, err := r.reconcileControlPlaneNodes(ctx, vcp, pkiSecret, configSecret); err != nil || !res.IsZero() {
		return res, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *reconciler) reconcileNamespace(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	target := buildTargetNamespace(vcp)

	current, err := r.getNamespace(ctx, target.Name)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, r.createNamespace(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get Namespace: %w", err)
	}

	if isNamespaceInSync(current, target) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	applyNamespaceTarget(current, target)

	return reconcile.Result{}, r.patchNamespace(ctx, base, current)
}

func buildTargetNamespace(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.VirtualControlPlaneNamespacePrefix + vcp.Name,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
	}
}

func isNamespaceInSync(current, target *corev1.Namespace) bool {
	for key, value := range target.Labels {
		if current.Labels[key] != value {
			return false
		}
	}
	return true
}

func applyNamespaceTarget(current, target *corev1.Namespace) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}

	maps.Copy(current.Labels, target.Labels)
}

func (r *reconciler) reconcileAPIServerService(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (*corev1.Service, reconcile.Result, error) {
	target := buildTargetAPIServerService(vcp)

	current, err := r.getService(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		if err := r.createService(ctx, target); err != nil {
			return nil, reconcile.Result{}, err
		}

		return nil, reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("get apiserver Service: %w", err)
	}

	if current.Spec.ClusterIP == "" || current.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil, reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	if isAPIServerServiceInSync(current, target) {
		return current, reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	applyAPIServerServiceTarget(current, target)

	return current, reconcile.Result{}, r.patchService(ctx, base, current)
}

func buildTargetAPIServerService(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Service {
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "kube-apiserver",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       6443,
					TargetPort: intstr.FromInt(6443),
				},
			},
		},
	}
}

func isAPIServerServiceInSync(current, target *corev1.Service) bool {
	for key, value := range target.Labels {
		if current.Labels[key] != value {
			return false
		}
	}

	return current.Spec.Type == target.Spec.Type &&
		equality.Semantic.DeepEqual(current.Spec.Selector, target.Spec.Selector) &&
		equality.Semantic.DeepEqual(current.Spec.Ports, target.Spec.Ports)
}

func applyAPIServerServiceTarget(current, target *corev1.Service) {
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}

	for key, value := range target.Labels {
		current.Labels[key] = value
	}

	current.Spec.Type = target.Spec.Type
	current.Spec.Selector = target.Spec.Selector
	current.Spec.Ports = target.Spec.Ports
}

func (r *reconciler) reconcilePKISecret(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane, apiserverService *corev1.Service) (*corev1.Secret, reconcile.Result, error) {
	target := buildTargetPKISecret(vcp)
	current, err := r.getSecret(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		data, err := buildTargetPKISecretData(vcp, apiserverService)
		if err != nil {
			return nil, reconcile.Result{}, fmt.Errorf("generate PKI Secret data: %w", err)
		}
		target.Data = data

		if err := r.createSecret(ctx, target); err != nil {
			return nil, reconcile.Result{}, err
		}

		return target, reconcile.Result{}, nil
	}
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("get PKI Secret: %w", err)
	}

	return current, reconcile.Result{}, nil
}

func buildTargetPKISecret(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Secret {
	name := constants.VirtualControlPlaneNamespacePrefix + vcp.Name + "-pki"
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func buildTargetPKISecretData(vcp *controlplanev1alpha1.VirtualControlPlane, apiserverService *corev1.Service) (map[string][]byte, error) {
	advertiseAddress := net.ParseIP(apiserverService.Spec.ClusterIP)
	if advertiseAddress == nil {
		return nil, fmt.Errorf("invalid apiserver Service ClusterIP: %q", apiserverService.Spec.ClusterIP)
	}

	pkiDir, err := os.MkdirTemp("", "vcp-pki-*")
	if err != nil {
		return nil, fmt.Errorf("create temp PKI dir: %w", err)
	}
	defer os.RemoveAll(pkiDir)

	nodeName := constants.VirtualControlPlaneNamespacePrefix + vcp.Name
	if _, err := pki.CreatePKIBundle(
		nodeName,
		constants.DefaultTenantClusterDomain,
		advertiseAddress,
		constants.DefaultTenantServiceSubnetCIDR,
		pki.WithPKIDir(pkiDir),
	); err != nil {
		return nil, fmt.Errorf("create PKI bundle: %w", err)
	}

	return readPKIBundleSecretData(pkiDir)
}

func readPKIBundleSecretData(pkiDir string) (map[string][]byte, error) {
	files := map[string]string{
		"ca.crt":                       "ca.crt",
		"ca.key":                       "ca.key",
		"apiserver.crt":                "apiserver.crt",
		"apiserver.key":                "apiserver.key",
		"apiserver-kubelet-client.crt": "apiserver-kubelet-client.crt",
		"apiserver-kubelet-client.key": "apiserver-kubelet-client.key",
		"front-proxy-ca.crt":           "front-proxy-ca.crt",
		"front-proxy-ca.key":           "front-proxy-ca.key",
		"front-proxy-client.crt":       "front-proxy-client.crt",
		"front-proxy-client.key":       "front-proxy-client.key",
		"etcd-ca.crt":                  "etcd/ca.crt",
		"etcd-ca.key":                  "etcd/ca.key",
		"etcd-server.crt":              "etcd/server.crt",
		"etcd-server.key":              "etcd/server.key",
		"etcd-peer.crt":                "etcd/peer.crt",
		"etcd-peer.key":                "etcd/peer.key",
		"etcd-healthcheck-client.crt":  "etcd/healthcheck-client.crt", // TODO: возможно откажемся
		"etcd-healthcheck-client.key":  "etcd/healthcheck-client.key", // TODO: возможно откажемся
		"apiserver-etcd-client.crt":    "apiserver-etcd-client.crt",
		"apiserver-etcd-client.key":    "apiserver-etcd-client.key",
		"sa.key":                       "sa.key",
		"sa.pub":                       "sa.pub",
	}

	data := make(map[string][]byte, len(files))
	for secretKey, relPath := range files {
		content, err := os.ReadFile(filepath.Join(pkiDir, relPath))
		if err != nil {
			return nil, fmt.Errorf("read PKI file %s: %w", relPath, err)
		}
		data[secretKey] = content
	}

	return data, nil
}

func (r *reconciler) reconcileKubeconfigSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	apiserverService *corev1.Service,
	pkiSecret *corev1.Secret,
) (reconcile.Result, error) {
	target := buildTargetKubeconfigSecret(vcp)

	_, err := r.getSecret(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		data, err := buildTargetKubeconfigSecretData(apiserverService, pkiSecret)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("generate kubeconfig Secret data: %w", err)
		}
		target.Data = data

		return reconcile.Result{}, r.createSecret(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get kubeconfig Secret: %w", err)
	}

	return reconcile.Result{}, nil
}

func buildTargetKubeconfigSecret(vcp *controlplanev1alpha1.VirtualControlPlane) *corev1.Secret {
	name := constants.VirtualControlPlaneNamespacePrefix + vcp.Name + "-kubeconfig"
	namespace := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey: constants.HeritageLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

var kubeconfigFiles = []kubeconfig.File{kubeconfig.ControllerManager, kubeconfig.Scheduler}

func buildTargetKubeconfigSecretData(apiserverService *corev1.Service, pkiSecret *corev1.Secret) (map[string][]byte, error) {
	clusterIP := apiserverService.Spec.ClusterIP
	if clusterIP == "" || clusterIP == corev1.ClusterIPNone {
		return nil, fmt.Errorf("apiserver Service has no ClusterIP")
	}

	caDir, err := os.MkdirTemp("", "vcp-kubeconfig-ca-*")
	if err != nil {
		return nil, fmt.Errorf("create temp CA dir: %w", err)
	}
	defer os.RemoveAll(caDir)

	if err := writeKubeconfigCA(caDir, pkiSecret.Data); err != nil {
		return nil, err
	}

	outDir, err := os.MkdirTemp("", "vcp-kubeconfig-out-*")
	if err != nil {
		return nil, fmt.Errorf("create temp out dir: %w", err)
	}
	defer os.RemoveAll(outDir)

	if _, err := kubeconfig.CreateKubeconfigFiles(kubeconfigFiles,
		kubeconfig.WithCertificatesDir(caDir),
		kubeconfig.WithOutDir(outDir),
		kubeconfig.WithLocalAPIEndpoint(clusterIP),
	); err != nil {
		return nil, fmt.Errorf("create kubeconfig files: %w", err)
	}

	return readKubeconfigSecretData(outDir)
}

func writeKubeconfigCA(dir string, pkiData map[string][]byte) error {
	for _, name := range []string{"ca.crt", "ca.key"} {
		content, ok := pkiData[name]
		if !ok {
			return fmt.Errorf("pki secret missing %q", name)
		}

		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}

func readKubeconfigSecretData(outDir string) (map[string][]byte, error) {
	data := make(map[string][]byte, len(kubeconfigFiles))
	for _, file := range kubeconfigFiles {
		content, err := os.ReadFile(filepath.Join(outDir, string(file)))
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig %s: %w", file, err)
		}
		data[string(file)] = content
	}

	return data, nil
}

func (r *reconciler) reconcileConfigSecret(ctx context.Context) (*corev1.Secret, reconcile.Result, error) {
	secret, err := r.getSecret(ctx, constants.KubeSystemNamespace, constants.VirtualControlPlaneConfigSecretName)
	if apierrors.IsNotFound(err) {
		return nil, reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("get config Secret: %w", err)
	}

	return secret, reconcile.Result{}, nil
}

func (r *reconciler) reconcileControlPlaneNodes(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	targets, err := buildTargetControlPlaneNodes(vcp, configSecret, pkiSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	targetNames := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetNames[target.Name] = struct{}{}

		current, err := r.getControlPlaneNode(ctx, target.Namespace, target.Name)
		if apierrors.IsNotFound(err) {
			if err := r.createControlPlaneNode(ctx, target); err != nil {
				return reconcile.Result{}, err
			}
			continue
		}
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("get ControlPlaneNode: %w", err)
		}

		if isControlPlaneNodeInSync(current, target) {
			continue
		}

		base := current.DeepCopy()
		applyControlPlaneNodeTarget(current, target)

		if err := r.patchControlPlaneNode(ctx, base, current); err != nil {
			return reconcile.Result{}, err
		}
	}
	return r.reconcileStaleControlPlaneNodes(ctx, vcp, targetNames)
}

func buildTargetControlPlaneNodes(
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
	pkiSecret *corev1.Secret,
) ([]*controlplanev1alpha1.ControlPlaneNode, error) {
	spec, err := buildTargetControlPlaneNodeSpec(configSecret, pkiSecret)
	if err != nil {
		return nil, err
	}

	targets := make([]*controlplanev1alpha1.ControlPlaneNode, 0, vcp.Spec.Replicas)
	for ordinal := int32(0); ordinal < vcp.Spec.Replicas; ordinal++ {
		targets = append(targets, buildTargetControlPlaneNode(vcp, ordinal, spec))
	}

	return targets, nil
}

func buildTargetControlPlaneNodeSpec(
	configSecret *corev1.Secret,
	pkiSecret *corev1.Secret,
) (controlplanev1alpha1.ControlPlaneNodeSpec, error) {
	caChecksum, err := checksum.PKIChecksum(pkiSecret.Data)
	if err != nil {
		return controlplanev1alpha1.ControlPlaneNodeSpec{}, err
	}

	configChecksums := make(map[string]string)
	pkiChecksums := make(map[string]string)
	for _, component := range []string{
		"etcd",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
	} {
		configChecksum, err := checksum.ComponentChecksum(configSecret.Data, component)
		if err != nil {
			return controlplanev1alpha1.ControlPlaneNodeSpec{}, fmt.Errorf("calculate config checksum for %s: %w", component, err)
		}
		configChecksums[component] = configChecksum
		pkiChecksum, err := checksum.ComponentPKIChecksum(configSecret.Data, component)
		if err != nil {
			return controlplanev1alpha1.ControlPlaneNodeSpec{}, fmt.Errorf("calculate pki checksum for %s: %w", component, err)
		}
		pkiChecksums[component] = pkiChecksum
	}
	return controlplanev1alpha1.ControlPlaneNodeSpec{
		CAChecksum: caChecksum,
		Components: controlplanev1alpha1.ComponentsSpec{
			Etcd: controlplanev1alpha1.ComponentSpec{
				Checksums: controlplanev1alpha1.Checksums{
					Config: configChecksums["etcd"],
					PKI:    pkiChecksums["etcd"],
				},
			},
			KubeAPIServer: controlplanev1alpha1.ComponentSpec{
				Checksums: controlplanev1alpha1.Checksums{
					Config: configChecksums["kube-apiserver"],
					PKI:    pkiChecksums["kube-apiserver"],
				},
			},
			KubeControllerManager: controlplanev1alpha1.ComponentSpec{
				Checksums: controlplanev1alpha1.Checksums{
					Config: configChecksums["kube-controller-manager"],
				},
			},
			KubeScheduler: controlplanev1alpha1.ComponentSpec{
				Checksums: controlplanev1alpha1.Checksums{
					Config: configChecksums["kube-scheduler"],
				},
			},
		},
	}, nil
}

func buildTargetControlPlaneNode(
	vcp *controlplanev1alpha1.VirtualControlPlane,
	ordinal int32,
	spec controlplanev1alpha1.ControlPlaneNodeSpec,
) *controlplanev1alpha1.ControlPlaneNode {
	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      computeControlPlaneNodeName(vcp, ordinal),
			Namespace: constants.VirtualControlPlaneNamespacePrefix + vcp.Name,
			Labels: map[string]string{
				constants.HeritageLabelKey:                       constants.HeritageLabelValue,
				constants.ControlPlaneTypeLabelKey:               string(constants.ControlPlaneTypeVirtual),
				constants.VirtualControlPlaneNodeOrdinalLabelKey: fmt.Sprintf("%d", ordinal),
			},
		},
		Spec: spec,
	}
}

func computeControlPlaneNodeName(vcp *controlplanev1alpha1.VirtualControlPlane, ordinal int32) string {
	return fmt.Sprintf("%s%s-%d", constants.VirtualControlPlaneNamespacePrefix, vcp.Name, ordinal)
}

func isControlPlaneNodeInSync(current, target *controlplanev1alpha1.ControlPlaneNode) bool {
	return equality.Semantic.DeepEqual(current.Labels, target.Labels) &&
		equality.Semantic.DeepEqual(current.OwnerReferences, target.OwnerReferences) &&
		equality.Semantic.DeepEqual(current.Spec, target.Spec)
}

func applyControlPlaneNodeTarget(current, target *controlplanev1alpha1.ControlPlaneNode) {
	current.Labels = target.Labels
	current.OwnerReferences = target.OwnerReferences
	current.Spec = target.Spec
}

func (r *reconciler) reconcileStaleControlPlaneNodes(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	targetNames map[string]struct{},
) (reconcile.Result, error) {
	current, err := r.getControlPlaneNodesByVirtualControlPlane(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("list ControlPlaneNodes: %w", err)
	}

	for i := range current {
		cpn := &current[i]
		if _, ok := targetNames[cpn.Name]; ok {
			continue
		}

		if err := r.deleteControlPlaneNode(ctx, cpn); err != nil {
			return reconcile.Result{}, fmt.Errorf("delete stale ControlPlaneNode %s: %w", cpn.Name, err)
		}
	}

	return reconcile.Result{}, nil
}

func controlPlaneNodeOrdinal(cpn *controlplanev1alpha1.ControlPlaneNode) int32 {
	value := cpn.Labels[constants.VirtualControlPlaneNodeOrdinalLabelKey]

	ordinal, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return -1
	}

	return int32(ordinal)
}

// Kubernetes I/O helpers.
// VirtualControlPlane
func (r *reconciler) getVirtualControlPlane(ctx context.Context, name string) (*controlplanev1alpha1.VirtualControlPlane, error) {
	vcp := &controlplanev1alpha1.VirtualControlPlane{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, vcp)
	return vcp, err
}

// Namespace
func (r *reconciler) getNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, ns)
	return ns, err
}

func (r *reconciler) createNamespace(ctx context.Context, ns *corev1.Namespace) error {
	return r.client.Create(ctx, ns)
}

func (r *reconciler) patchNamespace(ctx context.Context, base, ns *corev1.Namespace) error {
	return r.client.Patch(ctx, ns, client.MergeFrom(base))
}

// Secret
func (r *reconciler) getSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	return secret, err
}

func (r *reconciler) createSecret(ctx context.Context, secret *corev1.Secret) error {
	return r.client.Create(ctx, secret)
}

// Service
func (r *reconciler) getService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, service)
	return service, err
}

func (r *reconciler) createService(ctx context.Context, service *corev1.Service) error {
	return r.client.Create(ctx, service)
}

func (r *reconciler) patchService(ctx context.Context, base, service *corev1.Service) error {
	return r.client.Patch(ctx, service, client.MergeFrom(base))
}

// ControlPlaneNode
func (r *reconciler) getControlPlaneNode(ctx context.Context, namespace, name string) (*controlplanev1alpha1.ControlPlaneNode, error) {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cpn)
	return cpn, err
}

func (r *reconciler) getControlPlaneNodesByVirtualControlPlane(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) ([]controlplanev1alpha1.ControlPlaneNode, error) {
	cpnList := &controlplanev1alpha1.ControlPlaneNodeList{}
	err := r.client.List(
		ctx,
		cpnList,
		client.InNamespace(constants.VirtualControlPlaneNamespacePrefix+vcp.Name),
	)
	if err != nil {
		return nil, err
	}

	return cpnList.Items, nil
}

func (r *reconciler) createControlPlaneNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Create(ctx, cpn)
}

func (r *reconciler) patchControlPlaneNode(ctx context.Context, base, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Patch(ctx, cpn, client.MergeFrom(base))
}

func (r *reconciler) deleteControlPlaneNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	if cpn.DeletionTimestamp != nil {
		return nil
	}

	return client.IgnoreNotFound(r.client.Delete(ctx, cpn))
}
