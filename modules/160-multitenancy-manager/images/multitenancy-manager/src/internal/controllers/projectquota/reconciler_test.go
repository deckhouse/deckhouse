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

package projectquota

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
)

const projectName = "team"

type fixture struct {
	t      *testing.T
	r      *Reconciler
	client client.Client
	now    time.Time
}

func newFixture(t *testing.T, controlPlaneManaged string, objects ...client.Object) *fixture {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, coordinationv1.AddToScheme, admissionregistrationv1.AddToScheme, v1alpha1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	config := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: quota.ConfigNamespace, Name: quota.ConfigMapName},
		Data:       map[string]string{"controlPlaneManaged": controlPlaneManaged},
	}
	// the webhook of the ledger, configured with the CA of its certificate
	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: quota.LedgerWebhookName},
		Webhooks:   []admissionregistrationv1.ValidatingWebhook{ledgerWebhook()},
	}
	certs := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: quota.ConfigNamespace, Name: quota.LedgerCertsSecretName}, Data: map[string][]byte{"ca.crt": []byte("ca")}}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(objects, config, webhook, certs)...).
		WithStatusSubresource(&v1alpha1.ProjectQuota{}, &corev1.ResourceQuota{}).
		Build()
	f := &fixture{t: t, client: c, now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	f.r = &Reconciler{
		Client:   c,
		Config:   &quota.ConfigReader{Reader: c, Now: func() time.Time { return f.now }},
		Recorder: record.NewFakeRecorder(20),
		Now:      func() time.Time { return f.now },
	}
	return f
}

// ledgerWebhook is the webhook of the ledger as the module renders it, with the CA of its certificate.
func ledgerWebhook() admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name: quota.LedgerWebhookEntryName,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{CABundle: []byte("ca"), Service: &admissionregistrationv1.ServiceReference{
			Name: quota.LedgerServiceName, Namespace: quota.ConfigNamespace, Path: ptr.To(quota.LedgerWebhookPath)}},
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
			Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"resourcequotas/status"}},
		}},
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: quota.NamespaceLabel,
			Operator: metav1.LabelSelectorOpIn, Values: []string{quota.LabelPending, quota.LabelEnforced, quota.LabelCanary}}}},
		MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "the-quota-plugin-writes-the-project-quota-or-the-canary",
			Expression: quota.LedgerWebhookMatchCondition}},
	}
}

func (f *fixture) reconcile() reconcile.Result {
	f.t.Helper()
	result, err := f.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: projectName}})
	require.NoError(f.t, err)
	return result
}

func (f *fixture) quota() *v1alpha1.ProjectQuota {
	f.t.Helper()
	pq := new(v1alpha1.ProjectQuota)
	require.NoError(f.t, f.client.Get(context.Background(), client.ObjectKey{Name: projectName}, pq))
	return pq
}

func (f *fixture) resourceQuota(namespace string) *corev1.ResourceQuota {
	f.t.Helper()
	rq := new(corev1.ResourceQuota)
	require.NoError(f.t, f.client.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: v1alpha3.ProjectQuotaName}, rq))
	return rq
}

func (f *fixture) namespace(name string) *corev1.Namespace {
	f.t.Helper()
	ns := new(corev1.Namespace)
	require.NoError(f.t, f.client.Get(context.Background(), client.ObjectKey{Name: name}, ns))
	return ns
}

// count plays the resource quota controller of kube-controller-manager: it copies spec.hard into
// status.hard and sets the usage.
func (f *fixture) count(namespace string, used corev1.ResourceList) {
	f.t.Helper()
	rq := f.resourceQuota(namespace)
	rq.Status.Hard = rq.Spec.Hard.DeepCopy()
	rq.Status.Used = used
	require.NoError(f.t, f.client.Status().Update(context.Background(), rq))
}

func rl(pairs ...string) corev1.ResourceList {
	out := corev1.ResourceList{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[corev1.ResourceName(pairs[i])] = resource.MustParse(pairs[i+1])
	}
	return out
}

func project() *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: "project-uid"}}
}

func projectNamespace(name string, labels map[string]string) *corev1.Namespace {
	all := map[string]string{v1alpha3.ResourceLabelProject: projectName}
	for k, v := range labels {
		all[k] = v
	}
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: all}}
}

