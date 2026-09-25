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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/naming"
)

// TestCatalogFieldsValidCondition: a valid declaration is True, one stored past the webhook is False
// with the webhook's problem text, and fixing it turns the condition True again. A definition without
// catalogFields carries the condition too.
func TestCatalogFieldsValidCondition(t *testing.T) {
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	def.Generation = 1
	def.Spec.CatalogFields = []v1alpha1.CatalogField{{Name: "provisioner", Path: "$.provisioner"}}
	plain := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "plain", Generation: 1}}
	cl := buildClient(t, def, plain)
	ctx := context.Background()
	rec := &DefinitionReconciler{Client: cl, Factory: jsonpath.NewWithCache(), Mapper: testMapper()}

	condition := func(name string) *metav1.Condition {
		t.Helper()
		if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
			t.Fatalf("reconcile %s: %v", name, err)
		}
		got := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, got); err != nil {
			t.Fatal(err)
		}
		c := meta.FindStatusCondition(got.Status.Conditions, "CatalogFieldsValid")
		if c == nil {
			t.Fatalf("%s has no CatalogFieldsValid condition: %+v", name, got.Status.Conditions)
		}
		if c.ObservedGeneration != got.Generation {
			t.Fatalf("%s: observedGeneration %d, want %d", name, c.ObservedGeneration, got.Generation)
		}
		return c
	}
	update := func(mutate func(*v1alpha1.GrantableClusterResourceDefinition)) {
		t.Helper()
		got := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := cl.Get(ctx, types.NamespacedName{Name: "storageclasses"}, got); err != nil {
			t.Fatal(err)
		}
		mutate(got)
		if err := cl.Update(ctx, got); err != nil {
			t.Fatal(err)
		}
	}

	if c := condition("plain"); c.Status != metav1.ConditionTrue || c.Reason != "Valid" || c.Message != "No catalogFields are declared." {
		t.Fatalf("plain: %+v", c)
	}
	if c := condition("storageclasses"); c.Status != metav1.ConditionTrue || c.Reason != "Valid" || c.Message != "All catalogFields are valid." {
		t.Fatalf("valid: %+v", c)
	}

	// Stored past the webhook: a path that does not compile and one into managedFields.
	update(func(d *v1alpha1.GrantableClusterResourceDefinition) {
		d.Spec.CatalogFields = append(d.Spec.CatalogFields,
			v1alpha1.CatalogField{Name: "broken", Path: "$.foo-bar"},
			v1alpha1.CatalogField{Name: "managed", Path: "$.metadata.managedFields"})
	})
	c := condition("storageclasses")
	if c.Status != metav1.ConditionFalse || c.Reason != "InvalidCatalogFields" {
		t.Fatalf("invalid: %+v", c)
	}
	got := &v1alpha1.GrantableClusterResourceDefinition{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "storageclasses"}, got); err != nil {
		t.Fatal(err)
	}
	// The same text as the webhook's refusal, without its prefix.
	if want := strings.Join(engine.DefinitionProblems(jsonpath.NewWithCache(), got), "; "); c.Message != want {
		t.Fatalf("invalid: message %q, want %q", c.Message, want)
	}
	for _, want := range []string{`'spec.catalogFields[1].path' "$.foo-bar"`, `'spec.catalogFields[2].path' "$.metadata.managedFields"`} {
		if !strings.Contains(c.Message, want) {
			t.Fatalf("invalid: message %q lacks %q", c.Message, want)
		}
	}

	update(func(d *v1alpha1.GrantableClusterResourceDefinition) { d.Spec.CatalogFields = d.Spec.CatalogFields[:1] })
	if c := condition("storageclasses"); c.Status != metav1.ConditionTrue || c.Reason != "Valid" {
		t.Fatalf("fixed: %+v", c)
	}
}

