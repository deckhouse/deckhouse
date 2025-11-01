/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"safe-updater/internal/constant"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	defaultRequeueInterval  = 3 * time.Second
)

type reconciler struct {
	client client.Client
}

func RegisterController(runtimeManager manager.Manager) error {
	r := &reconciler{
		client: runtimeManager.GetClient(),
	}

	c, err := controller.New(constant.ControllerName, runtimeManager, controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		NeedLeaderElection:      ptr.To(true),
		Reconciler:              r,
	})
	if err != nil {
		return fmt.Errorf("create controller: %w", err)
	}

	var daemonSetPredicate predicate.Predicate = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() != constant.NodeLocalDNSDaemonSet || e.ObjectNew.GetNamespace() != constant.NodeLocalDNSNamespace {
				return false
			}

			oldDaemonSet, ok := e.ObjectOld.(*appsv1.DaemonSet)
			if !ok {
				return false
			}

			newDaemonSet, ok := e.ObjectNew.(*appsv1.DaemonSet)
			if !ok {
				return false
			}

			if oldDaemonSet.Status.UpdatedNumberScheduled == newDaemonSet.Status.UpdatedNumberScheduled {
				return false
			}

			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&appsv1.DaemonSet{}, builder.WithPredicates(daemonSetPredicate)).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Complete(c)
}

func (r *reconciler) daemonSetIsUpToDate(ds *appsv1.DaemonSet) bool {
	return ds.GetGeneration() == ds.Status.ObservedGeneration &&
		ds.Status.DesiredNumberScheduled == ds.Status.UpdatedNumberScheduled
}

func (r *reconciler) podIsReadyAndRunning(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}

func (r *reconciler) daemonSetIsStable(pods *corev1.PodList) bool {
	for _, pod := range pods.Items {
		if !r.podIsReadyAndRunning(&pod) || !pod.DeletionTimestamp.IsZero() {
			return false
		}
	}

	return true
}

// returns the revision number as a string as it is later compared against pods' annotations
func (r *reconciler) getCurrentControllerRevision(ctx context.Context) (string, error) {
	controllerRevisionList := new(appsv1.ControllerRevisionList)
	err := r.client.List(ctx, controllerRevisionList, &client.ListOptions{LabelSelector: constant.ControllerRevisionLabelSelector, Namespace: constant.NodeLocalDNSNamespace})
	if err != nil {
		return "", err
	}

	var currentRevision int64
	for _, cr := range controllerRevisionList.Items {
		if cr.Revision > currentRevision {
			currentRevision = cr.Revision
		}
	}

	if currentRevision == 0 {
		return "", fmt.Errorf("no controller revision found")
	}

	return strconv.FormatInt(currentRevision, 10), nil
}

func (r *reconciler) getDaemonSetPods(ctx context.Context) (*corev1.PodList, error) {
	podList := new(corev1.PodList)
	err := r.client.List(ctx, podList, &client.ListOptions{LabelSelector: constant.NodeLocalDNSPodLabelSelector, Namespace: constant.NodeLocalDNSNamespace})
	if err != nil {
		return nil, fmt.Errorf("failed to list the DaemonSet pods: %w", err)
	}

	return podList, nil
}

func (r *reconciler) updateNextReadyPod(ctx context.Context, pods *corev1.PodList, currentRevision string) (ctrl.Result, error) {
	for _, pod := range pods.Items {
		podRevision := pod.GetLabels()[constant.PodTemplateGenerationLabel]
		if podRevision != currentRevision && r.okToDeletePod(&pod) {
			if err := r.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete the %s ready pod", pod.Name)
			}
			klog.V(5).Infof("Deleted the %s ready pod", pod.Name)
			break
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) updateNextNotReadyPod(ctx context.Context, pods *corev1.PodList, currentRevision string) (ctrl.Result, error) {
	for _, pod := range pods.Items {
		if !r.podIsReadyAndRunning(&pod) {
			if !pod.DeletionTimestamp.IsZero() {
				break
			}

			podRevision := pod.GetLabels()[constant.PodTemplateGenerationLabel]
			if podRevision != currentRevision && r.okToDeletePod(&pod) {
				if err := r.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete the %s not ready pod", pod.Name)
				}
				klog.V(5).Infof("Deleted the %s not ready pod", pod.Name)
				break
			}
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) okToDeletePod(pod *corev1.Pod) bool {
	// TODO: CONDITIONS

	return true
}

func (r *reconciler) reconcileDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) (ctrl.Result, error) {
	pods, err := r.getDaemonSetPods(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if r.daemonSetIsUpToDate(ds) {
		klog.V(5).Infof("DaemonSet is up to date")
		return ctrl.Result{}, nil
	}

	currentControllerRevision, err := r.getCurrentControllerRevision(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get the current controller revision: %w", err)
	}
	klog.V(5).Infof("current controller revision is %s", currentControllerRevision)

	if r.daemonSetIsStable(pods) {
		return r.updateNextReadyPod(ctx, pods, currentControllerRevision)
	}

	return r.updateNextNotReadyPod(ctx, pods, currentControllerRevision)
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nldDaemonSet := new(appsv1.DaemonSet)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, nldDaemonSet); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("DaemonSet %s/%s not found not found", req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the %s node network interface: %w", req.Name, err)
	}

	// handle delete events
	if !nldDaemonSet.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcileDaemonSet(ctx, nldDaemonSet)
}