func projectQuota(hard corev1.ResourceList) *v1alpha1.ProjectQuota {
	return &v1alpha1.ProjectQuota{
		ObjectMeta: metav1.ObjectMeta{Name: projectName, UID: "quota-uid", Generation: 1},
		Spec:       v1alpha1.ProjectQuotaSpec{Hard: hard},
	}
}

func condition(pq *v1alpha1.ProjectQuota, conditionType string) metav1.Condition {
	c := quota.FindCondition(pq.Status.Conditions, conditionType)
	if c == nil {
		return metav1.Condition{}
	}
	return *c
}

func TestReconcilePutsAResourceQuotaInEveryNamespace(t *testing.T) {
	f := newFixture(t, "true",
		project(),
		projectNamespace(projectName, nil),
		projectNamespace(projectName+"-db", nil),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
		projectQuota(rl("requests.cpu", "4", "projectNamespaces", "2")),
	)
	f.reconcile()

	for _, ns := range []string{projectName, projectName + "-db"} {
		rq := f.resourceQuota(ns)
		assert.Equal(t, rl("requests.cpu", "4", string(quota.ProjectNamespacesKey), "2"), rq.Spec.Hard, "the alias is expanded")
		assert.Empty(t, rq.Spec.Scopes)
		assert.Empty(t, rq.Finalizers)
		assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, rq.Labels[v1alpha3.ResourceLabelHeritage])
		assert.Equal(t, projectName, rq.Labels[v1alpha3.ResourceLabelProject])
		assert.Empty(t, rq.OwnerReferences, "the garbage collector does not delete it before the label goes")
	}
	err := f.client.Get(context.Background(), client.ObjectKey{Namespace: "other", Name: v1alpha3.ProjectQuotaName}, new(corev1.ResourceQuota))
	assert.True(t, apierrors.IsNotFound(err), "a namespace of no project gets nothing")

	pq := f.quota()
	require.Len(t, pq.OwnerReferences, 1, "the project owns the quota")
	assert.Equal(t, types.UID("project-uid"), pq.OwnerReferences[0].UID)
	assert.Equal(t, rl("requests.cpu", "4", string(quota.ProjectNamespacesKey), "2"), pq.Status.Hard)
	assert.Equal(t, int32(2), pq.Status.NamespaceCount)
}

func TestReconcileLabelsANamespaceOnceItsQuotaIsCalculated(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "10")))

	f.reconcile()
	assert.NotContains(t, f.namespace(projectName).Labels, quota.NamespaceLabel, "the guard policy would refuse pods until the quota is calculated")
	enforced := condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced)
	assert.Equal(t, metav1.ConditionFalse, enforced.Status)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonKeyRollout, enforced.Reason)

	f.count(projectName, rl("pods", "1"))
	f.reconcile()
	assert.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])
	enforced = condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced)
	assert.Equal(t, metav1.ConditionTrue, enforced.Status)
}

// A namespace created for a ProjectNamespace is labelled pending at birth and stays so until its quota
// is calculated.
func TestReconcileEnforcesANamespaceLabelledPendingAtBirth(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelPending}),
		projectQuota(rl("pods", "10")))
	f.reconcile()
	assert.Equal(t, quota.LabelPending, f.namespace(projectName).Labels[quota.NamespaceLabel])
	assert.Equal(t, v1alpha1.ProjectQuotaReasonKeyRollout, condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced).Reason)

	f.count(projectName, rl("pods", "0"))
	f.reconcile()
	assert.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])
	assert.Equal(t, metav1.ConditionTrue, condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced).Status)
}

// A quota without limits for a ResourceQuota never gets status.hard from the resource quota controller.
// Its namespaces are enforced at once, or a namespace labelled pending at birth would refuse every
// object for good.
func TestReconcileEnforcesAQuotaWithoutResourceQuotaLimits(t *testing.T) {
	f := newFixture(t, "true", project(),
		projectNamespace(projectName, nil),
		projectNamespace(projectName+"-db", map[string]string{quota.NamespaceLabel: quota.LabelPending}),
		projectQuota(corev1.ResourceList{}))
	f.reconcile()

	for _, ns := range []string{projectName, projectName + "-db"} {
		assert.Empty(t, f.resourceQuota(ns).Spec.Hard)
		assert.Equal(t, quota.LabelEnforced, f.namespace(ns).Labels[quota.NamespaceLabel], ns)
	}
	assert.Equal(t, metav1.ConditionTrue, condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced).Status)
}

