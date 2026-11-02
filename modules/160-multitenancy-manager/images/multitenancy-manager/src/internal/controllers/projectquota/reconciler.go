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

// Package projectquota reconciles ProjectQuota objects. For every namespace of the project it keeps a
// d8-project-quota ResourceQuota with the limits of the project, so that the usage of each namespace
// is counted the standard way and no namespace alone goes over the limit of the whole project. It
// labels the namespaces for the admission policies of the quota, adds the usage of the namespaces up
// in the status and says there whether the quota applies and whether the project is above it.
package projectquota

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
	"controller/internal/quota/evaluator"
)

// recheckInterval is how often a quota is checked without an event: the sum over the namespaces is
// compared to the limits independently of the events that should have moved it, and a change of the
// enforcement of the module is picked up.
const recheckInterval = 60 * time.Second

// Values of the state label of the state metric.
const (
	stateExceeded    = "exceeded"
	stateOvershoot   = "overshoot"
	stateNotEnforced = "not_enforced"
)

var (
	quotaState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_multitenancy_project_quota_state",
		Help: "A ProjectQuota in a bad state: exceeded (the usage is above the quota because the quota was lowered or found it there), overshoot (the usage grew past the quota) or not_enforced (the quota does not apply in every namespace of the project yet). Only the states that hold are exported.",
	}, []string{"project", "state"})
	enforcementMode = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_multitenancy_project_quota_enforcement",
		Help: "1 for how the project quota applies: Enforce; NotSupported where Deckhouse does not manage the control plane; Suspended by the quota-ledger-suspend ConfigMap; WebhookMissing when the webhook of the ledger is missing or does not trust its certificate; WebhookMisconfigured when it is changed so that writes do not reach the ledger. Exported by the replica of the controller that leads.",
	}, []string{"mode"})
	// quotaCount is a vector without labels so that the replica that does not lead exports no series
	// rather than a zero.
	quotaCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_multitenancy_project_quotas",
		Help: "The number of ProjectQuota objects. Exported by the replica of the controller that leads.",
	}, nil)
	overshoots = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_multitenancy_quota_ledger_overshoot_total",
		Help: "Times the usage of a project grew past its quota, by the probable cause: not_enforced (the sum was not checked), leader_change or late_write_or_recalc.",
	}, []string{"cause"})
)

func init() {
	metrics.Registry.MustRegister(quotaState, enforcementMode, quotaCount, overshoots)
}

