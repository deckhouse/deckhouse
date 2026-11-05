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

package scope

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

func cm(ns, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
}

func TestAllowed(t *testing.T) {
	restricted := WithDecision(context.Background(), Decision{Mode: ModeNarrowed, Namespaces: sets.New("a")})
	empty := WithDecision(context.Background(), Decision{Mode: ModeFiltered})
	none := context.Background()
	for _, tc := range []struct {
		ctx  context.Context
		ns   string
		want bool
	}{
		{none, "a", true},
		{none, "", true},
		{restricted, "a", true},
		{restricted, "b", false},
		{restricted, "", false},
		{empty, "a", false},
		{empty, "", false},
	} {
		if got := Allowed(tc.ctx, tc.ns); got != tc.want {
			t.Errorf("Allowed(%q)=%v, want %v", tc.ns, got, tc.want)
		}
	}
}

func TestRestricted(t *testing.T) {
	if _, ok := Restricted(context.Background()); ok {
		t.Fatal("restricted without decision")
	}
	got, ok := Restricted(WithDecision(context.Background(), Decision{Mode: ModeFiltered, Namespaces: sets.New("c", "a", "b")}))
	if !ok || len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("%v %v", got, ok)
	}
}

func TestFilterList(t *testing.T) {
	ctx := WithDecision(context.Background(), Decision{Mode: ModeFiltered, Namespaces: sets.New("a", "b")})
	list := &corev1.ConfigMapList{
		ListMeta: metav1.ListMeta{ResourceVersion: "42"},
		Items:    []corev1.ConfigMap{*cm("a", "1"), *cm("b", "2"), *cm("c", "3"), *cm("", "cluster")},
	}
	if err := FilterList(ctx, list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Items[0].Name != "1" || list.Items[1].Name != "2" || list.ResourceVersion != "42" {
		t.Fatalf("%+v", list)
	}

	untouched := &corev1.ConfigMapList{Items: []corev1.ConfigMap{*cm("c", "3"), *cm("", "cluster")}}
	if err := FilterList(context.Background(), untouched); err != nil || len(untouched.Items) != 2 {
		t.Fatalf("no decision filtered: %v %d", err, len(untouched.Items))
	}

	emptySet := &corev1.ConfigMapList{Items: []corev1.ConfigMap{*cm("a", "1")}}
	if err := FilterList(WithDecision(context.Background(), Decision{Mode: ModeFiltered}), emptySet); err != nil || len(emptySet.Items) != 0 {
		t.Fatalf("empty set served: %v %d", err, len(emptySet.Items))
	}

	if err := FilterList(ctx, cm("a", "x")); err == nil {
		t.Fatal("not a list accepted")
	}
}

func TestFilter(t *testing.T) {
	ctx := WithDecision(context.Background(), Decision{Mode: ModeFiltered, Namespaces: sets.New("a")})
	got := Filter(ctx, []runtime.Object{cm("a", "1"), cm("b", "2"), cm("", "3"), &metav1.Status{}})
	if len(got) != 1 {
		t.Fatalf("%v", got)
	}
}

func TestCheckGet(t *testing.T) {
	gr := schema.GroupResource{Group: "g", Resource: "gizmos"}
	ctx := WithDecision(context.Background(), Decision{Mode: ModeNarrowed, Namespaces: sets.New("a")})
	if err := CheckGet(ctx, gr, "a", "x"); err != nil {
		t.Fatal(err)
	}
	for _, ns := range []string{"b", ""} {
		err := CheckGet(ctx, gr, ns, "x")
		if !apierrors.IsNotFound(err) {
			t.Fatalf("ns %q: %v", ns, err)
		}
		if msg := err.Error(); msg != `gizmos.g "x" not found` {
			t.Fatalf("message leaks more than a missing object: %q", msg)
		}
	}
	if err := CheckGet(context.Background(), gr, "b", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestWatchFilter(t *testing.T) {
	ctx := WithDecision(context.Background(), Decision{Mode: ModeFiltered, Namespaces: sets.New("a")})
	src := watch.NewFake()
	w := WatchFilter(ctx, src)
	defer w.Stop()

	go func() {
		src.Add(cm("b", "drop"))
		src.Add(cm("", "cluster-drop"))
		src.Add(cm("a", "keep-add"))
		src.Modify(cm("b", "drop"))
		src.Action(watch.Bookmark, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "7"}})
		src.Error(&metav1.Status{Message: "boom"})
		src.Delete(cm("a", "keep-delete"))
	}()

	want := []struct {
		t    watch.EventType
		name string
	}{{watch.Added, "keep-add"}, {watch.Bookmark, ""}, {watch.Error, ""}, {watch.Deleted, "keep-delete"}}
	for _, exp := range want {
		select {
		case e := <-w.ResultChan():
			if e.Type != exp.t {
				t.Fatalf("event %v, want %v", e.Type, exp.t)
			}
			if exp.name != "" && e.Object.(*corev1.ConfigMap).Name != exp.name {
				t.Fatalf("object %s, want %s", e.Object.(*corev1.ConfigMap).Name, exp.name)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %v", exp.t)
		}
	}

	if WatchFilter(context.Background(), src) != watch.Interface(src) {
		t.Fatal("no decision must return the watch unchanged")
	}
}
