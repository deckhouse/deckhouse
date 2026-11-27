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

package webhooks

import (
	"slices"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/internal/testutil"
)

const betaStorageClass = "volume.beta.kubernetes.io/storage-class"

// claimTemplate is a PersistentVolumeClaim template: class "-" leaves spec.storageClassName out, beta ""
// leaves the annotation out.
func claimTemplate(name, class, beta string) map[string]any {
	meta := map[string]any{"name": name}
	if beta != "" {
		meta["annotations"] = map[string]any{betaStorageClass: beta}
	}
	spec := map[string]any{"accessModes": []any{"ReadWriteOnce"}, "resources": map[string]any{"requests": map[string]any{"storage": "1Gi"}}}
	if class != "-" {
		spec["storageClassName"] = class
	}
	return map[string]any{"metadata": meta, "spec": spec}
}

func ephemeralVolume(name, class, beta string) map[string]any {
	tpl := claimTemplate(name, class, beta)
	delete(tpl["metadata"].(map[string]any), "name")
	return map[string]any{"name": name, "ephemeral": map[string]any{"volumeClaimTemplate": tpl}}
}

func podSpec(volumes ...map[string]any) map[string]any {
	list := []any{map[string]any{"name": "scratch", "emptyDir": map[string]any{}}}
	for _, v := range volumes {
		list = append(list, v)
	}
	return map[string]any{"containers": []any{map[string]any{"name": "c", "image": "busybox"}}, "volumes": list}
}

type workloadKind struct {
	gvr  metav1.GroupVersionResource
	gvk  metav1.GroupVersionKind
	wrap func(pod map[string]any) map[string]any
}

var (
	podTemplate = func(pod map[string]any) map[string]any {
		return map[string]any{"spec": map[string]any{"template": map[string]any{"spec": pod}}}
	}
	workloadKinds = map[string]workloadKind{
		"Pod": {
			metav1.GroupVersionResource{Version: "v1", Resource: "pods"}, metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
			func(pod map[string]any) map[string]any { return map[string]any{"spec": pod} },
		},
		"ReplicationController": {
			metav1.GroupVersionResource{Version: "v1", Resource: "replicationcontrollers"}, metav1.GroupVersionKind{Version: "v1", Kind: "ReplicationController"},
			podTemplate,
		},
		"Deployment": {
			metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			podTemplate,
		},
		"ReplicaSet": {
			metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
			podTemplate,
		},
		"DaemonSet": {
			metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"},
			podTemplate,
		},
		"StatefulSet": {
			metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			podTemplate,
		},
		"Job": {
			metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			podTemplate,
		},
		"CronJob": {
			metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
			func(pod map[string]any) map[string]any {
				return map[string]any{"spec": map[string]any{"schedule": "@daily", "jobTemplate": podTemplate(pod)}}
			},
		},
	}
)

// storageClass is a StorageClass created at the given minute, marked as the cluster default by the
// given annotations.
func storageClass(name string, minute int, defaultKeys ...string) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Date(2026, time.November, 17, 0, minute, 0, 0, time.UTC)),
		},
		Provisioner: "x",
	}
	for _, key := range defaultKeys {
		if sc.Annotations == nil {
			sc.Annotations = map[string]string{}
		}
		sc.Annotations[key] = "true"
	}
	return sc
}

const (
	isDefaultClass     = "storageclass.kubernetes.io/is-default-class"
	betaIsDefaultClass = "storageclass.beta.kubernetes.io/is-default-class"
)

// shippedTemplatesValidator serves /is-granted with the definitions and references the module ships,
// the given StorageClasses and three projects: "restricted" allows local only, "denylist" denies
// replicated only, and "open" has no policy. edit changes the shipped definitions first.
func shippedTemplatesValidator(t *testing.T, classes []*storagev1.StorageClass, edit func(*v1alpha1.GrantableClusterResourceDefinition)) *IsGrantedValidator {
	t.Helper()
	return shippedTemplatesValidatorWithReferences(t, classes, edit, nil)
}