// Reconciler keeps the ResourceQuota objects, the namespace labels and the status of a ProjectQuota.
type Reconciler struct {
	client.Client
	// Config reads the enforcement of the module.
	Config *quota.ConfigReader
	// Discovery finds the count/ keys whose resources are not served yet. Optional.
	Discovery quota.Discovery
	// Recorder reports a replaced ResourceQuota. Optional.
	Recorder record.EventRecorder
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	mu        sync.Mutex
	lastWrite map[string]time.Time
	known     map[string]struct{}
	// lastWithin is when the usage of each project was last found within its quota.
	lastWithin map[string]time.Time
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Reconcile brings one ProjectQuota and the namespaces of its project in line.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("projectquota", req.Name)

	cfg, err := r.Config.Read(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.publishEnforcement(cfg)

	pq := new(v1alpha1.ProjectQuota)
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, pq); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("get the ProjectQuota: %w", err)
		}
		r.forget(req.Name)
		return reconcile.Result{}, r.release(ctx, req.Name)
	}
	if !pq.DeletionTimestamp.IsZero() {
		r.forget(req.Name)
		if err := r.release(ctx, req.Name); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, r.removeFinalizer(ctx, pq)
	}
	if err := r.ensureFinalizer(ctx, pq); err != nil {
		return reconcile.Result{}, err
	}
	r.remember(pq)

	project := new(v1alpha3.Project)
	projectFound := true
	if err := r.Get(ctx, client.ObjectKey{Name: pq.Name}, project); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("get the project: %w", err)
		}
		projectFound = false
	}
	if projectFound && project.DeletionTimestamp.IsZero() {
		if err := r.ensureOwner(ctx, pq, project); err != nil {
			return reconcile.Result{}, err
		}
	}

	hard := quota.Canonical(pq.Spec.Hard)
	rqHard := quota.ResourceQuotaHard(hard)

	var notServed []string
	if r.Discovery != nil {
		check, err := quota.CheckKeys(pq.Spec.Hard, pq.Spec.Hard, r.Discovery)
		if err != nil {
			log.Error(err, "look up the resources of the count keys")
		}
		notServed = check.NotServed
	}
	vm := familyPlan{label: len(quota.VMHard(hard)) > 0}
	var notCounted []string
	if vm.label && !cfg.CountsVirtualMachines() {
		for key := range quota.VMHard(hard) {
			if !slices.Contains(notServed, string(key)) {
				notCounted = append(notCounted, string(key))
			}
		}
		slices.Sort(notCounted)
	}
	vm.count = vm.label && len(notCounted) == 0 && !slices.ContainsFunc(notServed, func(key string) bool { return evaluator.IsVMKey(corev1.ResourceName(key)) })
	lb := familyPlan{label: len(quota.LBHard(hard)) > 0}
	lb.count = lb.label
	var balancers evaluator.LoadBalancer
	if lb.count {
		balancers = r.loadBalancerClasses(ctx)
	}

	// a namespace whose quota could not be set up is reported in the status before the error is
	// returned, so the status does not claim a quota that is not there
	var namespaces []quota.Namespace
	var namespacesErr error
	if projectFound {
		namespaces, namespacesErr = r.reconcileNamespaces(ctx, pq, rqHard, families{vm: vm, lb: lb, balancers: balancers})
		if namespaces == nil && namespacesErr != nil {
			return reconcile.Result{}, namespacesErr
		}
	} else if err := r.unlabelTerminating(ctx, pq.Name); err != nil {
		// the namespaces of a project deleted without them, or not owned by it, are going on their own
		return reconcile.Result{}, err
	}

	desired := r.desiredStatus(pq, cfg, hard, namespaces, projectFound, notServed, notCounted)
	r.publishState(pq.Name, desired.Conditions)

	now := r.now()
	r.mu.Lock()
	last := r.lastWrite[pq.Name]
	r.mu.Unlock()
	due, wait := quota.WriteDue(&pq.Status, desired, last, now)
	if due {
		if err := r.writeStatus(ctx, pq, desired); err != nil {
			return reconcile.Result{}, errors.Join(namespacesErr, err)
		}
		r.mu.Lock()
		r.lastWrite[pq.Name] = now
		r.mu.Unlock()
	}
	if namespacesErr != nil {
		return reconcile.Result{}, namespacesErr
	}

	after := recheckInterval
	if wait > 0 && wait < after {
		after = wait
	}
	return reconcile.Result{RequeueAfter: after}, nil
}

