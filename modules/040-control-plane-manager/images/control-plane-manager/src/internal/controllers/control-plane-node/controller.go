package controlplanenode

import (
	"context"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client client.Client
}

func Register(mgr manager.Manager) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	r := &Reconciler{
		client: mgr.GetClient(),
	}

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: nodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpnControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(nodeLabelPredicate),
		).
		Watches(
			&controlplanev1alpha1.ControlPlaneNode{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(nodeLabelPredicate),
		).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeName := req.Name
	log.Info("Reconcile started for ControlPlaneNode", slog.String("node", nodeName))

	controlPlaneNode := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, controlPlaneNode)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ControlPlaneNode not found, skipping", slog.String("node", nodeName))
			return reconcile.Result{}, nil
		}
		return reconcile.Result{RequeueAfter: requeueInterval}, err
	}

	log.Info("ControlPlaneNode found", slog.String("node", nodeName))

	if err := r.reconcileComponents(ctx, controlPlaneNode); err != nil {
		return reconcile.Result{RequeueAfter: requeueInterval}, err
	}

	return reconcile.Result{}, nil
}

// componentCheck holds spec and status checksums for a single component.
type componentCheck struct {
	component      controlplanev1alpha1.OperationComponent
	specChecksum   string
	statusChecksum string
}

// reconcileComponents compares spec vs status checksums and creates ControlPlaneOperation
// for each component where they differ.
func (r *Reconciler) reconcileComponents(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	nodeName := cpn.Name
	checks := r.buildComponentChecks(cpn)

	for _, check := range checks {
		if check.specChecksum == check.statusChecksum {
			continue
		}

		operationName := operationNameForNode(nodeName, check.component, check.specChecksum)
		existing := &controlplanev1alpha1.ControlPlaneOperation{}
		err := r.client.Get(ctx, client.ObjectKey{Name: operationName}, existing)
		if err == nil {
			log.Debug("ControlPlaneOperation already exists, skipping",
				slog.String("operation", operationName),
				slog.String("component", string(check.component)))
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get ControlPlaneOperation %s: %w", operationName, err)
		}

		operation := &controlplanev1alpha1.ControlPlaneOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name: operationName,
			},
			Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
				ConfigVersion:      cpn.Spec.ConfigVersion,
				NodeName:           nodeName,
				Component:          check.component,
				Command:            controlplanev1alpha1.OperationCommandUpdate,
				DesiredChecksum:    check.specChecksum,
				DesiredPKIChecksum: cpn.Spec.PKIChecksum,
				Approved:           false,
			},
		}

		if err := r.client.Create(ctx, operation); err != nil {
			return fmt.Errorf("create ControlPlaneOperation %s: %w", operationName, err)
		}
		log.Info("ControlPlaneOperation created",
			slog.String("operation", operationName),
			slog.String("component", string(check.component)))
	}

	return nil
}

func (r *Reconciler) buildComponentChecks(cpn *controlplanev1alpha1.ControlPlaneNode) []componentCheck {
	spec := &cpn.Spec.Components
	status := &cpn.Status.Components

	getChecksum := func(c *controlplanev1alpha1.ComponentChecksum) string {
		if c == nil {
			return ""
		}
		return c.Checksum
	}

	return []componentCheck{
		{
			component:      controlplanev1alpha1.OperationComponentEtcd,
			specChecksum:   getChecksum(spec.Etcd),
			statusChecksum: getChecksum(status.Etcd),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeAPIServer,
			specChecksum:   getChecksum(spec.KubeAPIServer),
			statusChecksum: getChecksum(status.KubeAPIServer),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeControllerManager,
			specChecksum:   getChecksum(spec.KubeControllerManager),
			statusChecksum: getChecksum(status.KubeControllerManager),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeScheduler,
			specChecksum:   getChecksum(spec.KubeScheduler),
			statusChecksum: getChecksum(status.KubeScheduler),
		},
		{
			component:      controlplanev1alpha1.OperationComponentHotReload,
			specChecksum:   cpn.Spec.HotReloadChecksum,
			statusChecksum: cpn.Status.HotReloadChecksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentPKI,
			specChecksum:   cpn.Spec.PKIChecksum,
			statusChecksum: cpn.Status.PKIChecksum,
		},
	}
}

// operationNameForNode returns a deterministic name for ControlPlaneOperation.
// Node names may contain dots (e.g. ip-10-0-0-1.ec2.internal); k8s resource names do not allow them.
func operationNameForNode(nodeName string, component controlplanev1alpha1.OperationComponent, specChecksum string) string {
	sanitized := strings.ReplaceAll(nodeName, ".", "-")
	if len(specChecksum) > 6 {
		specChecksum = specChecksum[:6]
	}
	return fmt.Sprintf("%s-%s-%s", sanitized, component, specChecksum)
}
