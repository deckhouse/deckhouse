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

package controlplane

import (
	"context"
	"control-plane-manager/pkg/constants"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sort"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 1 * time.Minute
)

type Reconciler struct {
	client client.Client
}

func Register(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.ControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneConfiguration{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(getControlPlaneConfigurationPredicate()),
		).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretToControlPlaneConfigurations),
			builder.WithPredicates(getSecretPredicate()),
		).
		Complete(r)
}

func getSecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isControlPlaneManagerConfigSecret(e.Object)
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			return isControlPlaneManagerConfigSecret(e.ObjectNew)
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return isControlPlaneManagerConfigSecret(e.Object)
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func isControlPlaneManagerConfigSecret(o client.Object) bool {
	secret, ok := o.(*corev1.Secret)
	if !ok {
		return false
	}
	return (secret.Name == constants.ControlPlaneManagerConfigSecretName || secret.Name == constants.PkiSecretName) && secret.Namespace == constants.KubeSystemNamespace
}

func getControlPlaneConfigurationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *Reconciler) getSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: constants.KubeSystemNamespace,
	}, secret)

	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Reconcile started for request: %v", req)

	cmpSecret, err := r.getSecret(ctx, constants.ControlPlaneManagerConfigSecretName)
	// TODO (trofimovdals): Add errors to status conditions.
	if err != nil {
		klog.Error("Error occurred while getting secret",
			"secret", constants.ControlPlaneManagerConfigSecretName,
			"namespace", constants.KubeSystemNamespace,
			err,
		)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	pkiSecret, err := r.getSecret(ctx, constants.PkiSecretName)
	if err != nil {
		klog.Error("Error occurred while getting secret",
			"secret", constants.ControlPlaneManagerConfigSecretName,
			"namespace", constants.KubeSystemNamespace,
			err,
		)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	desired, err := buildDesiredControlPlaneConfiguration(cmpSecret, pkiSecret)
	if err != nil {
		klog.Error("Error occurred while building desired ControlPlaneConfiguration", err)
		return reconcile.Result{}, err
	}
	if err := r.applyControlPlaneConfiguration(ctx, desired); err != nil {
		klog.Error("Error occurred while reconciling ControlPlaneConfiguration", err)
		return reconcile.Result{}, err
	}

	klog.Info("Reconcile completed successfully")
	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *Reconciler) applyControlPlaneConfiguration(ctx context.Context, desired *controlplanev1alpha1.ControlPlaneConfiguration) error {
	current := &controlplanev1alpha1.ControlPlaneConfiguration{}
	key := client.ObjectKeyFromObject(desired)
	err := r.client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		klog.Info("ControlPlaneConfiguration not found, creating")
		return r.client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(desired.Spec, current.Spec) {
		klog.Info("ControlPlaneConfiguration spec differs from desired, updating")
		current.Spec = desired.Spec
		return r.client.Update(ctx, current)
	}
	return nil
}

func calculateSimpleComponentChecksum(manifestData []byte) string {
	hash := sha256.New()
	hash.Write(manifestData)
	return hex.EncodeToString(hash.Sum(nil))
}

func calculatePKIChecksum(pkiSecret *corev1.Secret) (string, error) {
	h := sha256.New()

	keys := make([]string, 0, len(pkiSecret.Data))
	for key := range pkiSecret.Data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		h.Write([]byte(key))
		h.Write(pkiSecret.Data[key])
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func buildDesiredControlPlaneConfiguration(cmpSecret *corev1.Secret, pkiSecret *corev1.Secret) (*controlplanev1alpha1.ControlPlaneConfiguration, error) {
	pkiChecksum, err := calculatePKIChecksum(pkiSecret)
	if err != nil {
		return &controlplanev1alpha1.ControlPlaneConfiguration{}, err
	}
	return &controlplanev1alpha1.ControlPlaneConfiguration{
		ObjectMeta: ctrl.ObjectMeta{
			Name: constants.ControlPlaneConfigurationName,
		},
		Spec: controlplanev1alpha1.ControlPlaneConfigurationSpec{
			PKIChecksum: pkiChecksum,
			Components: &controlplanev1alpha1.ControlPlaneComponents{
				Etcd: &controlplanev1alpha1.ComponentChecksum{
					Checksum: calculateSimpleComponentChecksum(cmpSecret.Data["etcd.yaml.tpl"]),
				},
				KubeAPIServer: &controlplanev1alpha1.ComponentChecksum{
					Checksum: calculateSimpleComponentChecksum(cmpSecret.Data["kube-apiserver.yaml.tpl"]),
				},
				KubeControllerManager: &controlplanev1alpha1.ComponentChecksum{
					Checksum: calculateSimpleComponentChecksum(cmpSecret.Data["kube-controller-manager.yaml.tpl"]),
				},
				KubeScheduler: &controlplanev1alpha1.ComponentChecksum{
					Checksum: calculateSimpleComponentChecksum(cmpSecret.Data["kube-scheduler.yaml.tpl"]),
				},
			},
		},
	}, nil
}

func (r *Reconciler) mapSecretToControlPlaneConfigurations(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*corev1.Secret)
	if !ok {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: constants.ControlPlaneConfigurationName,
			},
		},
	}
}