// ensureFinalizer makes the quota wait for release: the labels that bind the guard policy to the
// ResourceQuota objects come off first, and then release deletes the ResourceQuota objects.
func (r *Reconciler) ensureFinalizer(ctx context.Context, pq *v1alpha1.ProjectQuota) error {
	if controllerutil.ContainsFinalizer(pq, v1alpha1.ProjectQuotaFinalizer) {
		return nil
	}
	patch := client.MergeFromWithOptions(pq.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.AddFinalizer(pq, v1alpha1.ProjectQuotaFinalizer)
	if err := r.Patch(ctx, pq, patch); err != nil {
		return fmt.Errorf("add the finalizer to the ProjectQuota: %w", err)
	}
	return nil
}

// removeFinalizer lets a released quota go.
func (r *Reconciler) removeFinalizer(ctx context.Context, pq *v1alpha1.ProjectQuota) error {
	if !controllerutil.ContainsFinalizer(pq, v1alpha1.ProjectQuotaFinalizer) {
		return nil
	}
	patch := client.MergeFromWithOptions(pq.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.RemoveFinalizer(pq, v1alpha1.ProjectQuotaFinalizer)
	if err := r.Patch(ctx, pq, patch); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("remove the finalizer from the ProjectQuota: %w", err)
	}
	return nil
}

// ensureOwner makes the project the owner of the quota, so that the quota goes with the project.
func (r *Reconciler) ensureOwner(ctx context.Context, pq *v1alpha1.ProjectQuota, project *v1alpha3.Project) error {
	for _, ref := range pq.OwnerReferences {
		if ref.UID == project.UID {
			return nil
		}
	}
	patch := client.MergeFrom(pq.DeepCopy())
	pq.OwnerReferences = append(pq.OwnerReferences, metav1.OwnerReference{
		APIVersion: v1alpha3.SchemeGroupVersion.String(),
		Kind:       v1alpha3.ProjectKind,
		Name:       project.Name,
		UID:        project.UID,
	})
	if err := r.Patch(ctx, pq, patch); err != nil {
		return fmt.Errorf("make the project the owner of the ProjectQuota: %w", err)
	}
	return nil
}

// reconcileNamespaces keeps the ResourceQuota and the label of every namespace of the project and
// returns what it found in each. A namespace that is going is not counted: it loses the label. A
// namespace that fails is returned with its error, the others are still reconciled, and the errors
// are joined.
func (r *Reconciler) reconcileNamespaces(ctx context.Context, pq *v1alpha1.ProjectQuota, rqHard corev1.ResourceList, plan families) ([]quota.Namespace, error) {
	list := new(corev1.NamespaceList)
	if err := r.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelProject: pq.Name}); err != nil {
		return nil, fmt.Errorf("list the namespaces of the project: %w", err)
	}

	namespaces := make([]quota.Namespace, 0, len(list.Items))
	var errs []error
	for i := range list.Items {
		ns := &list.Items[i]
		if !ns.DeletionTimestamp.IsZero() {
			// The namespace controller deletes d8-project-quota with the rest of the namespace, but the
			// objects that wait for a finalizer, such as the claims behind pvc-protection, are still
			// updated, and the guard policy refuses those updates once its parameter is gone. The
			// label comes off, and nothing is made in a namespace that is going.
			if err := r.removeLabel(ctx, ns); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		found, err := r.reconcileNamespace(ctx, pq, ns, rqHard, plan)
		if err != nil {
			errs = append(errs, err)
			found.Problem = err.Error()
		}
		namespaces = append(namespaces, found)
	}
	return namespaces, errors.Join(errs...)
}