// A quota an earlier release of the module made carries an owner reference to the ProjectQuota, with
// which a foreground deletion of the ProjectQuota or of its project lets the garbage collector delete the
// quota while the namespace is still labelled. The reference goes at the next reconcile.
func TestReconcileDropsTheOwnerReferenceOfTheQuota(t *testing.T) {
	owned := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: projectName, Name: v1alpha3.ProjectQuotaName,
		Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy, v1alpha3.ResourceLabelProject: projectName,
			v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController},
		OwnerReferences: []metav1.OwnerReference{
			{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ProjectQuotaKind, Name: projectName, UID: "quota-uid",
				Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(true)},
			{APIVersion: "v1", Kind: "ConfigMap", Name: "other", UID: "other-uid"},
		}}}
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), owned, projectQuota(rl("pods", "10")))
	f.reconcile()

	refs := f.resourceQuota(projectName).OwnerReferences
	require.Len(t, refs, 1, "only the reference to the ProjectQuota goes")
	assert.Equal(t, "ConfigMap", refs[0].Kind)
}

// The guard policy of an enforced namespace takes its d8-project-quota as the parameter and refuses
// every create, the quota itself included, while the quota is missing; the controller labels the
// namespace pending before it makes the quota again.
func TestReconcileHoldsAnEnforcedNamespaceWhoseQuotaIsMissing(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}),
		projectQuota(rl("pods", "10")))
	var labelAtCreate string
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.ResourceQuota); ok {
				labelAtCreate = f.namespace(obj.GetNamespace()).Labels[quota.NamespaceLabel]
			}
			return c.Create(ctx, obj, opts...)
		},
	})
	f.reconcile()
	assert.Equal(t, quota.LabelPending, labelAtCreate)
	f.resourceQuota(projectName)

	f.count(projectName, rl("pods", "0"))
	f.reconcile()
	assert.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])
}

// A foreign quota of an enforced namespace is replaced: the namespace is labelled pending before the
// quota is deleted.
func TestReconcileHoldsTheNamespaceBeforeItDeletesAForeignQuota(t *testing.T) {
	foreignRQ := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: projectName, Name: v1alpha3.ProjectQuotaName, UID: "foreign"},
		Spec: corev1.ResourceQuotaSpec{Hard: rl("pods", "1000")}}
	f := newFixture(t, "true", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}), foreignRQ,
		projectQuota(rl("pods", "10")))
	var calls []string
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if ns, ok := obj.(*corev1.Namespace); ok {
				calls = append(calls, "label "+ns.Labels[quota.NamespaceLabel])
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			calls = append(calls, "delete "+string(obj.GetUID()))
			return c.Delete(ctx, obj, opts...)
		},
	})
	f.reconcile()
	assert.Equal(t, []string{"label pending", "delete foreign"}, calls)
	assert.Equal(t, quota.LabelPending, f.namespace(projectName).Labels[quota.NamespaceLabel])
}

// A namespace whose quota the controller cannot make is reported in the status, and the other
// namespaces are still reconciled.
func TestReconcileReportsANamespaceWithoutItsQuota(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil),
		projectNamespace(projectName+"-stuck", map[string]string{quota.NamespaceLabel: quota.LabelPending}), projectQuota(rl("pods", "10")))
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetNamespace() == projectName+"-stuck" {
				return apierrors.NewForbidden(corev1.Resource("resourcequotas"), obj.GetName(), errors.New("denied by a policy"))
			}
			return c.Create(ctx, obj, opts...)
		},
	})
	_, err := f.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: projectName}})
	require.Error(t, err)
	f.resourceQuota(projectName)

	enforced := condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced)
	assert.Equal(t, metav1.ConditionFalse, enforced.Status)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonKeyRollout, enforced.Reason)
	assert.Contains(t, enforced.Message, projectName+"-stuck: ")
	assert.Contains(t, enforced.Message, "denied by a policy")
}

