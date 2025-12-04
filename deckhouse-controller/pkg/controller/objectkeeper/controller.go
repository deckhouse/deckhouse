/*
Copyright 2025 Flant JSC

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

package objectkeeper

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// FollowObjectCheckInterval is the interval for checking FollowObject status
	FollowObjectCheckInterval = 2 * time.Minute
	// TTLCheckInterval is the interval for checking TTL expiration
	TTLCheckInterval = 1 * time.Minute

	MaxConcurrentReconciles = 5

	// Retained until the referenced object is deleted
	ModeFollowObject = "FollowObject"
	// Retained for a specific time after creation
	ModeTTL = "TTL"
	// Retained until the referenced object is deleted, then kept for an additional TTL period
	ModeFollowObjectWithTTL = "FollowObjectWithTTL"
)

// ObjectKeeperController reconciles ObjectKeeper objects
// This is a system controller that manages the lifecycle of ObjectKeeper resources
// It requires privileged access to GET any namespaced objects
type ObjectKeeperController struct {
	client.Client
	dc         dependency.Container
	restMapper meta.RESTMapper
	dyn        dynamic.Interface
	logger     *log.Logger
}

func RegisterController(
	mgr manager.Manager,
	dc dependency.Container,
	logger *log.Logger,
) error {
	// Build dynamic client for accessing arbitrary API resources
	dyn, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// RESTMapper for efficient Kind-to-resource mapping

	restMapper := mgr.GetRESTMapper()

	r := &ObjectKeeperController{
		Client:     mgr.GetClient(),
		dc:         dc,
		restMapper: restMapper,
		dyn:        dyn,
		logger:     logger,
	}

	ctr, err := controller.New("objectkeeper-controller", mgr, controller.Options{
		MaxConcurrentReconciles: MaxConcurrentReconciles,
		CacheSyncTimeout:        3 * time.Minute,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	objectkeeperPredicate := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Reconcile on spec changes
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			// No need to reconcile deleted ObjectKeepers
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}

	namespacePredicate := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(_ event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			// Reconcile related ObjectKeepers
			return true
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}

	// Index spec.followObjectRef.namespace field for ObjectKeeper resources.
	// Enables efficient lookup of ObjectKeepers linked to a specific namespace,
	// useful for triggering reconciliations, when a referenced namespace is deleted.
	err = mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.ObjectKeeper{}, "spec.followObjectRef.namespace", func(obj client.Object) []string {
		ret, ok := obj.(*v1alpha1.ObjectKeeper)
		if !ok || ret.Spec.FollowObjectRef == nil {
			return nil // No index
		}
		ns := ret.Spec.FollowObjectRef.Namespace
		if ns == "" {
			return nil
		}
		return []string{ns}
	})
	if err != nil {
		return fmt.Errorf("failed to index followObjectRef namespace: %w", err)
	}

	r.logger.Info("Controller started")

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ObjectKeeper{},
			builder.WithPredicates(objectkeeperPredicate),
		).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				ns, ok := obj.(*corev1.Namespace)
				if !ok {
					return nil
				}

				var objectkeepersList v1alpha1.ObjectKeeperList
				if err := r.List(ctx, &objectkeepersList, client.MatchingFields{"spec.followObjectRef.namespace": ns.Name}); err != nil {
					r.logger.Error("Failed to list ObjectKeepers for namespace cleanup", log.Err(err))
					return nil
				}

				var reqs []reconcile.Request
				for _, ret := range objectkeepersList.Items {
					if ret.Spec.Mode == ModeFollowObject || ret.Spec.Mode == ModeFollowObjectWithTTL {
						r.logger.Info("Requeue objectkeeper due to namespace deletion",
							"objectkeeper", ret.Name,
							"namespace", ns.Name)
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{Name: ret.Name},
						})
					}
				}
				return reqs
			}),
			builder.WithPredicates(namespacePredicate),
		).Complete(ctr)
}

func (r *ObjectKeeperController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Info("Reconciling ObjectKeeper", "name", req.Name)

	objectkeeper := &v1alpha1.ObjectKeeper{}
	if err := r.Get(ctx, req.NamespacedName, objectkeeper); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	// No finalizer needed - GC handles cleanup via ownerReferences
	if !objectkeeper.DeletionTimestamp.IsZero() {
		// ObjectKeeper is being deleted, nothing to do
		// GC will handle cleanup of dependent objects
		return ctrl.Result{}, nil
	}
	// Process based on mode
	switch objectkeeper.Spec.Mode {
	case ModeFollowObject:
		return r.reconcileFollowObject(ctx, objectkeeper)
	case ModeTTL:
		return r.reconcileTTL(ctx, objectkeeper)
	case ModeFollowObjectWithTTL:
		return r.reconcileFollowObjectWithTTL(ctx, objectkeeper)
	default:
		// Should never happen: mode is validated at the API level(enum).
		return ctrl.Result{}, fmt.Errorf("Unknown mode %v", objectkeeper.Spec.Mode)
	}
}

// reconcileFollowObject handles ObjectKeeper in FollowObject mode
func (r *ObjectKeeperController) reconcileFollowObject(ctx context.Context, objectkeeper *v1alpha1.ObjectKeeper) (ctrl.Result, error) {
	if objectkeeper.Spec.FollowObjectRef == nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = "FollowObjectRef is required for FollowObject mode"
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "MissingFollowObjectRef",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})

		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	ref := objectkeeper.Spec.FollowObjectRef

	// Parse APIVersion to get Group and Version
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = fmt.Sprintf("Invalid APIVersion: %s", ref.APIVersion)
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "InvalidAPIVersion",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Determine resource from Kind
	resource, err := r.kindToResource(ref.Kind, gv)
	if err != nil {
		r.logger.Error("RESTMapper failed",
			"kind", ref.Kind,
			"groupVersion", gv.String(),
			"error", err)
		return ctrl.Result{}, err
	}
	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}

	// Get the object using dynamic client
	obj, err := r.dyn.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found - delete ObjectKeeper
			r.logger.Info("FollowObject not found - deleting ObjectKeeper",
				"objectkeeper", objectkeeper.Name,
				"object", fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name),
				"namespace", ref.Namespace)
			if err := r.Delete(ctx, objectkeeper); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
			}
			return ctrl.Result{}, nil
		}
		// Other error - retry
		r.logger.Error("Failed to get FollowObject",
			"objectkeeper", objectkeeper.Name,
			"object", fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: FollowObjectCheckInterval}, nil
	}

	// Verify UID matches
	objUID := string(obj.GetUID())
	if objUID != ref.UID {
		// Object was recreated with different UID - delete ObjectKeeper
		r.logger.Info("FollowObject UID mismatch - deleting ObjectKeeper",
			"objectkeeper", objectkeeper.Name,
			"expectedUID", ref.UID,
			"actualUID", objUID)
		if err := r.Delete(ctx, objectkeeper); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Object exists and UID matches - ObjectKeeper is active
	base := objectkeeper.DeepCopy()
	objectkeeper.Status.Phase = v1alpha1.PhaseTracking
	objectkeeper.Status.Message = fmt.Sprintf("Following object %s/%s/%s", ref.APIVersion, ref.Kind, ref.Name)
	setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
		Type:               "Active",
		Status:             metav1.ConditionTrue,
		Reason:             "ObjectExists",
		Message:            objectkeeper.Status.Message,
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for periodic check
	return ctrl.Result{RequeueAfter: FollowObjectCheckInterval}, nil
}

// reconcileTTL handles ObjectKeeper in TTL mode
func (r *ObjectKeeperController) reconcileTTL(ctx context.Context, objectkeeper *v1alpha1.ObjectKeeper) (ctrl.Result, error) {
	if objectkeeper.Spec.TTL == nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = "TTL is required for TTL mode"
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "MissingTTL",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Calculate expiration time
	expiresAt := objectkeeper.CreationTimestamp.Add(objectkeeper.Spec.TTL.Duration)
	now := metav1.Now()

	if now.After(expiresAt) {
		// TTL expired - delete ObjectKeeper
		r.logger.Info("TTL expired - deleting ObjectKeeper",
			"objectkeeper", objectkeeper.Name,
			"ttl", objectkeeper.Spec.TTL.Duration.Round(time.Minute),
			"created", objectkeeper.CreationTimestamp,
			"expired", expiresAt)
		if err := r.Delete(ctx, objectkeeper); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// TTL not expired - ObjectKeeper is active
	base := objectkeeper.DeepCopy()
	objectkeeper.Status.Phase = v1alpha1.PhaseWaitingTTL
	objectkeeper.Status.Message = fmt.Sprintf("TTL expires at %v", expiresAt.Format(time.RFC3339))
	setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
		Type:               "Active",
		Status:             metav1.ConditionTrue,
		Reason:             "TTLActive",
		Message:            objectkeeper.Status.Message,
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for periodic check
	return ctrl.Result{RequeueAfter: TTLCheckInterval}, nil
}

// reconcileFollowObjectWithTTL handles ObjectKeeper in FollowObjectWithTTL mode
// This is a hybrid mode: follows object, but if object disappears, starts TTL countdown
func (r *ObjectKeeperController) reconcileFollowObjectWithTTL(ctx context.Context, objectkeeper *v1alpha1.ObjectKeeper) (ctrl.Result, error) {
	if objectkeeper.Spec.FollowObjectRef == nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = "FollowObjectRef is required for FollowObjectWithTTL mode"
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "MissingFollowObjectRef",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if objectkeeper.Spec.TTL == nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = "TTL is required for FollowObjectWithTTL mode"
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "MissingTTL",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	ref := objectkeeper.Spec.FollowObjectRef

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.Phase = v1alpha1.PhasePending
		objectkeeper.Status.Message = fmt.Sprintf("Invalid APIVersion: %s", ref.APIVersion)
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "Active",
			Status:             metav1.ConditionFalse,
			Reason:             "InvalidAPIVersion",
			Message:            objectkeeper.Status.Message,
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Determine resource from Kind using RESTMapper
	resource, err := r.kindToResource(ref.Kind, gv)
	if err != nil {
		r.logger.Error("RESTMapper failed",
			"kind", ref.Kind,
			"groupVersion", gv.String(),
			"error", err)
		return ctrl.Result{}, err
	}

	// Get the object using dynamic client
	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}

	obj, err := r.dyn.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Set LostAt and start TTL countdown
			now := metav1.Now()
			if objectkeeper.Status.LostAt == nil {
				base := objectkeeper.DeepCopy()
				objectkeeper.Status.LostAt = &now
				objectkeeper.Status.Phase = v1alpha1.PhaseWaitingTTL
				objectkeeper.Status.Message = "FollowObject not found; starting TTL countdown"
				setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
					Type:               "TTLActive",
					Status:             metav1.ConditionFalse,
					Reason:             "MissingFollowObject",
					Message:            fmt.Sprintf("TTL expires at %v", now.Add(objectkeeper.Spec.TTL.Duration).Format(time.RFC3339)),
					LastTransitionTime: now,
				})
				if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
					return ctrl.Result{RequeueAfter: TTLCheckInterval}, err
				}
			}

			// Calculate expiration time if LostAt exist
			expiresAt := objectkeeper.Status.LostAt.Add(objectkeeper.Spec.TTL.Duration)
			// Object not found - wait TTL before deleting objects
			r.logger.Info("FollowObject not found - will delete ObjectKeeper after TTL expiration",
				"objectkeeper", objectkeeper.Name,
				"object", fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name),
				"namespace", ref.Namespace,
				"expired", expiresAt.Format(time.RFC3339))

			if now.After(expiresAt) {
				// TTL expired and FollowObject notFound - delete ObjectKeeper
				r.logger.Info("TTL expired - deleting ObjectKeeper",
					"objectkeeper", objectkeeper.Name,
					"ttl", objectkeeper.Spec.TTL.Duration.Round(time.Minute),
					"lostAt", objectkeeper.Status.LostAt,
					"expired", expiresAt.Format(time.RFC3339))
				if err := r.Delete(ctx, objectkeeper); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
				}
			}
			return ctrl.Result{RequeueAfter: TTLCheckInterval}, nil
		}
		// Other error - retry
		r.logger.Error("Failed to get FollowObject",
			"objectkeeper", objectkeeper.Name,
			"object", fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name), log.Err(err))
		return ctrl.Result{RequeueAfter: FollowObjectCheckInterval}, nil
	}

	// Object exists - verify UID matches
	objUID := string(obj.GetUID())
	if objUID != ref.UID {
		if objectkeeper.Status.LostAt == nil {
			now := metav1.Now()
			base := objectkeeper.DeepCopy()
			objectkeeper.Status.LostAt = &now
			objectkeeper.Status.Phase = v1alpha1.PhaseWaitingTTL
			objectkeeper.Status.Message = "FollowObject not found; starting TTL countdown"
			setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
				Type:               "TTLActive",
				Status:             metav1.ConditionFalse,
				Reason:             "MissingFollowObject",
				Message:            fmt.Sprintf("TTL expires at %v", now.Add(objectkeeper.Spec.TTL.Duration).Format(time.RFC3339)),
				LastTransitionTime: now,
			})
			if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Object was recreated with different UID - treat as deletion, delete ObjectKeeper after TTL
		expiresAt := objectkeeper.Status.LostAt.Add(objectkeeper.Spec.TTL.Duration)
		now := metav1.Now()
		base := objectkeeper.DeepCopy()
		objectkeeper.Status.LostAt = &now
		objectkeeper.Status.Phase = v1alpha1.PhaseWaitingTTL
		objectkeeper.Status.Message = "FollowObject UID mismatch - will delete ObjectKeeper after TTL expiration"
		setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
			Type:               "TTLActive",
			Status:             metav1.ConditionFalse,
			Reason:             "MissingFollowObjectRef",
			Message:            fmt.Sprintf("TTL expires at %v", expiresAt.Format(time.RFC3339)),
			LastTransitionTime: now,
		})
		if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}

		if now.After(expiresAt) {
			// TTL expired and FollowObject notFound - delete ObjectKeeper
			r.logger.Info("TTL expired - deleting ObjectKeeper",
				"objectkeeper", objectkeeper.Name,
				"ttl", objectkeeper.Spec.TTL.Duration.Round(time.Minute),
				"created", objectkeeper.CreationTimestamp,
				"expired", expiresAt)
			if err := r.Delete(ctx, objectkeeper); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
			}
		}
		r.logger.Info("FollowObject UID mismatch (recreated) - deleting ObjectKeeper immediately",
			"objectkeeper", objectkeeper.Name,
			"expectedUID", ref.UID,
			"actualUID", objUID)
		if err := r.Delete(ctx, objectkeeper); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete ObjectKeeper: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Object exists and UID matches - ObjectKeeper is active
	base := objectkeeper.DeepCopy()
	objectkeeper.Status.Phase = v1alpha1.PhaseTracking
	objectkeeper.Status.Message = fmt.Sprintf("Following object %s/%s/%s", ref.APIVersion, ref.Kind, ref.Name)
	setSingleCondition(&objectkeeper.Status.Conditions, metav1.Condition{
		Type:               "Active",
		Status:             metav1.ConditionTrue,
		Reason:             "ObjectExists",
		Message:            objectkeeper.Status.Message,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Patch(ctx, objectkeeper, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for periodic check
	return ctrl.Result{RequeueAfter: FollowObjectCheckInterval}, nil
}

// kindToResource converts Kind to resource name using RESTMapper
func (r *ObjectKeeperController) kindToResource(kind string, gv schema.GroupVersion) (string, error) {
	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
	mapping, err := r.restMapper.RESTMapping(gvk.GroupKind(), gv.Version)
	if err != nil {
		return "", fmt.Errorf("failed to resolve resource for kind %s: %w", kind, err)
	}
	return mapping.Resource.Resource, nil
}

// setSingleCondition sets a condition, removing any existing condition of the same type first
// This ensures that each condition type appears only once, preventing duplicates
func setSingleCondition(conds *[]metav1.Condition, cond metav1.Condition) {
	meta.RemoveStatusCondition(conds, cond.Type)
	meta.SetStatusCondition(conds, cond)
}