// shippedTemplatesValidatorWithReferences is shippedTemplatesValidator that also lets editRef change
// the shipped references.
func shippedTemplatesValidatorWithReferences(t *testing.T, classes []*storagev1.StorageClass,
	edit func(*v1alpha1.GrantableClusterResourceDefinition), editRef func(*v1alpha1.GrantableClusterResourceReference),
) *IsGrantedValidator {
	t.Helper()
	objs := []client.Object{
		projectNS("restricted", map[string]string{"env": "prod"}),
		projectNS("denylist", map[string]string{"env": "dev"}),
		projectNS("open", nil),
		&grantsv1alpha2.ClusterResourceGrantPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "g"},
			Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				Resources:         []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"local"}}},
			},
		},
		&grantsv1alpha2.ClusterResourceGrantPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "d"},
			Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
				Resources:         []v1alpha1.GrantResource{{ResourceName: "storageclasses", Denied: []string{"replicated"}}},
			},
		},
	}
	for _, sc := range classes {
		objs = append(objs, sc)
	}
	for _, def := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceDefinition](t, "GrantableClusterResourceDefinition") {
		if edit != nil {
			edit(def)
		}
		objs = append(objs, def)
	}
	for _, ref := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceReference](t, "GrantableClusterResourceReference") {
		if editRef != nil {
			editRef(ref)
		}
		objs = append(objs, ref)
	}
	return isGranted(t, objs...)
}

func decideWorkload(t *testing.T, v *IsGrantedValidator, op admissionv1.Operation, ns, kind string, obj, old map[string]any) *admissionv1.AdmissionResponse {
	t.Helper()
	k := workloadKinds[kind]
	var oldRaw []byte
	if old != nil {
		oldRaw = raw(t, old)
	}
	return serve(t, v, "/is-granted", review(op, k.gvr, k.gvk, ns, "w", raw(t, obj), oldRaw))
}

// noClassObjects returns a StatefulSet with a volume claim template and one object of every workload
// kind with an ephemeral volume, each leaving the class unset in one template.
func noClassObjects() map[string]map[string]any {
	out := map[string]map[string]any{}
	sts := podTemplate(podSpec())
	sts["spec"].(map[string]any)["volumeClaimTemplates"] = []any{claimTemplate("ok", "local", ""), claimTemplate("data", "-", "")}
	out["StatefulSet claim template"] = sts
	for kind, k := range workloadKinds {
		out[kind+" ephemeral volume"] = k.wrap(podSpec(ephemeralVolume("ok", "local", ""), ephemeralVolume("data", "-", "")))
	}
	return out
}

func workloadKindOf(name string) string { return strings.Fields(name)[0] }