// Where Deckhouse does not manage the control plane, the quotas are kept in every namespace and the
// sum is shown, but it is not enforced, and the quota says so.
func TestReconcileOnAControlPlaneNotManaged(t *testing.T) {
	f := newFixture(t, "false", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}),
		projectQuota(rl("pods", "10")))
	f.reconcile()
	f.count(projectName, rl("pods", "0"))
	f.reconcile()
	assert.Equal(t, rl("pods", "10"), f.resourceQuota(projectName).Spec.Hard)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonControlPlaneNotManaged, condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced).Reason)
}

func TestReconcileReplacesAForeignResourceQuota(t *testing.T) {
	tests := []struct {
		name string
		rq   *corev1.ResourceQuota
	}{
		{name: "without the heritage label", rq: &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: projectName, Name: v1alpha3.ProjectQuotaName, UID: "foreign"},
			Spec: corev1.ResourceQuotaSpec{Hard: rl("pods", "1000")}}},
		{name: "with scopes", rq: &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: projectName, Name: v1alpha3.ProjectQuotaName, UID: "foreign",
			Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}},
			Spec: corev1.ResourceQuotaSpec{Hard: rl("pods", "10"), Scopes: []corev1.ResourceQuotaScope{corev1.ResourceQuotaScopeBestEffort}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "true", project(), projectNamespace(projectName, nil), tt.rq, projectQuota(rl("pods", "10")))
			f.reconcile()
			rq := f.resourceQuota(projectName)
			assert.NotEqual(t, types.UID("foreign"), rq.UID)
			assert.Empty(t, rq.Spec.Scopes)
			assert.Equal(t, rl("pods", "10"), rq.Spec.Hard)
		})
	}
}

func TestReconcileSumsTheUsage(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectNamespace(projectName+"-a", nil),
		projectQuota(rl("pods", "10")))
	f.reconcile()
	f.count(projectName, rl("pods", "3"))
	f.count(projectName+"-a", rl("pods", "3"))
	f.reconcile()
	require.Equal(t, v1alpha1.ProjectQuotaReasonWithinLimits, condition(f.quota(), v1alpha1.ProjectQuotaConditionExceeded).Reason)

	f.count(projectName, rl("pods", "6"))
	f.count(projectName+"-a", rl("pods", "6"))
	f.now = f.now.Add(time.Minute)
	f.reconcile()

	pq := f.quota()
	used := pq.Status.Used[corev1.ResourcePods]
	assert.Equal(t, int64(12), used.Value())
	require.Len(t, pq.Status.TopNamespaces, 2)
	exceeded := condition(pq, v1alpha1.ProjectQuotaConditionExceeded)
	assert.Equal(t, metav1.ConditionTrue, exceeded.Status)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonOvershoot, exceeded.Reason, "the usage grew past the quota after it was counted within it")
}

func TestReconcileFindsAQuotaCreatedBelowTheUsage(t *testing.T) {
	main := projectNamespace(projectName, nil)
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Namespace: projectName, Name: v1alpha3.ProjectQuotaName,
			Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}},
		Spec:   corev1.ResourceQuotaSpec{Hard: rl("pods", "10")},
		Status: corev1.ResourceQuotaStatus{Hard: rl("pods", "10"), Used: rl("pods", "15")},
	}
	f := newFixture(t, "true", project(), main, rq, projectQuota(rl("pods", "10")))
	f.reconcile()
	exceeded := condition(f.quota(), v1alpha1.ProjectQuotaConditionExceeded)
	assert.Equal(t, metav1.ConditionTrue, exceeded.Status)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonHardLowered, exceeded.Reason, "the quota was created below the usage")
}

// The usage counted before every namespace has its quota calculated is not the usage of the project:
// a quota whose namespaces were above the limit before it applied, as after an upgrade, is found
// above it, not overshot.
func TestReconcileDoesNotJudgeAnIncompleteSum(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "1")))
	f.reconcile()
	assert.Nil(t, quota.FindCondition(f.quota().Status.Conditions, v1alpha1.ProjectQuotaConditionExceeded))

	f.count(projectName, rl("pods", "3"))
	f.now = f.now.Add(time.Minute)
	f.reconcile()
	exceeded := condition(f.quota(), v1alpha1.ProjectQuotaConditionExceeded)
	assert.Equal(t, metav1.ConditionTrue, exceeded.Status)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonHardLowered, exceeded.Reason)
}