// TestGrantedResourceValidCondition: the condition says whether spec.grantedResource resolves; a kind
// the mapper does not know is Unknown. Every answer is requeued, so a removed CRD turns True to Unknown.
func TestGrantedResourceValidCondition(t *testing.T) {
	named := func(name, group, kind string) *v1alpha1.GrantableClusterResourceDefinition {
		def := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name, Generation: 1}}
		if kind != "" {
			def.Spec.GrantedResource = &v1alpha1.GrantedResource{APIGroup: group, Kind: kind}
		}
		return def
	}
	defs := []*v1alpha1.GrantableClusterResourceDefinition{
		named("storageclasses", "storage.k8s.io", "StorageClass"),
		named("values", "", ""),
		named("secrets", "", "Secret"),
		named("widgets", "example.com", "Widget"),
	}
	objs := make([]client.Object, 0, len(defs))
	for _, d := range defs {
		objs = append(objs, d)
	}
	cl := buildClient(t, objs...)
	rec := &DefinitionReconciler{Client: cl, Factory: jsonpath.NewWithCache(), Mapper: testMapper()}
	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		status  metav1.ConditionStatus
		reason  string
		message string
	}{
		{"storageclasses", metav1.ConditionTrue, "ClusterScoped", "grantedResource StorageClass.storage.k8s.io is cluster-scoped."},
		{"values", metav1.ConditionTrue, "ValueBacked", "No grantedResource is set: the definition is value-backed."},
		{"secrets", metav1.ConditionFalse, "Namespaced", "'spec.grantedResource' Secret is namespaced: only cluster-scoped resources can be granted. "},
		{"widgets", metav1.ConditionUnknown, "KindNotServed", `no matches for kind "Widget"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.name}})
			if err != nil {
				t.Fatal(err)
			}
			if res.RequeueAfter != ResyncInterval {
				t.Fatalf("RequeueAfter = %v, want %v", res.RequeueAfter, ResyncInterval)
			}
			got := &v1alpha1.GrantableClusterResourceDefinition{}
			if err := cl.Get(ctx, types.NamespacedName{Name: tc.name}, got); err != nil {
				t.Fatal(err)
			}
			c := meta.FindStatusCondition(got.Status.Conditions, "GrantedResourceValid")
			if c == nil || c.Status != tc.status || c.Reason != tc.reason || !strings.Contains(c.Message, tc.message) || c.ObservedGeneration != 1 {
				t.Fatalf("condition %+v", c)
			}
			// The rest of the status is written whatever the scope.
			if meta.FindStatusCondition(got.Status.Conditions, "CatalogFieldsValid") == nil || got.Status.ObservedGeneration != 1 {
				t.Fatalf("status %+v", got.Status)
			}
		})
	}

	t.Run("the CRD of a cluster-scoped kind is removed", func(t *testing.T) {
		rec.Mapper = meta.NewDefaultRESTMapper(nil)
		if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "storageclasses"}}); err != nil {
			t.Fatal(err)
		}
		got := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := cl.Get(ctx, types.NamespacedName{Name: "storageclasses"}, got); err != nil {
			t.Fatal(err)
		}
		if c := meta.FindStatusCondition(got.Status.Conditions, "GrantedResourceValid"); c == nil || c.Reason != "KindNotServed" {
			t.Fatalf("condition %+v", c)
		}
	})
}

// TestGrantedResourceValidCondition_Rescoped: with the grants REST mapper, a kind re-created as
// namespaced turns the condition False once the mapper is reset; before that the stale mapping holds.
func TestGrantedResourceValidCondition_Rescoped(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets", Generation: 1}}
	def.Spec.GrantedResource = &v1alpha1.GrantedResource{APIGroup: "example.com", Kind: "Widget"}
	cl := buildClient(t, def)
	widgets := func(namespaced bool) []*metav1.APIResourceList {
		return []*metav1.APIResourceList{{GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: namespaced}}}}
	}
	disc := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: widgets(false)}}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disc))
	rec := &DefinitionReconciler{Client: cl, Factory: jsonpath.NewWithCache(), Mapper: mapper}
	ctx := context.Background()
	reason := func() string {
		t.Helper()
		if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "widgets"}}); err != nil {
			t.Fatal(err)
		}
		got := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := cl.Get(ctx, types.NamespacedName{Name: "widgets"}, got); err != nil {
			t.Fatal(err)
		}
		return meta.FindStatusCondition(got.Status.Conditions, "GrantedResourceValid").Reason
	}

	if r := reason(); r != "ClusterScoped" {
		t.Fatalf("reason %q, want ClusterScoped", r)
	}
	disc.Resources = widgets(true)
	if r := reason(); r != "ClusterScoped" {
		t.Fatalf("reason %q before the reset, want the stale ClusterScoped", r)
	}
	mapper.Reset()
	if r := reason(); r != "Namespaced" {
		t.Fatalf("reason %q after the reset, want Namespaced", r)
	}
}

// TestCatalogFieldsValidCondition_ValueBacked: catalogFields on a value-backed definition stored past
// the webhook are reported, with the webhook's text.
func TestCatalogFieldsValidCondition_ValueBacked(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "values", Generation: 1},
		Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{CatalogFields: []v1alpha1.CatalogField{{Name: "a", Path: "$.a"}}},
	}
	cl := buildClient(t, def)
	rec := &DefinitionReconciler{Client: cl, Factory: jsonpath.NewWithCache(), Mapper: testMapper()}
	ctx := context.Background()
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "values"}}); err != nil {
		t.Fatal(err)
	}
	got := &v1alpha1.GrantableClusterResourceDefinition{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "values"}, got); err != nil {
		t.Fatal(err)
	}
	c := meta.FindStatusCondition(got.Status.Conditions, "CatalogFieldsValid")
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != "InvalidCatalogFields" ||
		c.Message != engine.ValueBackedCatalogFieldsProblem(def) {
		t.Fatalf("condition %+v", c)
	}
}

// bulkyValue is the value of each of the ten parameters of a bulky StorageClass, under
// MaxCatalogFieldValueBytes so that it is projected.
var bulkyValue = strings.Repeat("v", engine.MaxCatalogFieldValueBytes-12)

// bulkyStorageClasses returns n StorageClasses whose ten parameters take close to
// MaxCatalogFieldValueBytes each, and a definition that shows all ten.
func bulkyStorageClasses(n int) (*v1alpha1.GrantableClusterResourceDefinition, []client.Object) {
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	params := map[string]string{}
	for i := range engine.MaxCatalogFields {
		key := fmt.Sprintf("p%d", i)
		params[key] = bulkyValue
		def.Spec.CatalogFields = append(def.Spec.CatalogFields, v1alpha1.CatalogField{Name: key, Path: "$.parameters." + key})
	}
	objs := make([]client.Object, 0, n)
	for i := range n {
		objs = append(objs, &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("sc-%03d", i)}, Provisioner: "x", Parameters: params})
	}
	return def, objs
}

// bulkyFitting is the largest number of bulky StorageClasses whose catalog fits
// MaxCatalogFieldsBytes, computed from the serialization the budget counts: a list of n entries of the
// same size takes n*(entry+1)+1 bytes (the brackets and n-1 commas).
func bulkyFitting(t *testing.T) int {
	t.Helper()
	fields := map[string]apiextensionsv1.JSON{}
	for i := range engine.MaxCatalogFields {
		raw, err := json.Marshal(bulkyValue)
		if err != nil {
			t.Fatal(err)
		}
		fields[fmt.Sprintf("p%d", i)] = apiextensionsv1.JSON{Raw: raw}
	}
	entry, err := json.Marshal(v1alpha1.AvailableObject{Name: "sc-000", Fields: fields})
	if err != nil {
		t.Fatal(err)
	}
	return (engine.MaxCatalogFieldsBytes - 1) / (len(entry) + 1)
}

// reconcileFieldsCatalog runs one pass for team-a and returns its storageclasses catalog and the events.
func reconcileFieldsCatalog(t *testing.T, objs ...client.Object) (*v1alpha1.AvailableClusterResource, []string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	recorder := record.NewFakeRecorder(10)
	r := &ProjectReconciler{Client: buildClient(t, append(objs, ns)...), Mapper: testMapper(), Factory: jsonpath.NewWithCache(), Recorder: recorder}
	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
		t.Fatal(err)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a", Name: "storageclasses"}, ar); err != nil {
		t.Fatal(err)
	}
	close(recorder.Events)
	var events []string
	for e := range recorder.Events {
		events = append(events, e)
	}
	return ar, events
}

// TestReconcile_CatalogFieldsBudget: under the budget every entry carries its fields; over it no entry
// does, the names stay, and the definition gets a Warning event saying so.
func TestReconcile_CatalogFieldsBudget(t *testing.T) {
	fitting := bulkyFitting(t)

	t.Run("under the budget", func(t *testing.T) {
		def, scs := bulkyStorageClasses(fitting)
		ar, events := reconcileFieldsCatalog(t, append(scs, def)...)
		if len(ar.Status.Available) != fitting {
			t.Fatalf("available = %d, want %d", len(ar.Status.Available), fitting)
		}
		for _, e := range ar.Status.Available {
			if len(e.Fields) != engine.MaxCatalogFields {
				t.Fatalf("%s carries %d fields, want %d", e.Name, len(e.Fields), engine.MaxCatalogFields)
			}
		}
		if len(events) != 0 {
			t.Fatalf("unexpected events: %v", events)
		}
	})

	t.Run("over the budget", func(t *testing.T) {
		over := fitting + 1
		def, scs := bulkyStorageClasses(over)
		ar, events := reconcileFieldsCatalog(t, append(scs, def)...)
		if len(ar.Status.Available) != over || ar.Status.AvailableCount != over {
			t.Fatalf("available = %d (count %d), want %d", len(ar.Status.Available), ar.Status.AvailableCount, over)
		}
		for i, e := range ar.Status.Available {
			if want := fmt.Sprintf("sc-%03d", i); e.Name != want || e.Fields != nil {
				t.Fatalf("entry %d = %s with fields %v, want %s without fields", i, e.Name, e.Fields, want)
			}
		}
		if len(events) != 1 || !strings.HasPrefix(events[0], "Warning "+ReasonCatalogFieldsSkipped+" ") ||
			!strings.Contains(events[0], "team-a/storageclasses") ||
			!strings.Contains(events[0], fmt.Sprintf("over the budget of %d bytes", engine.MaxCatalogFieldsBytes)) {
			t.Fatalf("events = %v", events)
		}
	})
}

// TestReportSkips_Message: the event lists at most maxSkippedSample names and counts the rest, and a
// catalog that is both oversized and over the budget says both.
func TestReportSkips_Message(t *testing.T) {
	recorder := record.NewFakeRecorder(1)
	r := &ProjectReconciler{Recorder: recorder}
	var oversized []string
	for i := range maxSkippedSample + 2 {
		oversized = append(oversized, fmt.Sprintf("sc-%d/big", i))
	}
	r.reportSkips(context.Background(), "team-a", storageClassDefinition(v1alpha1.AvailabilityAll),
		engine.CatalogSkips{Oversized: oversized, OverBudgetBytes: engine.MaxCatalogFieldsBytes + 1})
	want := fmt.Sprintf("Warning %s Catalog fields of AvailableClusterResource team-a/storageclasses: "+
		"values longer than %d bytes are left out, 7 in total: sc-0/big, sc-1/big, sc-2/big, sc-3/big, sc-4/big and 2 more; "+
		"all fields are left out: the catalog with them takes at least %d bytes, over the budget of %d bytes per catalog.",
		ReasonCatalogFieldsSkipped, engine.MaxCatalogFieldValueBytes, engine.MaxCatalogFieldsBytes+1, engine.MaxCatalogFieldsBytes)
	if got := <-recorder.Events; got != want {
		t.Fatalf("event\n%q\nwant\n%q", got, want)
	}
}

// TestReportSkips_LogsOnChange: the event is emitted on every pass, the log only when the skip set of
// the catalog changes, and a pass without skips forgets it.
func TestReportSkips_LogsOnChange(t *testing.T) {
	var lines []string
	ctx := ctrllog.IntoContext(context.Background(), funcr.New(func(_, args string) {
		if strings.Contains(args, "catalog fields skipped") {
			lines = append(lines, args)
		}
	}, funcr.Options{Verbosity: 1}))
	recorder := record.NewFakeRecorder(10)
	r := &ProjectReconciler{Recorder: recorder}
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	pass := func(oversized ...string) int {
		t.Helper()
		before := len(lines)
		r.reportSkips(ctx, "team-a", def, engine.CatalogSkips{Oversized: oversized})
		return len(lines) - before
	}

	if n := pass("big/big"); n != 2 {
		t.Fatalf("first pass: %d log lines, want an Info and a V(1) one: %v", n, lines)
	}
	if !strings.Contains(lines[0], `"level"=0`) || !strings.Contains(lines[1], `"level"=1`) || !strings.Contains(lines[1], `"oversized"=["big/big"]`) {
		t.Fatalf("log lines: %v", lines)
	}
	if n := pass("big/big"); n != 0 {
		t.Fatalf("same skips: %d log lines, want none: %v", n, lines)
	}
	if n := pass("big/big", "other/big"); n != 2 {
		t.Fatalf("changed skips: %d log lines, want 2", n)
	}
	pass()
	if n := pass("big/big", "other/big"); n != 2 {
		t.Fatalf("skips back after a clean pass: %d log lines, want 2", n)
	}
	if got := len(recorder.Events); got != 4 {
		t.Fatalf("events: %d, want one per pass that skips", got)
	}
}

// TestReconcile_CatalogFieldOversizedValueIsReported: a value over MaxCatalogFieldValueBytes is left
// out of its entry, the other fields stay, and the definition gets a Warning event naming it.
func TestReconcile_CatalogFieldOversizedValueIsReported(t *testing.T) {
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	def.Spec.CatalogFields = []v1alpha1.CatalogField{{Name: "provisioner", Path: "$.provisioner"}, {Name: "big", Path: "$.parameters.big"}}
	big := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "big"}, Provisioner: "x",
		Parameters: map[string]string{"big": strings.Repeat("v", 600)}}
	small := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "small"}, Provisioner: "y",
		Parameters: map[string]string{"big": "fits"}}
	ar, events := reconcileFieldsCatalog(t, def, big, small)

	if f := ar.Status.Available[0].Fields; ar.Status.Available[0].Name != "big" || len(f) != 1 || string(f["provisioner"].Raw) != `"x"` {
		t.Fatalf("big: %+v", ar.Status.Available[0])
	}
	if f := ar.Status.Available[1].Fields; len(f) != 2 || string(f["big"].Raw) != `"fits"` {
		t.Fatalf("small: %+v", ar.Status.Available[1])
	}
	if len(events) != 1 || !strings.HasPrefix(events[0], "Warning "+ReasonCatalogFieldsSkipped+" ") ||
		!strings.Contains(events[0], "values longer than 512 bytes are left out, 1 in total: big/big.") {
		t.Fatalf("events = %v", events)
	}
}

// TestReconcile_SameSkipsLogOnce: a second pass over the same catalog with the same skip emits the
// event again but writes no Info log line.
func TestReconcile_SameSkipsLogOnce(t *testing.T) {
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	def.Spec.CatalogFields = []v1alpha1.CatalogField{{Name: "big", Path: "$.parameters.big"}}
	big := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "big"}, Provisioner: "x",
		Parameters: map[string]string{"big": strings.Repeat("v", 600)}}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	var infos int
	ctx := ctrllog.IntoContext(context.Background(), funcr.New(func(_, args string) {
		if strings.Contains(args, `"level"=0`) && strings.Contains(args, `"msg"="catalog fields skipped"`) {
			infos++
		}
	}, funcr.Options{}))
	recorder := record.NewFakeRecorder(10)
	r := &ProjectReconciler{Client: buildClient(t, def, big, ns), Mapper: testMapper(), Factory: jsonpath.NewWithCache(), Recorder: recorder}
	for pass, want := range []int{1, 1} {
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
			t.Fatal(err)
		}
		if infos != want {
			t.Fatalf("pass %d: %d Info lines, want %d", pass+1, infos, want)
		}
	}
	if len(recorder.Events) != 2 {
		t.Fatalf("events: %d, want one per pass", len(recorder.Events))
	}
}

// TestReconcile_DeletedDefinitionForgetsSkips: a pass after the definition is deleted drops its logged
// skip set, so the same definition re-created with the same skip is logged again.
func TestReconcile_DeletedDefinitionForgetsSkips(t *testing.T) {
	def := storageClassDefinition(v1alpha1.AvailabilityAll)
	def.Spec.CatalogFields = []v1alpha1.CatalogField{{Name: "big", Path: "$.parameters.big"}}
	big := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "big"}, Provisioner: "x",
		Parameters: map[string]string{"big": strings.Repeat("v", 600)}}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	var infos int
	ctx := ctrllog.IntoContext(context.Background(), funcr.New(func(_, args string) {
		if strings.Contains(args, `"level"=0`) && strings.Contains(args, `"msg"="catalog fields skipped"`) {
			infos++
		}
	}, funcr.Options{}))
	cl := buildClient(t, def.DeepCopy(), big, ns)
	r := &ProjectReconciler{Client: cl, Mapper: testMapper(), Factory: jsonpath.NewWithCache(), Recorder: record.NewFakeRecorder(10)}
	pass := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
			t.Fatal(err)
		}
	}

	pass()
	if err := cl.Delete(ctx, def.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	pass()
	if len(r.loggedSkips) != 0 {
		t.Fatalf("logged skips after the deletion: %v", r.loggedSkips)
	}
	if err := cl.Create(ctx, def.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	pass()
	if infos != 2 {
		t.Fatalf("%d Info lines, want one before the deletion and one after the re-creation", infos)
	}
}