// TestShippedStorageClassTemplates runs the volume claim templates of the workload kinds through
// /is-granted with the references the module ships: kube-controller-manager creates the claims from
// them as a writer the grant webhooks skip, so the template is where the class is checked.
func TestShippedStorageClassTemplates(t *testing.T) {
	v := shippedTemplatesValidator(t, []*storagev1.StorageClass{
		storageClass("local", 0), storageClass("replicated", 1, isDefaultClass),
	}, nil)

	type volumeCase struct {
		name        string
		class, beta string
		denied      bool
		missing     bool
	}
	volumeCases := []volumeCase{
		{name: "an available class", class: "local"},
		{name: "an unavailable class", class: "replicated", denied: true},
		{name: "no class and an unavailable cluster default", class: "-", denied: true, missing: true},
		{name: "an empty class for a pre-provisioned volume", class: ""},
		{name: "an available class in the beta annotation only", class: "-", beta: "local"},
		{name: "an unavailable class in the beta annotation", class: "local", beta: "replicated", denied: true},
		{name: "an available class in the beta annotation over an unavailable one in spec", class: "replicated", beta: "local"},
	}
	check := func(t *testing.T, c volumeCase, resp *admissionv1.AdmissionResponse) {
		t.Helper()
		if resp.Allowed == c.denied {
			t.Fatalf("allowed = %v, want %v: %v", resp.Allowed, !c.denied, resp.Result)
		}
		if c.missing && !strings.Contains(resp.Result.Message, `leaves the StorageClass unset where one is expected, so it gets the cluster default StorageClass "replicated"`) {
			t.Fatalf("the denial must name the cluster default, got %q", resp.Result.Message)
		}
	}

	for _, c := range volumeCases {
		t.Run("StatefulSet claim template with "+c.name, func(t *testing.T) {
			sts := podTemplate(podSpec())
			sts["spec"].(map[string]any)["volumeClaimTemplates"] = []any{claimTemplate("ok", "local", ""), claimTemplate("data", c.class, c.beta)}
			check(t, c, decideWorkload(t, v, admissionv1.Create, "restricted", "StatefulSet", sts, nil))
		})
		for kind, k := range workloadKinds {
			t.Run(kind+" ephemeral volume with "+c.name, func(t *testing.T) {
				obj := k.wrap(podSpec(ephemeralVolume("ok", "local", ""), ephemeralVolume("data", c.class, c.beta)))
				check(t, c, decideWorkload(t, v, admissionv1.Create, "restricted", kind, obj, nil))
			})
		}
	}

	t.Run("a template without a class is allowed where no policy configures StorageClasses", func(t *testing.T) {
		sts := podTemplate(podSpec(ephemeralVolume("scratch-claim", "-", "")))
		sts["spec"].(map[string]any)["volumeClaimTemplates"] = []any{claimTemplate("data", "-", "")}
		if resp := decideWorkload(t, v, admissionv1.Create, "open", "StatefulSet", sts, nil); !resp.Allowed {
			t.Fatalf("must be allowed: %v", resp.Result)
		}
	})

	t.Run("an UPDATE that keeps a template without a class is allowed", func(t *testing.T) {
		old := workloadKinds["Deployment"].wrap(podSpec(ephemeralVolume("data", "-", "")))
		obj := workloadKinds["Deployment"].wrap(podSpec(ephemeralVolume("data", "-", "")))
		obj["metadata"] = map[string]any{"labels": map[string]any{"changed": "true"}}
		if resp := decideWorkload(t, v, admissionv1.Update, "restricted", "Deployment", obj, old); !resp.Allowed {
			t.Fatalf("must be allowed: %v", resp.Result)
		}
	})

	t.Run("an UPDATE that adds a template without a class is denied", func(t *testing.T) {
		old := workloadKinds["Deployment"].wrap(podSpec(ephemeralVolume("data", "-", "")))
		obj := workloadKinds["Deployment"].wrap(podSpec(ephemeralVolume("data", "-", ""), ephemeralVolume("cache", "-", "")))
		if resp := decideWorkload(t, v, admissionv1.Update, "restricted", "Deployment", obj, old); resp.Allowed {
			t.Fatal("must be denied")
		}
	})
}

// TestShippedStorageClassTemplatesClusterDefault checks a template without a class against the
// StorageClass Kubernetes gives it: the cluster default, picked as the DefaultStorageClass admission
// plugin picks it.
func TestShippedStorageClassTemplatesClusterDefault(t *testing.T) {
	cases := []struct {
		name    string
		classes []*storagev1.StorageClass
		// denied lists the projects where a template without a class is refused.
		denied []string
		def    string
	}{
		{
			name:    "no cluster default",
			classes: []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1)},
		},
		{
			name:    "an available cluster default",
			classes: []*storagev1.StorageClass{storageClass("local", 0, isDefaultClass), storageClass("replicated", 1)},
		},
		{
			name:    "an unavailable cluster default",
			classes: []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1, isDefaultClass)},
			denied:  []string{"restricted", "denylist"},
			def:     "replicated",
		},
		{
			name:    "an unavailable cluster default marked by the beta annotation only",
			classes: []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1, betaIsDefaultClass)},
			denied:  []string{"restricted", "denylist"},
			def:     "replicated",
		},
		{
			name:    "an older available and a newer unavailable cluster default",
			classes: []*storagev1.StorageClass{storageClass("local", 0, isDefaultClass), storageClass("replicated", 1, isDefaultClass)},
			denied:  []string{"restricted", "denylist"},
			def:     "replicated",
		},
		{
			name:    "an older unavailable and a newer available cluster default",
			classes: []*storagev1.StorageClass{storageClass("local", 1, isDefaultClass), storageClass("replicated", 0, isDefaultClass)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := shippedTemplatesValidator(t, c.classes, nil)
			for _, project := range []string{"restricted", "denylist", "open"} {
				for name, obj := range noClassObjects() {
					resp := decideWorkload(t, v, admissionv1.Create, project, workloadKindOf(name), obj, nil)
					want := slices.Contains(c.denied, project)
					if resp.Allowed == want {
						t.Fatalf("%s in %s: allowed = %v, want %v: %v", name, project, resp.Allowed, !want, resp.Result)
					}
					if want && !strings.Contains(resp.Result.Message, `the cluster default StorageClass "`+c.def+`"`) {
						t.Fatalf("%s in %s: the denial must name the cluster default %q, got %q", name, project, c.def, resp.Result.Message)
					}
				}
			}
		})
	}
}

