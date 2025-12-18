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
	"k8s.io/apimachinery/pkg/labels"
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

	"safe-updater/internal/checks"
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

func (r *reconciler) getControllerRevisionByLabelSelector(ctx context.Context, namespace string, labelSelector labels.Selector) (string, error) {
	controllerRevisionList := new(appsv1.ControllerRevisionList)
	err := r.client.List(ctx, controllerRevisionList, &client.ListOptions{LabelSelector: labelSelector, Namespace: namespace})
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

func (r *reconciler) listPodsByLabelSelector(ctx context.Context, namespace string, labelSelector labels.Selector) (*corev1.PodList, error) {
	podList := new(corev1.PodList)
	err := r.client.List(ctx, podList, &client.ListOptions{LabelSelector: labelSelector, Namespace: namespace})

	return podList, err
}

func (r *reconciler) updateNextReadyPod(ctx context.Context, pods *corev1.PodList, currentRevision string, externalChecks ...checks.ExternalCheck) (ctrl.Result, error) {
ExtLoop:
	for _, pod := range pods.Items {
		podRevision := pod.GetLabels()[constant.PodTemplateGenerationLabel]
		if podRevision != currentRevision {
			for _, check := range externalChecks {
				checkRes := check.GetCheckResult(&pod)
				klog.V(5).Infof("Updating %s is %s by the %s check", pod.Name, checkRes, check.GetName())

				switch checkRes {
				case checks.Allowed:

				case checks.Denied:
					continue ExtLoop
				}
			}

			if err := r.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete the %s ready pod", pod.Name)
			}
			klog.V(5).Infof("Deleted the %s ready pod", pod.Name)
			break ExtLoop
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) updateNextNotReadyPod(ctx context.Context, pods *corev1.PodList, currentRevision string, externalChecks ...checks.ExternalCheck) (ctrl.Result, error) {
ExtLoop:
	for _, pod := range pods.Items {
		if !checks.PodIsReadyAndRunning(&pod) {
			if !pod.DeletionTimestamp.IsZero() {
				break
			}

			podRevision := pod.GetLabels()[constant.PodTemplateGenerationLabel]
			if podRevision != currentRevision {
				for _, check := range externalChecks {
					checkRes := check.GetCheckResult(&pod)
					klog.V(5).Infof("Updating %s is %s by the %s check", pod.Name, checkRes, check.GetName())

					switch checkRes {
					case checks.Allowed:

					case checks.Denied:
						continue ExtLoop
					}
				}

				if err := r.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete the %s not ready pod", pod.Name)
				}
				klog.V(5).Infof("Deleted the %s not ready pod", pod.Name)
				break ExtLoop
			}
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *reconciler) reconcileDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) (ctrl.Result, error) {
	if checks.DaemonSetIsUpToDate(ds) {
		klog.V(5).Infof("DaemonSet is up to date")
		return ctrl.Result{}, nil
	}

	pods, err := r.listPodsByLabelSelector(ctx, constant.NodeLocalDNSNamespace, constant.NodeLocalDNSPodLabelSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodeLocalDNSControllerRevision, err := r.getControllerRevisionByLabelSelector(ctx, constant.NodeLocalDNSNamespace, constant.ControllerRevisionLabelSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get the node-local-dns controller revision: %w", err)
	}
	klog.V(5).Infof("current node-local-dns controller revision is %s", nodeLocalDNSControllerRevision)

	ciliumPods, err := r.listPodsByLabelSelector(ctx, constant.CiliumNamespace, constant.CiliumAgentPodLabelSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	ciliumControllerRevision, err := r.getControllerRevisionByLabelSelector(ctx, constant.CiliumNamespace, constant.CiliumAgentPodLabelSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get the cilium controller revision: %w", err)
	}
	klog.V(5).Infof("current cilium controller revision is %s", ciliumControllerRevision)

	ciliumCheck, err := checks.NewCniCiliumCheck(ctx, ciliumPods, ciliumControllerRevision)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get a new cilium check: %w", err)
	}

	if checks.DaemonSetIsStable(pods) {
		return r.updateNextReadyPod(ctx, pods, nodeLocalDNSControllerRevision, ciliumCheck)
	}

	return r.updateNextNotReadyPod(ctx, pods, nodeLocalDNSControllerRevision, ciliumCheck)
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

	// skip daemonset if its update strategy is rolling update
	if nldDaemonSet.Spec.UpdateStrategy.Type != appsv1.OnDeleteDaemonSetStrategyType {
		return ctrl.Result{}, nil
	}

	return r.reconcileDaemonSet(ctx, nldDaemonSet)
}
