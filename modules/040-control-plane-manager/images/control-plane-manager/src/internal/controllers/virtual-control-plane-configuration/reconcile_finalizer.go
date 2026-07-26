package virtualcontrolplaneconfiguration

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = "virtualcontrolplanes.control-plane.deckhouse.io/finalizer"

	postgresGoneAwaitingRequeueInterval = 10 * time.Second
	postgresSecretPrefix                = "d8ms-pg"
)

func (r *reconciler) reconcileFinalizer(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	if vcp.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(vcp, finalizerName) {
			return reconcile.Result{}, nil
		}

		base := vcp.DeepCopy()
		controllerutil.AddFinalizer(vcp, finalizerName)
		return reconcile.Result{}, r.client.Patch(ctx, vcp, client.MergeFrom(base))
	}

	if !controllerutil.ContainsFinalizer(vcp, finalizerName) {
		return reconcile.Result{}, nil
	}

	res, err := r.finalize(ctx, vcp)
	if err != nil || !res.IsZero() {
		return res, err
	}

	base := vcp.DeepCopy()
	controllerutil.RemoveFinalizer(vcp, finalizerName)
	return reconcile.Result{}, r.client.Patch(ctx, vcp, client.MergeFrom(base))
}

func (r *reconciler) finalize(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	return r.deletePostgresArtifacts(ctx, vcp)
}

func (r *reconciler) deletePostgresArtifacts(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (reconcile.Result, error) {
	postgresDeleted, err := r.ensurePostgresDeleted(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !postgresDeleted {
		return reconcile.Result{RequeueAfter: postgresGoneAwaitingRequeueInterval}, nil
	}

	postgresName := constants.VirtualResourceName(constants.VirtualDatastoreName, vcp.Name)
	for _, suffix := range []string{"repl-cert", "server-cert"} {
		name := fmt.Sprintf("%s-%s-%s", postgresSecretPrefix, postgresName, suffix)
		if err := r.deleteSecret(ctx, vcp.Namespace, name); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) ensurePostgresDeleted(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (bool, error) {
	obj := postgres()
	obj.SetNamespace(vcp.Namespace)
	obj.SetName(constants.VirtualResourceName(constants.VirtualDatastoreName, vcp.Name))

	err := r.client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get Postgres: %w", err)
	}

	if obj.GetDeletionTimestamp().IsZero() {
		if err := client.IgnoreNotFound(r.client.Delete(ctx, obj)); err != nil {
			return false, fmt.Errorf("delete Postgres: %w", err)
		}
	}

	return false, nil
}
