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
	"strings"
	"testing"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

const (
	stableDefaultKey = "storageclass.kubernetes.io/is-default-class"
	betaDefaultKey   = "storageclass.beta.kubernetes.io/is-default-class"
)

func markedClass(name, key string, created time.Time) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)}, Provisioner: "x"}
	if key != "" {
		sc.Annotations = map[string]string{key: "true"}
	}
	return sc
}

func defaultFromConditionOf(t *testing.T, def *v1alpha1.GrantableClusterResourceDefinition, objs ...client.Object) *metav1.Condition {
	t.Helper()
	cl := buildClient(t, append([]client.Object{def}, objs...)...)
	rec := &DefinitionReconciler{Client: cl, Factory: jsonpath.NewWithCache(), Mapper: testMapper()}
	ctx := context.Background()
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: def.Name}}); err != nil {
		t.Fatal(err)
	}
	got := &v1alpha1.GrantableClusterResourceDefinition{}
	if err := cl.Get(ctx, types.NamespacedName{Name: def.Name}, got); err != nil {
		t.Fatal(err)
	}
	return meta.FindStatusCondition(got.Status.Conditions, DefaultFromResolvedCondition)
}

func TestDefaultFromResolvedCondition(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	withDefaultFrom := func() *v1alpha1.GrantableClusterResourceDefinition {
		def := storageClassDefinition(v1alpha1.AvailabilityAll)
		def.Generation = 3
		def.Spec.DefaultFrom = &v1alpha1.DefaultFrom{AnnotationKey: stableDefaultKey, AdditionalAnnotationKeys: []string{betaDefaultKey}}
		return def
	}

	if c := defaultFromConditionOf(t, storageClassDefinition(v1alpha1.AvailabilityAll), markedClass("fast", stableDefaultKey, older)); c != nil {
		t.Fatalf("a definition without defaultFrom must not carry the condition, got %+v", c)
	}

	c := defaultFromConditionOf(t, withDefaultFrom(), markedClass("fast", stableDefaultKey, older), markedClass("slow", "", older))
	if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "Found" || c.ObservedGeneration != 3 || !strings.Contains(c.Message, `"fast"`) {
		t.Fatalf("one marked class must be Found, got %+v", c)
	}

	c = defaultFromConditionOf(t, withDefaultFrom(), markedClass("fast", stableDefaultKey, older), markedClass("legacy", betaDefaultKey, newer))
	if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "SeveralDefaults" ||
		!strings.Contains(c.Message, "legacy, fast") || !strings.Contains(c.Message, `The newest, "legacy", is used`) {
		t.Fatalf("two marked classes must name both and the newest, got %+v", c)
	}

	notTrue := markedClass("fast", "", older)
	notTrue.Annotations = map[string]string{stableDefaultKey: "True", betaDefaultKey: "false"}
	c = defaultFromConditionOf(t, withDefaultFrom(), notTrue)
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != "NoDefault" ||
		!strings.Contains(c.Message, stableDefaultKey) || !strings.Contains(c.Message, betaDefaultKey) {
		t.Fatalf("no class with exactly \"true\" must be NoDefault naming both keys, got %+v", c)
	}
}