// TestMissingPathWithoutDefaultFrom checks a definition that cannot tell what an unset place gets: a
// template without a class is refused wherever a policy configures the definition.
func TestMissingPathWithoutDefaultFrom(t *testing.T) {
	v := shippedTemplatesValidator(t, []*storagev1.StorageClass{storageClass("local", 0, isDefaultClass), storageClass("replicated", 1)},
		func(def *v1alpha1.GrantableClusterResourceDefinition) { def.Spec.DefaultFrom = nil })
	for _, project := range []string{"restricted", "denylist", "open"} {
		for name, obj := range noClassObjects() {
			resp := decideWorkload(t, v, admissionv1.Create, project, workloadKindOf(name), obj, nil)
			want := project != "open"
			if resp.Allowed == want {
				t.Fatalf("%s in %s: allowed = %v, want %v: %v", name, project, resp.Allowed, !want, resp.Result)
			}
			if want && !strings.Contains(resp.Result.Message, "but a ClusterResourceGrantPolicy configures storageclasses") {
				t.Fatalf("%s in %s: unexpected denial %q", name, project, resp.Result.Message)
			}
		}
	}
}

// TestMissingName checks that missingName decides the places missingPath selects as the given name,
// in place of the cluster default that defaultFrom finds and, without defaultFrom, in place of the
// denial in a project that a ClusterResourceGrantPolicy restricts.
func TestMissingName(t *testing.T) {
	setMissingName := func(name string) func(*v1alpha1.GrantableClusterResourceReference) {
		return func(ref *v1alpha1.GrantableClusterResourceReference) {
			for i := range ref.Spec.FieldPaths {
				if ref.Spec.FieldPaths[i].MissingPath != "" {
					ref.Spec.FieldPaths[i].MissingName = name
				}
			}
		}
	}
	noDefaultFrom := func(def *v1alpha1.GrantableClusterResourceDefinition) { def.Spec.DefaultFrom = nil }
	cases := []struct {
		name        string
		classes     []*storagev1.StorageClass
		edit        func(*v1alpha1.GrantableClusterResourceDefinition)
		missingName string
		// denied lists the projects where a template without a class is refused.
		denied []string
	}{
		{
			name:        "an unavailable name over an available cluster default",
			classes:     []*storagev1.StorageClass{storageClass("local", 0, isDefaultClass), storageClass("replicated", 1)},
			missingName: "replicated",
			denied:      []string{"restricted", "denylist"},
		},
		{
			name:        "an available name over an unavailable cluster default",
			classes:     []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1, isDefaultClass)},
			missingName: "local",
		},
		{
			name:        "an unavailable name without a cluster default",
			classes:     []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1)},
			missingName: "replicated",
			denied:      []string{"restricted", "denylist"},
		},
		{
			name:        "an available name without defaultFrom",
			classes:     []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1)},
			edit:        noDefaultFrom,
			missingName: "local",
		},
		{
			name:        "an unavailable name without defaultFrom",
			classes:     []*storagev1.StorageClass{storageClass("local", 0), storageClass("replicated", 1)},
			edit:        noDefaultFrom,
			missingName: "replicated",
			denied:      []string{"restricted", "denylist"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := shippedTemplatesValidatorWithReferences(t, c.classes, c.edit, setMissingName(c.missingName))
			for _, project := range []string{"restricted", "denylist", "open"} {
				for name, obj := range noClassObjects() {
					resp := decideWorkload(t, v, admissionv1.Create, project, workloadKindOf(name), obj, nil)
					want := slices.Contains(c.denied, project)
					if resp.Allowed == want {
						t.Fatalf("%s in %s: allowed = %v, want %v: %v", name, project, resp.Allowed, !want, resp.Result)
					}
					if want && !strings.Contains(resp.Result.Message, `so it is handled as StorageClass "`+c.missingName+`"`) {
						t.Fatalf("%s in %s: the denial must name %q, got %q", name, project, c.missingName, resp.Result.Message)
					}
				}
			}
		})
	}
}