func TestReconcileWritesASmallMoveLater(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "1000")))
	f.reconcile()
	f.count(projectName, rl("pods", "100"))
	f.now = f.now.Add(time.Minute)
	f.reconcile()
	require.Equal(t, int64(100), f.quota().Status.Used.Pods().Value())

	f.count(projectName, rl("pods", "200"))
	f.now = f.now.Add(time.Second)
	result := f.reconcile()
	assert.Equal(t, int64(100), f.quota().Status.Used.Pods().Value(), "a write of the usage waits for the interval")
	assert.Equal(t, quota.StatusWriteInterval-time.Second, result.RequeueAfter)

	f.now = f.now.Add(result.RequeueAfter)
	f.reconcile()
	assert.Equal(t, int64(200), f.quota().Status.Used.Pods().Value())
}

func TestReconcileReleasesTheNamespacesOfADeletedQuota(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "10")))
	f.reconcile()
	f.count(projectName, rl("pods", "0"))
	f.reconcile()
	require.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])

	require.NoError(t, f.client.Delete(context.Background(), f.quota()))
	f.reconcile()
	assert.NotContains(t, f.namespace(projectName).Labels, quota.NamespaceLabel)
	err := f.client.Get(context.Background(), client.ObjectKey{Namespace: projectName, Name: v1alpha3.ProjectQuotaName}, new(corev1.ResourceQuota))
	assert.True(t, apierrors.IsNotFound(err))
}

// terminatingNamespace is a namespace of the project the namespace controller is deleting.
func terminatingNamespace(name string, labels map[string]string) *corev1.Namespace {
	ns := projectNamespace(name, labels)
	now := metav1.NewTime(time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC))
	ns.DeletionTimestamp = &now
	ns.Finalizers = []string{"kubernetes"}
	return ns
}

// The namespace controller deletes d8-project-quota with the namespace, and the claims behind
// pvc-protection are updated after that: the guard policies refuse those updates for as long as the
// namespace carries the label, so the label comes off a namespace that is going, and nothing else is
// written in it.
func TestReconcileUnlabelsTerminatingNamespaces(t *testing.T) {
	f := newFixture(t, "true", project(),
		projectNamespace(projectName, nil),
		terminatingNamespace(projectName+"-enforced", map[string]string{quota.NamespaceLabel: quota.LabelEnforced}),
		terminatingNamespace(projectName+"-pending", map[string]string{quota.NamespaceLabel: quota.LabelPending}),
		terminatingNamespace(projectName+"-other", map[string]string{quota.NamespaceLabel: "other"}),
		terminatingNamespace(projectName+"-plain", nil),
		projectQuota(rl("pods", "10")))
	var writes []string
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			writes = append(writes, "create "+obj.GetNamespace()+"/"+obj.GetName())
			return c.Create(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				writes = append(writes, "patch "+obj.GetName())
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			writes = append(writes, "delete "+obj.GetNamespace()+"/"+obj.GetName())
			return c.Delete(ctx, obj, opts...)
		},
	})
	f.reconcile()

	for _, name := range []string{"enforced", "pending", "other", "plain"} {
		assert.NotContains(t, f.namespace(projectName+"-"+name).Labels, quota.NamespaceLabel, name)
		err := f.client.Get(context.Background(), client.ObjectKey{Namespace: projectName + "-" + name, Name: v1alpha3.ProjectQuotaName}, new(corev1.ResourceQuota))
		assert.True(t, apierrors.IsNotFound(err), "no quota is made in the %s namespace that is going", name)
	}
	assert.ElementsMatch(t, []string{
		"create " + projectName + "/" + v1alpha3.ProjectQuotaName,
		"patch " + projectName + "-enforced",
		"patch " + projectName + "-pending",
		"patch " + projectName + "-other",
	}, writes, "the namespace without the label is not written, and neither is a quota")
	assert.Equal(t, int32(1), f.quota().Status.NamespaceCount, "a namespace that is going is not counted")
}