// unlabelTerminating takes the label off the namespaces of the project that are being deleted, for a
// quota whose project is not there: the guard policies would refuse the updates of the objects that
// wait for a finalizer there once the namespace controller has deleted d8-project-quota.
func (r *Reconciler) unlabelTerminating(ctx context.Context, project string) error {
	list := new(corev1.NamespaceList)
	if err := r.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelProject: project}); err != nil {
		return fmt.Errorf("list the namespaces of the project: %w", err)
	}
	var errs []error
	for i := range list.Items {
		if ns := &list.Items[i]; !ns.DeletionTimestamp.IsZero() {
			if err := r.removeLabel(ctx, ns); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// familyPlan is what the quota of a project asks of its namespaces for a family of objects: the label
// that calls the webhook of the family, and the count of its objects in the status.
type familyPlan struct {
	label bool
	count bool
}

// families is what the quota of a project asks of its namespaces for every family of objects.
type families struct {
	vm, lb familyPlan
	// balancers evaluates the load balancers with the default class of the cluster.
	balancers evaluator.LoadBalancer
}

// reconcileNamespace keeps the ResourceQuota and the labels of one namespace and finds its usage: the
// one of the ResourceQuota and the one of the objects of the families.
func (r *Reconciler) reconcileNamespace(ctx context.Context, pq *v1alpha1.ProjectQuota, ns *corev1.Namespace, rqHard corev1.ResourceList,
	plan families,
) (quota.Namespace, error) {
	found := quota.Namespace{Name: ns.Name}
	rq, err := r.ensureResourceQuota(ctx, pq, ns, rqHard)
	// ensureResourceQuota may have labelled the namespace pending
	found.Labelled = ns.Labels[quota.NamespaceLabel] == quota.LabelEnforced
	if err != nil {
		return found, err
	}
	found.Used = rq.Status.Used.DeepCopy()
	found.Calculated = quota.Calculated(rq, rqHard)
	// the webhooks of the families select the namespace before it turns enforced: the pending guard
	// holds the creates until then, and an object made between the two labels would not be checked
	if err := r.ensureFamilyLabel(ctx, ns, quota.VMLabel, plan.vm.label); err != nil {
		return found, err
	}
	if err := r.ensureFamilyLabel(ctx, ns, quota.LBLabel, plan.lb.label); err != nil {
		return found, err
	}
	if found.Labelled, err = r.ensureLabel(ctx, ns, found.Calculated); err != nil {
		return found, err
	}
	if found.Used == nil {
		found.Used = corev1.ResourceList{}
	}
	if plan.vm.count {
		usage, err := r.virtualMachineUsage(ctx, ns.Name)
		if err != nil {
			return found, err
		}
		maps.Copy(found.Used, usage)
	}
	if plan.lb.count {
		services := new(corev1.ServiceList)
		if err := r.List(ctx, services, client.InNamespace(ns.Name)); err != nil {
			return found, fmt.Errorf("list the services of the %s namespace: %w", ns.Name, err)
		}
		maps.Copy(found.Used, plan.balancers.SumLoadBalancers(services.Items))
	}
	return found, nil
}

// ensureFamilyLabel puts the label of a family on the namespace, so that the webhook of the family
// is called there, or takes it off when the quota has no keys of the family.
func (r *Reconciler) ensureFamilyLabel(ctx context.Context, ns *corev1.Namespace, label string, want bool) error {
	_, has := ns.Labels[label]
	if has == want && (!want || ns.Labels[label] == quota.LabelEnforced) {
		return nil
	}
	patch := client.MergeFrom(ns.DeepCopy())
	if want {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels[label] = quota.LabelEnforced
	} else {
		delete(ns.Labels, label)
	}
	if err := r.Patch(ctx, ns, patch); err != nil {
		return fmt.Errorf("label the %s namespace %s: %w", ns.Name, label, err)
	}
	return nil
}

// loadBalancerClasses is the evaluator of the load balancers with the MetalLoadBalancerClass objects
// of the cluster, without them when the kind is not served.
func (r *Reconciler) loadBalancerClasses(ctx context.Context) evaluator.LoadBalancer {
	classes := new(unstructured.UnstructuredList)
	classes.SetGroupVersionKind(schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "MetalLoadBalancerClassList"})
	if err := r.List(ctx, classes); err != nil {
		return evaluator.LoadBalancer{}
	}
	defaultClass, names := evaluator.ClassesOf(classes.Items)
	return evaluator.LoadBalancer{DefaultClass: defaultClass, Classes: names}
}

// virtualMachineUsage sums what the virtual machines of a namespace take, by the evaluator the ledger
// counts them with. The list goes to the API server: the controller keeps no cache of virtual machines.
func (r *Reconciler) virtualMachineUsage(ctx context.Context, namespace string) (corev1.ResourceList, error) {
	list := new(unstructured.UnstructuredList)
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: quota.VirtualMachineGroup, Version: "v1alpha2", Kind: "VirtualMachineList"})
	if err := r.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list the virtual machines of the %s namespace: %w", namespace, err)
	}
	total := corev1.ResourceList{}
	for i := range list.Items {
		usage, err := evaluator.VirtualMachine{}.Usage(&list.Items[i])
		if err != nil {
			continue
		}
		for key, value := range usage {
			sum := total[key]
			sum.Add(value)
			total[key] = sum
		}
	}
	return total, nil
}

