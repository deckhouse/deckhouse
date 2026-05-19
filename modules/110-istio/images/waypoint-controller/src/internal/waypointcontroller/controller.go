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

package waypointcontroller

import (
	"context"
	"fmt"
	"os"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

type WaypointController struct {
	client.Client
	apiReader          client.Reader
	proxyImage         string
	clusterDomain      string
	istioRevision      string
	istioNetworkName   string
	istioCloudPlatform string
	istioClusterID     string
	VPAEnabled         bool
	scheme             *runtime.Scheme
}

func (r *WaypointController) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.apiReader = mgr.GetAPIReader()
	r.scheme = mgr.GetScheme()
	r.proxyImage = os.Getenv("WAYPOINT_PROXY_IMAGE")
	r.clusterDomain = envOrDefault("CLUSTER_DOMAIN", "cluster.local")
	r.istioRevision = os.Getenv("ISTIO_REVISION")
	r.istioNetworkName = os.Getenv("ISTIO_NETWORK_NAME")
	r.istioCloudPlatform = os.Getenv("ISTIO_CLOUD_PLATFORM")
	r.istioClusterID = os.Getenv("ISTIO_CLUSTER_ID")

	if err := ensureOwnerUIDIndex(context.Background(), mgr.GetFieldIndexer(), r.VPAEnabled); err != nil {
		return fmt.Errorf("ensure owner UID indexes: %w", err)
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.WaypointInstance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&gatewayv1.Gateway{})
	if r.VPAEnabled {
		b = b.Owns(&vpav1.VerticalPodAutoscaler{})
	}

	return b.Complete(r)
}

func (r *WaypointController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.InfoS(
		"Reconciling WaypointInstance",
		"namespace", req.NamespacedName.Namespace,
		"name", req.NamespacedName.Name,
	)

	instance := &networkv1alpha1.WaypointInstance{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	result, syncErr := r.syncInstance(ctx, instance)

	if statusErr := r.syncedStatus(ctx, instance, syncErr); statusErr != nil {
		klog.ErrorS(statusErr, "Failed to update synced status, will requeue",
			"namespace", instance.Namespace,
			"name", instance.Name,
		)
		// If the sync itself succeeded but the status update failed,
		// return the status error to trigger a requeue.
		if syncErr == nil {
			return ctrl.Result{}, statusErr
		}
	}

	return result, syncErr
}

func (r *WaypointController) syncInstance(ctx context.Context, instance *networkv1alpha1.WaypointInstance) (ctrl.Result, error) {
	if res, err := r.handleFinalizers(ctx, instance); err != nil || res.RequeueAfter > 0 {
		return res, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	if err := r.ensureWaypointDeployment(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWaypointDisruptionBudget(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWaypointVPA(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWaypointHPA(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWaypointService(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWaypointGateway(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WaypointController) syncedStatus(ctx context.Context, instance *networkv1alpha1.WaypointInstance, syncError error) error {
	current := &networkv1alpha1.WaypointInstance{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, current); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get instance for status update: %w", err)
	}

	updated := current.DeepCopy()
	updated.Status.ObservedGeneration = current.GetGeneration()
	updated.Status.Synced = syncError == nil

	if reflect.DeepEqual(current.Status, updated.Status) {
		return nil
	}

	if err := r.Status().Update(ctx, updated); err != nil {
		return fmt.Errorf("update synced status: %w", err)
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