// A quota whose project is gone, deleted without its namespaces or never owning them, still takes the
// label off the namespaces of the project that are going, and leaves the others as they are.
func TestReconcileUnlabelsTerminatingNamespacesWithoutAProject(t *testing.T) {
	f := newFixture(t, "true",
		projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}),
		terminatingNamespace(projectName+"-going", map[string]string{quota.NamespaceLabel: quota.LabelEnforced}),
		projectQuota(rl("pods", "10")))
	f.reconcile()

	assert.NotContains(t, f.namespace(projectName+"-going").Labels, quota.NamespaceLabel)
	assert.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])
}

// The release of a quota that is deleted must not wait for a change of its generation.
func TestDeletionStartedEnqueuesATerminatingQuota(t *testing.T) {
	live := projectQuota(rl("pods", "10"))
	going := live.DeepCopy()
	now := metav1.Now()
	going.DeletionTimestamp = &now

	assert.False(t, deletionStarted.Update(event.UpdateEvent{ObjectOld: live, ObjectNew: live}))
	assert.True(t, deletionStarted.Update(event.UpdateEvent{ObjectOld: live, ObjectNew: going}))
}

func TestReconcileAddsTheFinalizerToTheQuota(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "10")))
	f.reconcile()
	assert.Equal(t, []string{v1alpha1.ProjectQuotaFinalizer}, f.quota().Finalizers)

	f.reconcile()
	assert.Equal(t, []string{v1alpha1.ProjectQuotaFinalizer}, f.quota().Finalizers, "it is added once")
}

// The garbage collector deletes the ResourceQuota objects a ProjectQuota owns once it is gone; the
// finalizer keeps it until the labels are off, so that no namespace is left labelled without its quota.
func TestReconcileReleasesTheNamespacesBeforeTheQuotaGoes(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectNamespace(projectName+"-db", nil), projectQuota(rl("pods", "10")))
	f.reconcile()
	f.count(projectName, rl("pods", "0"))
	f.count(projectName+"-db", rl("pods", "0"))
	f.reconcile()
	require.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])

	var calls []string
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			switch obj.(type) {
			case *corev1.Namespace:
				calls = append(calls, "unlabel "+obj.GetName())
			case *v1alpha1.ProjectQuota:
				calls = append(calls, "finalizer")
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			calls = append(calls, "delete "+obj.GetNamespace()+"/"+obj.GetName())
			return c.Delete(ctx, obj, opts...)
		},
	})
	require.NoError(t, f.client.Delete(context.Background(), f.quota()))
	assert.Equal(t, []string{v1alpha1.ProjectQuotaFinalizer}, f.quota().Finalizers, "the quota waits for the controller")
	assert.False(t, f.quota().DeletionTimestamp.IsZero())

	f.reconcile()
	require.NotEmpty(t, calls)
	assert.Equal(t, "finalizer", calls[len(calls)-1], "the finalizer goes last")
	assert.Equal(t, 1, countOf(calls, "finalizer"))
	for _, ns := range []string{projectName, projectName + "-db"} {
		assert.NotContains(t, f.namespace(ns).Labels, quota.NamespaceLabel, ns)
		assert.Less(t, indexOf(calls, "unlabel "+ns), indexOf(calls, "finalizer"), "the label of %s goes before the quota", ns)
		assert.Less(t, indexOf(calls, "unlabel "+ns), indexOf(calls, "delete "+ns+"/"+v1alpha3.ProjectQuotaName), "and before its ResourceQuota", ns)
	}
	err := f.client.Get(context.Background(), client.ObjectKey{Name: projectName}, new(v1alpha1.ProjectQuota))
	assert.True(t, apierrors.IsNotFound(err), "the quota is gone once the finalizer is")
}

// A quota whose project is gone is released the same way.
func TestReconcileReleasesAQuotaWhoseProjectIsGone(t *testing.T) {
	pq := projectQuota(rl("pods", "10"))
	pq.Finalizers = []string{v1alpha1.ProjectQuotaFinalizer}
	f := newFixture(t, "true", projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}), pq)
	require.NoError(t, f.client.Delete(context.Background(), f.quota()))

	f.reconcile()
	assert.NotContains(t, f.namespace(projectName).Labels, quota.NamespaceLabel)
	err := f.client.Get(context.Background(), client.ObjectKey{Name: projectName}, new(v1alpha1.ProjectQuota))
	assert.True(t, apierrors.IsNotFound(err))
}