// ensureResourceQuota creates or updates d8-project-quota in the namespace. A ResourceQuota of that
// name that the controller did not make, or one with scopes, which cannot be removed from a quota, is
// deleted and made anew: it would count only part of the objects.
func (r *Reconciler) ensureResourceQuota(ctx context.Context, pq *v1alpha1.ProjectQuota, ns *corev1.Namespace, hard corev1.ResourceList) (*corev1.ResourceQuota, error) {
	namespace := ns.Name
	rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1alpha3.ProjectQuotaName}}
	switch err := r.Get(ctx, client.ObjectKeyFromObject(rq), rq); {
	case apierrors.IsNotFound(err):
		if err := r.holdNamespace(ctx, ns); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("get the ResourceQuota of the %s namespace: %w", namespace, err)
	case foreign(rq):
		if err := r.holdNamespace(ctx, ns); err != nil {
			return nil, err
		}
		if err := r.Delete(ctx, rq, client.Preconditions{UID: &rq.UID}); err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("delete the foreign ResourceQuota of the %s namespace: %w", namespace, err)
		}
		if r.Recorder != nil {
			r.Recorder.Eventf(pq, corev1.EventTypeWarning, "ResourceQuotaReplaced",
				"namespace %s: the %s ResourceQuota was not made by the controller or had scopes; it was replaced", namespace, v1alpha3.ProjectQuotaName)
		}
		rq = &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1alpha3.ProjectQuotaName}}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rq, func() error {
		if rq.Labels == nil {
			rq.Labels = map[string]string{}
		}
		maps.Copy(rq.Labels, map[string]string{
			v1alpha3.ResourceLabelHeritage:  v1alpha3.ResourceHeritageMultitenancy,
			v1alpha3.ResourceLabelProject:   pq.Name,
			v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController,
		})
		rq.Spec.Hard = hard.DeepCopy()
		// The ProjectQuota does not own the quota: the garbage collector would delete it, also in a
		// foreground deletion, while the namespace still carries the label that binds the guard to it.
		// release() takes the label off and then deletes the quota, held by the finalizer of the
		// ProjectQuota; an owner reference an earlier release of the module set goes.
		rq.OwnerReferences = slices.DeleteFunc(rq.OwnerReferences, func(ref metav1.OwnerReference) bool {
			return ref.Kind == v1alpha1.ProjectQuotaKind && ref.APIVersion == v1alpha1.SchemeGroupVersion.String()
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("upsert the ResourceQuota of the %s namespace: %w", namespace, err)
	}
	return rq, nil
}

// foreign reports a d8-project-quota the controller did not make or one that counts only part of the
// objects.
func foreign(rq *corev1.ResourceQuota) bool {
	return rq.Labels[v1alpha3.ResourceLabelHeritage] != v1alpha3.ResourceHeritageMultitenancy ||
		len(rq.Spec.Scopes) > 0 || rq.Spec.ScopeSelector != nil
}

// ensureLabel labels the namespace enforced once its ResourceQuota is calculated, and reports whether
// it carries that label. A namespace without the label gets it only then: the guard policy would
// otherwise refuse the objects the quota counts until then. A namespace created by the controller
// carries the pending label from the start, and the guard policy holds there until the quota is ready.
func (r *Reconciler) ensureLabel(ctx context.Context, ns *corev1.Namespace, calculated bool) (bool, error) {
	if ns.Labels[quota.NamespaceLabel] == quota.LabelEnforced {
		return true, nil
	}
	if !calculated {
		return false, nil
	}
	if err := r.setLabel(ctx, ns, quota.LabelEnforced); err != nil {
		return false, err
	}
	return true, nil
}

// holdNamespace labels an enforced namespace pending before its ResourceQuota is made anew. The guard
// policy of an enforced namespace takes the ResourceQuota as its parameter and, while it is missing,
// refuses every create but the quota itself and the few objects its match constraints leave out; the
// pending label says so in the refusal and in the readiness of the namespace.
func (r *Reconciler) holdNamespace(ctx context.Context, ns *corev1.Namespace) error {
	if ns.Labels[quota.NamespaceLabel] != quota.LabelEnforced {
		return nil
	}
	return r.setLabel(ctx, ns, quota.LabelPending)
}

// removeLabel takes the quota label and the labels of the families off a namespace, whatever their
// values.
func (r *Reconciler) removeLabel(ctx context.Context, ns *corev1.Namespace) error {
	_, quotaLabel := ns.Labels[quota.NamespaceLabel]
	_, vmLabel := ns.Labels[quota.VMLabel]
	_, lbLabel := ns.Labels[quota.LBLabel]
	if !quotaLabel && !vmLabel && !lbLabel {
		return nil
	}
	patch := client.MergeFrom(ns.DeepCopy())
	delete(ns.Labels, quota.NamespaceLabel)
	delete(ns.Labels, quota.VMLabel)
	delete(ns.Labels, quota.LBLabel)
	if err := r.Patch(ctx, ns, patch); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("unlabel the %s namespace: %w", ns.Name, err)
	}
	return nil
}

