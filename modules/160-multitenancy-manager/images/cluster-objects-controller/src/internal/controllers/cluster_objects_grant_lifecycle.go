package controllers

import (
	"context"
	"fmt"
	"sort"

	"controller/api/v1alpha1"
	"controller/internal/namespaces"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const conditionReady = "Ready"

// ClusterObjectGrantReconciler reconciles ClusterObjectGrant resources: it resolves
// the set of project namespaces each grant matches (via spec.projectSelector against
// namespace labels) and reflects the available objects per project in the grant status.
//
// Grants are authored by operators; this controller does not create them, and it does
// not manage per-project RBAC (read access is granted via the aggregated user roles).
type ClusterObjectGrantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ClusterObjectGrantReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("grant", req.Name)

	grant := &v1alpha1.ClusterObjectGrant{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name}, grant); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ClusterObjectGrant: %w", err)
	}

	projects, err := r.matchingProjects(ctx, grant)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("resolve matching projects: %w", err)
	}

	availability, err := r.buildAvailability(ctx, grant, projects)
	if err != nil {
		// Surface the failure in the status but keep the work item; a policy may be
		// created shortly after the grant references it.
		r.setReady(grant, metav1.ConditionFalse, "AvailabilityError", err.Error())
		if statusErr := r.updateStatus(ctx, grant, nil); statusErr != nil {
			log.Error(statusErr, "update status after availability error")
		}
		return ctrl.Result{}, fmt.Errorf("build availability: %w", err)
	}

	r.setReady(grant, metav1.ConditionTrue, "Reconciled",
		fmt.Sprintf("Grant applies to %d project(s)", len(projects)))
	if err := r.updateStatus(ctx, grant, availability); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}

	return ctrl.Result{}, nil
}

// matchingProjects returns the sorted names of non-system project namespaces whose
// labels match the grant's projectSelector. A nil selector matches nothing.
func (r *ClusterObjectGrantReconciler) matchingProjects(
	ctx context.Context,
	grant *v1alpha1.ClusterObjectGrant,
) ([]string, error) {
	if grant.Spec.ProjectSelector == nil {
		return nil, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(grant.Spec.ProjectSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid projectSelector: %w", err)
	}

	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	projects := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		if namespaces.IsSystem(ns.Name) {
			continue
		}
		projects = append(projects, ns.Name)
	}
	sort.Strings(projects)
	return projects, nil
}

// buildAvailability computes, for every matching project, the objects the grant makes
// available there (grouped by the granted resource kind). Selector-based grants are
// reflected as a synthetic "<selector>" entry; explicit names are listed individually.
func (r *ClusterObjectGrantReconciler) buildAvailability(
	ctx context.Context,
	grant *v1alpha1.ClusterObjectGrant,
	projects []string,
) ([]v1alpha1.ProjectAvailability, error) {
	if len(projects) == 0 {
		return nil, nil
	}

	// Resolve the granted resource kind for each referenced policy once.
	byKind := make(map[string][]v1alpha1.AvailableClusterObjectRef)
	for _, p := range grant.Spec.Policies {
		policy := &v1alpha1.ClusterObjectGrantPolicy{}
		if err := r.Get(ctx, types.NamespacedName{Name: p.Name}, policy); err != nil {
			if k8serrors.IsNotFound(err) {
				// Referencing a missing policy is not fatal; just skip it.
				continue
			}
			return nil, fmt.Errorf("get ClusterObjectGrantPolicy %s: %w", p.Name, err)
		}

		kind := policy.Spec.GrantedResource.Kind
		if kind == "" {
			kind = p.Name
		}

		for _, name := range p.Allowed {
			byKind[kind] = append(byKind[kind], v1alpha1.AvailableClusterObjectRef{
				Name:    name,
				Default: name == p.Default,
			})
		}
		if p.AllowedSelector != nil {
			sel, err := metav1.LabelSelectorAsSelector(p.AllowedSelector)
			if err != nil {
				return nil, fmt.Errorf("invalid allowedSelector in policy %s: %w", p.Name, err)
			}
			byKind[kind] = append(byKind[kind], v1alpha1.AvailableClusterObjectRef{
				Name: "selector:" + sel.String(),
			})
		}
	}

	// The available set is currently identical across all matching projects; per-project
	// divergence (e.g. status field values fetched from live objects) can extend this.
	availability := make([]v1alpha1.ProjectAvailability, 0, len(projects))
	for _, project := range projects {
		availability = append(availability, v1alpha1.ProjectAvailability{
			Project:   project,
			Available: byKind,
		})
	}
	return availability, nil
}

func (r *ClusterObjectGrantReconciler) setReady(
	grant *v1alpha1.ClusterObjectGrant,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&grant.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: grant.Generation,
	})
}

func (r *ClusterObjectGrantReconciler) updateStatus(
	ctx context.Context,
	grant *v1alpha1.ClusterObjectGrant,
	availability []v1alpha1.ProjectAvailability,
) error {
	grant.Status.Projects = availability
	grant.Status.ObservedGeneration = grant.Generation
	return r.Status().Update(ctx, grant)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterObjectGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// A namespace label change can alter which grants match it, so re-enqueue every
	// grant when any namespace changes.
	enqueueAllGrants := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			grants := &v1alpha1.ClusterObjectGrantList{}
			if err := r.List(ctx, grants); err != nil {
				return nil
			}
			reqs := make([]reconcile.Request, 0, len(grants.Items))
			for _, g := range grants.Items {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: g.Name}})
			}
			return reqs
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterObjectGrant{}).
		Watches(&corev1.Namespace{}, enqueueAllGrants).
		Named("ClusterObjectGrant").
		Complete(r)
}