// A release that fails keeps the quota, so that it is tried again.
func TestReconcileKeepsTheQuotaWhileTheLabelStays(t *testing.T) {
	pq := projectQuota(rl("pods", "10"))
	pq.Finalizers = []string{v1alpha1.ProjectQuotaFinalizer}
	f := newFixture(t, "true", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}), pq)
	require.NoError(t, f.client.Delete(context.Background(), f.quota()))
	f.r.Client = interceptor.NewClient(f.client.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				return apierrors.NewInternalError(errors.New("etcd is slow"))
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	})

	_, err := f.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: projectName}})
	require.Error(t, err)
	assert.Equal(t, []string{v1alpha1.ProjectQuotaFinalizer}, f.quota().Finalizers)
	assert.Equal(t, quota.LabelEnforced, f.namespace(projectName).Labels[quota.NamespaceLabel])
}

func countOf(calls []string, call string) int {
	n := 0
	for _, c := range calls {
		if c == call {
			n++
		}
	}
	return n
}

// indexOf is the position of the call, or the length of the list when it was not made, which fails a
// comparison with a call that was.
func indexOf(calls []string, call string) int {
	for i, c := range calls {
		if c == call {
			return i
		}
	}
	return len(calls)
}

func TestReconcileWithoutTheProject(t *testing.T) {
	f := newFixture(t, "true", projectQuota(rl("pods", "10")))
	f.reconcile()
	pq := f.quota()
	assert.Empty(t, pq.OwnerReferences)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonProjectNotFound, condition(pq, v1alpha1.ProjectQuotaConditionEnforced).Reason)
}

func TestReconcileReportsTheStateOfTheLedger(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(f *fixture)
		reason string
	}{
		{name: "suspended", reason: v1alpha1.ProjectQuotaReasonSuspended, edit: func(f *fixture) {
			require.NoError(t, f.client.Create(context.Background(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: quota.ConfigNamespace, Name: quota.SuspendConfigMapName}}))
		}},
		{name: "a webhook with another CA", reason: v1alpha1.ProjectQuotaReasonWebhookMissing, edit: func(f *fixture) {
			certs := new(corev1.Secret)
			require.NoError(t, f.client.Get(context.Background(), client.ObjectKey{Namespace: quota.ConfigNamespace, Name: quota.LedgerCertsSecretName}, certs))
			certs.Data["ca.crt"] = []byte("another")
			require.NoError(t, f.client.Update(context.Background(), certs))
		}},
		{name: "a webhook the API servers do not call", reason: v1alpha1.ProjectQuotaReasonWebhookNotCalled, edit: func(f *fixture) {
			require.NoError(t, f.client.Create(context.Background(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: quota.ConfigNamespace, Name: quota.LedgerStateConfigMapName},
				Data: map[string]string{quota.LedgerStateNotCalledKey: "10.0.0.1:6443"}}))
		}},
		{name: "a webhook that leaves the quota namespaces out", reason: v1alpha1.ProjectQuotaReasonWebhookMisconfigured, edit: func(f *fixture) {
			webhook := new(admissionregistrationv1.ValidatingWebhookConfiguration)
			require.NoError(t, f.client.Get(context.Background(), client.ObjectKey{Name: quota.LedgerWebhookName}, webhook))
			webhook.Webhooks[0].NamespaceSelector.MatchExpressions[0].Values = []string{quota.LabelCanary}
			require.NoError(t, f.client.Update(context.Background(), webhook))
		}},
		{name: "no webhook", reason: v1alpha1.ProjectQuotaReasonWebhookMissing, edit: func(f *fixture) {
			require.NoError(t, f.client.Delete(context.Background(), &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: quota.LedgerWebhookName}}))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "true", project(), projectNamespace(projectName, map[string]string{quota.NamespaceLabel: quota.LabelEnforced}), projectQuota(rl("pods", "10")))
			tt.edit(f)
			f.reconcile()
			f.count(projectName, rl("pods", "0"))
			f.reconcile()
			assert.Equal(t, tt.reason, condition(f.quota(), v1alpha1.ProjectQuotaConditionEnforced).Reason)
		})
	}
}