func (r *Reconciler) setLabel(ctx context.Context, ns *corev1.Namespace, label string) error {
	patch := client.MergeFrom(ns.DeepCopy())
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels[quota.NamespaceLabel] = label
	if err := r.Patch(ctx, ns, patch); err != nil {
		return fmt.Errorf("label the %s namespace %s: %w", ns.Name, label, err)
	}
	return nil
}

// release takes the quota off the namespaces of a project whose ProjectQuota is going or gone. The
// label goes first: the guard policy refuses every object in a labelled namespace without a quota. The
// ResourceQuota objects of the controller are deleted here, after the label; the ProjectQuota does not
// own them, so no garbage collection, foreground deletion included, removes them first.
func (r *Reconciler) release(ctx context.Context, project string) error {
	list := new(corev1.NamespaceList)
	if err := r.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelProject: project}); err != nil {
		return fmt.Errorf("list the namespaces of the project: %w", err)
	}
	for i := range list.Items {
		ns := &list.Items[i]
		if err := r.removeLabel(ctx, ns); err != nil {
			return err
		}
		rq := new(corev1.ResourceQuota)
		switch err := r.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: v1alpha3.ProjectQuotaName}, rq); {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("get the ResourceQuota of the %s namespace: %w", ns.Name, err)
		}
		if foreign(rq) || rq.Labels[v1alpha3.ResourceLabelManagedBy] != v1alpha3.ManagedByController {
			continue
		}
		if err := r.Delete(ctx, rq, client.Preconditions{UID: &rq.UID}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the ResourceQuota of the %s namespace: %w", ns.Name, err)
		}
	}
	return nil
}

// desiredStatus is the status the quota should have.
func (r *Reconciler) desiredStatus(pq *v1alpha1.ProjectQuota, cfg quota.Config, hard corev1.ResourceList, namespaces []quota.Namespace, projectFound bool,
	notServed, notCounted []string,
) *v1alpha1.ProjectQuotaStatus {
	used := quota.Sum(hard, namespaces)
	status := &v1alpha1.ProjectQuotaStatus{
		Hard:               hard,
		Used:               used,
		NamespaceCount:     int32(len(namespaces)), //nolint:gosec // a project has far fewer namespaces than an int32 holds
		TopNamespaces:      quota.Top(hard, namespaces, v1alpha1.ProjectQuotaTopNamespaces),
		ObservedGeneration: pq.Generation,
		Conditions:         append([]metav1.Condition(nil), pq.Status.Conditions...),
	}

	enforced := quota.EnforcedCondition(quota.EnforcedInput{
		Config:       cfg,
		ProjectFound: projectFound,
		NotServed:    notServed,
		NotCounted:   notCounted,
		Namespaces:   namespaces,
	})
	conditions := []metav1.Condition{enforced}
	// the sum is complete only once every namespace has its quota calculated; until then the usage of
	// some namespace is missing from it, and Exceeded keeps what it said
	if projectFound && quota.AllCalculated(namespaces) {
		previous := quota.FindCondition(pq.Status.Conditions, v1alpha1.ProjectQuotaConditionExceeded)
		now := r.now()
		cause, causeLabel := overshootCause(cfg, enforced, quota.FindCondition(pq.Status.Conditions, v1alpha1.ProjectQuotaConditionEnforced),
			r.withinSince(pq.Name, previous, now))
		exceeded := quota.ExceededCondition(previous, pq.Status.Hard, hard, used, pq.Generation, cause)
		if exceeded.Status == metav1.ConditionFalse {
			r.mu.Lock()
			if r.lastWithin != nil {
				r.lastWithin[pq.Name] = now
			}
			r.mu.Unlock()
		}
		wasOvershoot := previous != nil && previous.Status == metav1.ConditionTrue && previous.Reason == v1alpha1.ProjectQuotaReasonOvershoot
		if exceeded.Reason == v1alpha1.ProjectQuotaReasonOvershoot && !wasOvershoot {
			overshoots.WithLabelValues(causeLabel).Inc()
		}
		conditions = append(conditions, exceeded)
	}
	for _, condition := range conditions {
		condition.ObservedGeneration = pq.Generation
		meta.SetStatusCondition(&status.Conditions, condition)
	}
	return status
}

