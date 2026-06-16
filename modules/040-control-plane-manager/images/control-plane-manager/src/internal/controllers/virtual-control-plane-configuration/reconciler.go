package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"maps"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	requeueInterval = 5 * time.Minute
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

	pkiSecret, res, err := r.reconcilePKISecret(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}

	configSecret, res, err := r.reconcileConfigSecret(ctx)
	if err != nil || !res.IsZero() {
		return res, err
	}

	if res, err := r.reconcileControlPlaneNode(ctx, vcp, pkiSecret, configSecret); err != nil || !res.IsZero() {
		return res, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *reconciler) reconcileNamespace(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	target := buildTargetNamespace(vcp)

	current, err := r.getNamespace(ctx, target.Name)
	if apierrors.IsNotFound(err) {
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

func (r *reconciler) reconcilePKISecret(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (*corev1.Secret, reconcile.Result, error) {
	target := buildTargetPKISecret(vcp)
	current, err := r.getSecret(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return nil, reconcile.Result{}, err
		}
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
			Annotations: map[string]string{
				"control-plane.deckhouse.io/pki-generation": "pending",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{}, // TODO
	}
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

func (r *reconciler) reconcileControlPlaneNode(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	target, err := buildTargetControlPlaneNode(vcp, configSecret, pkiSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	current, err := r.getControlPlaneNode(ctx, target.Namespace, target.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createControlPlaneNode(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get ControlPlaneNode: %w", err)
	}

	if isControlPlaneNodeInSync(current, target) {
		return reconcile.Result{}, nil
	}

	base := current.DeepCopy()
	applyControlPlaneNodeTarget(current, target)

	return reconcile.Result{}, r.patchControlPlaneNode(ctx, base, current)
}

func buildTargetControlPlaneNode(
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
	pkiSecret *corev1.Secret,
) (*controlplanev1alpha1.ControlPlaneNode, error) {
	spec, err := buildTargetControlPlaneNodeSpec(configSecret, pkiSecret)
	if err != nil {
		return nil, err
	}

	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.HeritageLabelKey:             constants.HeritageLabelValue,
				constants.ControlPlaneNodeNameLabelKey: name,
				constants.ControlPlaneTypeLabelKey:     string(constants.ControlPlaneTypeVirtual),
			},
		},
		Spec: spec,
	}, nil
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

// ControlPlaneNode
func (r *reconciler) getControlPlaneNode(ctx context.Context, namespace, name string) (*controlplanev1alpha1.ControlPlaneNode, error) {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cpn)
	return cpn, err
}

func (r *reconciler) createControlPlaneNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Create(ctx, cpn)
}

func (r *reconciler) patchControlPlaneNode(ctx context.Context, base, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Patch(ctx, cpn, client.MergeFrom(base))
}