func TestOvershootCause(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	enforced := metav1.Condition{Status: metav1.ConditionTrue, Reason: v1alpha1.ProjectQuotaReasonEnforced}
	notEnforced := metav1.Condition{Status: metav1.ConditionFalse, Reason: v1alpha1.ProjectQuotaReasonWebhookMissing}
	within := now.Add(-10 * time.Second)

	_, label := overshootCause(quota.Config{LeaderChangedAt: now.Add(-5 * time.Second)}, enforced, &enforced, within)
	assert.Equal(t, "leader_change", label, "the leader changed since the usage was within the quota")
	_, label = overshootCause(quota.Config{LeaderChangedAt: now.Add(-time.Minute)}, enforced, &enforced, within)
	assert.Equal(t, "late_write_or_recalc", label, "the leader changed before the usage was last within the quota")
	_, label = overshootCause(quota.Config{}, enforced, nil, within)
	assert.Equal(t, "late_write_or_recalc", label)
	_, label = overshootCause(quota.Config{LeaderChangedAt: now.Add(-5 * time.Second)}, enforced, nil, time.Time{})
	assert.Equal(t, "late_write_or_recalc", label, "the usage was never within the quota")

	cause, label := overshootCause(quota.Config{LeaderChangedAt: now.Add(-time.Minute)}, notEnforced, &enforced, within)
	assert.Equal(t, "not_enforced", label)
	assert.Equal(t, "the sum of the project was not checked: WebhookMissing", cause)
	_, label = overshootCause(quota.Config{}, enforced, &notEnforced, within)
	assert.Equal(t, "not_enforced", label, "the sum was not checked until the previous evaluation")
}

// A quota that has been within its limits for weeks is not taken for one the leader change of last
// week pushed over: the controller remembers when it last found the usage within the quota, and after
// its own restart takes only a change of the leader within the last minute.
func TestWithinSince(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	weeksAgo := &metav1.Condition{Status: metav1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-21 * 24 * time.Hour))}
	r := &Reconciler{}
	r.remember(projectQuota(rl("pods", "10")))

	assert.Equal(t, now.Add(-leaderChangeWindow), r.withinSince(projectName, weeksAgo, now), "after a restart, the window")
	recent := &metav1.Condition{Status: metav1.ConditionFalse, LastTransitionTime: metav1.NewTime(now.Add(-10 * time.Second))}
	assert.Equal(t, now.Add(-10*time.Second), r.withinSince(projectName, recent, now))
	assert.True(t, r.withinSince(projectName, nil, now).IsZero())

	r.lastWithin[projectName] = now.Add(-2 * time.Second)
	assert.Equal(t, now.Add(-2*time.Second), r.withinSince(projectName, weeksAgo, now), "the last evaluation within the quota")
}

// The usage that grew while the webhook of the ledger was missing is an overshoot whose cause says
// the sum was not checked.
func TestReconcileTellsAnOvershootNotChecked(t *testing.T) {
	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "5")))
	f.reconcile()
	f.count(projectName, rl("pods", "3"))
	f.reconcile()
	require.NoError(t, f.client.Delete(context.Background(), &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: quota.LedgerWebhookName}}))
	f.count(projectName, rl("pods", "8"))
	f.now = f.now.Add(time.Minute)
	f.reconcile()
	exceeded := condition(f.quota(), v1alpha1.ProjectQuotaConditionExceeded)
	assert.Equal(t, v1alpha1.ProjectQuotaReasonOvershoot, exceeded.Reason)
	assert.Contains(t, exceeded.Message, "Probable cause: the sum of the project was not checked: WebhookMissing.")
}

// The replica of the controller that does not lead reconciles nothing and exports no series of the
// quota, rather than a zero that reads as "no quotas" or "the webhook is missing".
func TestOnlyTheLeaderExportsTheQuotaSeries(t *testing.T) {
	quotaCount.Reset()
	enforcementMode.Reset()
	assert.Zero(t, testutil.CollectAndCount(quotaCount))
	assert.Zero(t, testutil.CollectAndCount(enforcementMode))

	f := newFixture(t, "true", project(), projectNamespace(projectName, nil), projectQuota(rl("pods", "10")))
	f.reconcile()
	assert.Equal(t, 1, testutil.CollectAndCount(quotaCount))
	assert.InDelta(t, 1, testutil.ToFloat64(enforcementMode.WithLabelValues("Enforce")), 0)
}