// leaderChangeWindow is how far back a change of the ledger leader is taken for the cause of an
// overshoot when the controller does not know when the usage was last within the quota, after its
// own restart: the condition only says when the usage came within the quota, maybe weeks ago.
const leaderChangeWindow = time.Minute

// withinSince is when the usage of the project was last found within the quota: by this controller,
// or else, from the previous Exceeded condition, no earlier than leaderChangeWindow ago. Zero when the
// usage was not within the quota before.
func (r *Reconciler) withinSince(name string, previous *metav1.Condition, now time.Time) time.Time {
	r.mu.Lock()
	at, ok := r.lastWithin[name]
	r.mu.Unlock()
	if ok {
		return at
	}
	if previous == nil || previous.Status != metav1.ConditionFalse {
		return time.Time{}
	}
	if window := now.Add(-leaderChangeWindow); previous.LastTransitionTime.Time.Before(window) {
		return window
	}
	return previous.LastTransitionTime.Time
}

// overshootCause is the probable cause of a usage that grew past the quota: the sum not checked, as
// the Enforced condition says, now or at the previous evaluation; a change of the ledger leader since
// the usage was last found within the quota; or else a status write saved late or a recount of
// kube-controller-manager, which the ledger cannot see.
func overshootCause(cfg quota.Config, enforced metav1.Condition, previousEnforced *metav1.Condition, withinSince time.Time) (string, string) {
	notChecked := enforced
	if previousEnforced != nil && previousEnforced.Status != metav1.ConditionTrue {
		notChecked = *previousEnforced
	}
	if notChecked.Status != metav1.ConditionTrue {
		return "the sum of the project was not checked: " + notChecked.Reason, "not_enforced"
	}
	if !withinSince.IsZero() && cfg.LeaderChangedAt.After(withinSince) {
		return "the leader of the quota ledger changed since the usage was last found within the quota", "leader_change"
	}
	return "a quota status write saved late, or a recount of kube-controller-manager", "late_write_or_recalc"
}

func (r *Reconciler) writeStatus(ctx context.Context, pq *v1alpha1.ProjectQuota, status *v1alpha1.ProjectQuotaStatus) error {
	patch := client.MergeFrom(pq.DeepCopy())
	pq.Status = *status
	if err := r.Status().Patch(ctx, pq, patch); err != nil {
		return fmt.Errorf("write the status of the ProjectQuota: %w", err)
	}
	return nil
}

func (r *Reconciler) publishEnforcement(cfg quota.Config) {
	enforcementMode.Reset()
	enforcementMode.WithLabelValues(cfg.MetricMode()).Set(1)
}

