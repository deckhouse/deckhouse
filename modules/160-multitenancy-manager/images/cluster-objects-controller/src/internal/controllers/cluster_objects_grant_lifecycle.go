package controllers

import (
	"context"
	"fmt"

	"controller/api/v1alpha1"
	"controller/internal/namespaces"

	projectsv1alpha2 "controller/internal/extapi/deckhouse.io/v1alpha2"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterObjectGrantLifecycleController listens to the Project lifecycle events,
// creating and destroying basic ClusterObjectGrant and RBAC RoleBindings.
type ClusterObjectGrantLifecycleController struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ClusterObjectGrantLifecycleController) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Reconciling Project")
	proj := &projectsv1alpha2.Project{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name}, proj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get Project: %w", err)
	}

	if namespaces.IsSystem(req.Name) {
		log.Info("Will not reconcile system project", "project_name", proj.Name)
		return ctrl.Result{}, nil
	}

	// We are not interested in virtual projects
	if proj.Spec.ProjectTemplateName == "virtual" {
		log.Info("Will not reconcile virtual project", "project_name", proj.Name)
		return ctrl.Result{}, nil
	}

	if !proj.DeletionTimestamp.IsZero() {
		return r.reconcileDeletedProject(ctx, proj)
	}

	return r.reconcileProject(ctx, proj)
}

func (r *ClusterObjectGrantLifecycleController) reconcileProject(
	ctx context.Context,
	proj *projectsv1alpha2.Project,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("project", proj.Name)
	grant := &v1alpha1.ClusterObjectsGrant{}
	err := r.Get(ctx, types.NamespacedName{Name: proj.Name}, grant)
	if err == nil {
		log.Info("Project already has corresponding ClusterObjectsGrant, nothing to do")
		return ctrl.Result{}, nil
	}
	if !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get ClusterObjectsGrant for Project %s: %w", proj.Name, err)
	}

	log.Info("Creating new ClusterObjectsGrant and setting up RBAC for it")

	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{Name: fmt.Sprintf("d8:%s-objects-grants-reader", proj.Name)},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"projects.deckhouse.io"},
				Resources:     []string{"clusterobjectsgrants"},
				ResourceNames: []string{proj.Name},

				// Watch will require field selector like `metadata.name=${proj.Name}` or 401 will be returned.
				Verbs: []string{"get", "watch"},
			},
		},
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "cluster-objects-grants-read-access",
			Namespace: proj.Name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.GroupKind,
				Name:     "system:authenticated",
			},
		},
	}

	err = r.Create(ctx, &clusterRole)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("create %s ClusterRole: %w", clusterRole.Name, err)
	}

	err = r.Create(ctx, &roleBinding)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("create cluster-objects-grants-read-access RoleBinding: %w", err)
	}

	grant = &v1alpha1.ClusterObjectsGrant{
		ObjectMeta: v1.ObjectMeta{
			Name: proj.Name,
		},
		Spec: v1alpha1.ClusterObjectsGrantSpec{
			Policies: make([]v1alpha1.ApplicablePolicy, 0),
		},
	}
	grant.Name = proj.Name
	err = r.Create(ctx, grant)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("create basic ClusterObjectsGrant: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *ClusterObjectGrantLifecycleController) reconcileDeletedProject(
	ctx context.Context,
	proj *projectsv1alpha2.Project,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("project", proj.Name)
	log.Info("Project is deleted, removing corresponding ClusterObjectsGrant")
	grant := &v1alpha1.ClusterObjectsGrant{ObjectMeta: v1.ObjectMeta{Name: proj.Name}}
	err := r.Delete(ctx, grant)
	if !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("delete ClusterObjectsGrant: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterObjectGrantLifecycleController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectsv1alpha2.Project{}).
		Named("ClusterObjectGrantLifecycle").
		Complete(r)
}