// publishState exports the bad states of the quota and drops the ones that no longer hold.
func (r *Reconciler) publishState(name string, conditions []metav1.Condition) {
	states := map[string]bool{stateExceeded: false, stateOvershoot: false, stateNotEnforced: false}
	if c := quota.FindCondition(conditions, v1alpha1.ProjectQuotaConditionExceeded); c != nil && c.Status == metav1.ConditionTrue {
		if c.Reason == v1alpha1.ProjectQuotaReasonOvershoot {
			states[stateOvershoot] = true
		} else {
			states[stateExceeded] = true
		}
	}
	if c := quota.FindCondition(conditions, v1alpha1.ProjectQuotaConditionEnforced); c != nil && c.Status == metav1.ConditionFalse {
		switch c.Reason {
		case v1alpha1.ProjectQuotaReasonKeyRollout, v1alpha1.ProjectQuotaReasonKeyNotServed, v1alpha1.ProjectQuotaReasonProjectNotFound,
			v1alpha1.ProjectQuotaReasonWebhookMissing, v1alpha1.ProjectQuotaReasonWebhookMisconfigured, v1alpha1.ProjectQuotaReasonWebhookNotCalled:
			states[stateNotEnforced] = true
		}
	}
	for state, holds := range states {
		if holds {
			quotaState.WithLabelValues(name, state).Set(1)
		} else {
			quotaState.DeleteLabelValues(name, state)
		}
	}
}

func (r *Reconciler) remember(pq *v1alpha1.ProjectQuota) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.known == nil {
		r.known = map[string]struct{}{}
		r.lastWrite = map[string]time.Time{}
		r.lastWithin = map[string]time.Time{}
	}
	r.known[pq.Name] = struct{}{}
	quotaCount.WithLabelValues().Set(float64(len(r.known)))
}

func (r *Reconciler) forget(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.known, name)
	delete(r.lastWrite, name)
	delete(r.lastWithin, name)
	quotaCount.WithLabelValues().Set(float64(len(r.known)))
	quotaState.DeletePartialMatch(prometheus.Labels{"project": name})
}

// deletionStarted passes the update of an object that is being deleted: the release of a ProjectQuota
// must not wait for a change of its generation, which a deletion may not bump.
var deletionStarted = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool { return !e.ObjectNew.GetDeletionTimestamp().IsZero() },
}

// SetupWithManager wires the reconciler and its watches.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	byProjectLabel := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
		project := object.GetLabels()[v1alpha3.ResourceLabelProject]
		if project == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: project}}}
	})
	byName := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: object.GetName()}}}
	})
	isProjectQuotaRQ := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == v1alpha3.ProjectQuotaName
	})
	// what the enforcement of every quota depends on: the configuration, the suspension and the canary
	// of the ledger, and its webhook, whose loss or another CA is shown at once rather than at the
	// next recheck
	everyQuota := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		r.Config.Invalidate()
		list := new(v1alpha1.ProjectQuotaList)
		if err := r.List(ctx, list); err != nil {
			ctrllog.FromContext(ctx).Error(err, "list the project quotas")
			return nil
		}
		requests := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: list.Items[i].Name}})
		}
		return requests
	})
	isConfig := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return quota.IsConfigObject(object.GetNamespace(), object.GetName())
	})

	return ctrl.ControllerManagedBy(mgr).
		Named("project-quota").
		For(&v1alpha1.ProjectQuota{}, ctrl.WithPredicates(predicate.Or[client.Object](predicate.GenerationChangedPredicate{}, deletionStarted))).
		Watches(&corev1.ResourceQuota{}, byProjectLabel, ctrl.WithPredicates(isProjectQuotaRQ)).
		Watches(&corev1.Namespace{}, byProjectLabel).
		Watches(&corev1.ConfigMap{}, everyQuota, ctrl.WithPredicates(isConfig)).
		Watches(&corev1.Secret{}, everyQuota, ctrl.WithPredicates(isConfig)).
		Watches(&admissionregistrationv1.ValidatingWebhookConfiguration{}, everyQuota, ctrl.WithPredicates(isConfig)).
		Watches(&v1alpha3.Project{}, byName, ctrl.WithPredicates(predicate.Funcs{
			// the owner of the quota is set once the project is there, and the namespaces are released
			// once it goes; nothing in between concerns the quota
			UpdateFunc: func(event.UpdateEvent) bool { return false },
		})).
		Complete(r)
}
