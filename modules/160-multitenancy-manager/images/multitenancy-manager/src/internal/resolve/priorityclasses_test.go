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

package resolve

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
	"controller/internal/testutil"
)

// platformClassesOpen are the classes of the deckhouse module that projects may use without a policy.
// Every class the deckhouse module ships is in exactly one of three groups: here, in the excluded names
// or in the reserved names of the shipped definition, so a new platform class cannot reach the projects
// unnoticed.
var platformClassesOpen = []string{
	"production-medium",
	"production-low",
	"staging",
	"develop",
}

// platformClassesReserved are the classes the shipped definition must reserve, as agreed with the
// Scheduling team: the ones above the platform components and the CI/CD class.
var platformClassesReserved = []string{
	"production-high",
	"deployment-machinery",
}

func shippedPriorityClasses(t *testing.T) *v1alpha1.GrantableClusterResourceDefinition {
	t.Helper()
	for _, def := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceDefinition](t, "GrantableClusterResourceDefinition") {
		if def.Name == "priorityclasses" {
			return def
		}
	}
	t.Fatal("definition priorityclasses is not in the rendered template")
	return nil
}

func filterNames(filters []v1alpha1.ResourceFilter) []string {
	var names []string
	for _, f := range filters {
		names = append(names, f.Names...)
	}
	return names
}

var (
	priorityClassName = regexp.MustCompile(`(?m)^kind: PriorityClass\nmetadata:\n  name: (\S+)$`)
	globalDefaultTrue = regexp.MustCompile(`(?m)^globalDefault: true$`)
)

// TestDeckhousePriorityClassesAreClassified walks the PriorityClasses the deckhouse module ships and
// requires each of them to be in exactly one group: excluded, reserved or open. The cluster default
// class must be open: a pod created without priorityClassName gets it before the check, so closing it
// would refuse such pods in every project.
func TestDeckhousePriorityClassesAreClassified(t *testing.T) {
	def := shippedPriorityClasses(t)
	groups := map[string][]string{
		"excluded": filterNames(def.Spec.Excluded),
		"reserved": filterNames(def.Spec.Reserved),
		"open":     platformClassesOpen,
	}
	groupsOf := func(name string) []string {
		var in []string
		for _, g := range []string{"excluded", "reserved", "open"} {
			if slices.Contains(groups[g], name) {
				in = append(in, g)
			}
		}
		return in
	}
	for _, name := range []string{"system-node-critical", "system-cluster-critical"} {
		if got := groupsOf(name); !slices.Equal(got, []string{"excluded"}) {
			t.Errorf("the Kubernetes class %s: got groups %v, want only excluded", name, got)
		}
	}
	for _, name := range platformClassesReserved {
		if got := groupsOf(name); !slices.Equal(got, []string{"reserved"}) {
			t.Errorf("%s: got groups %v, want only reserved", name, got)
		}
	}

	dir := filepath.Dir(testutil.ModuleFile(t, "modules/002-deckhouse/templates/priority-class/develop.yaml"))
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var shipped, defaults []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		matches := priorityClassName.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			shipped = append(shipped, m[1])
			if got := groupsOf(m[1]); len(got) != 1 {
				t.Errorf("%s (%s): got groups %v, want exactly one of excluded and reserved in grantable-priorityclasses.yaml or platformClassesOpen",
					m[1], filepath.Base(path), got)
			}
		}
		// One class per file, so the file's globalDefault belongs to its class.
		if len(matches) == 1 && globalDefaultTrue.Match(data) {
			defaults = append(defaults, matches[0][1])
		}
	}
	// Guard the walk itself: a pattern that matches nothing must not pass as "all classified".
	if len(shipped) < len(paths) {
		t.Fatalf("found %d PriorityClasses in %d files of %s", len(shipped), len(paths), dir)
	}
	if len(defaults) != 1 {
		t.Fatalf("found cluster default classes %v in %s, want exactly one", defaults, dir)
	}
	if got := groupsOf(defaults[0]); !slices.Equal(got, []string{"open"}) {
		t.Errorf("the cluster default class %s: got groups %v, want only open", defaults[0], got)
	}
	// Every name in the lists is a class that exists: a stale entry would hide a missing one.
	for g, names := range groups {
		for _, name := range names {
			if !slices.Contains(shipped, name) && name != "system-node-critical" && name != "system-cluster-critical" {
				t.Errorf("%s lists %s, which neither the deckhouse module nor Kubernetes ships", g, name)
			}
		}
	}
}

// TestShippedPriorityClassAvailability resolves the shipped priorityclasses definition against the
// classes of Kubernetes and Deckhouse: the excluded classes are unavailable to every project, even when
// a policy allows them; the reserved classes are unavailable until a policy names them in allowed; the
// other classes stay available without a policy. There is no cluster default to offer: the definition
// has no defaultFrom.
func TestShippedPriorityClassAvailability(t *testing.T) {
	reg := shippedPriorityClasses(t)

	excluded := []string{"system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "standby"}
	open := append(slices.Clone(platformClassesOpen), "custom-tenant-class")
	all := slices.Concat(excluded, platformClassesReserved, open)

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "scheduling.k8s.io", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "scheduling.k8s.io", Version: "v1", Kind: "PriorityClass"}, meta.RESTScopeRoot)
	scheme := runtime.NewScheme()
	if err := schedulingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	objs := make([]client.Object, 0, len(all))
	for _, name := range all {
		objs = append(objs, &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: name}, GlobalDefault: name == "develop"})
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	check := func(t *testing.T, policy []v1alpha1.GrantResource, available []string) {
		t.Helper()
		resolved, err := Resolve(context.Background(), cl, m, reg, policy)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		for _, name := range all {
			if got, want := resolved.Decide(name), slices.Contains(available, name); got != want {
				t.Errorf("%s: got available=%v want %v", name, got, want)
			}
		}
		if got := resolved.Default(); got != "" {
			t.Errorf("default: got %q want none", got)
		}
	}

	t.Run("no policy", func(t *testing.T) {
		check(t, nil, open)
	})
	t.Run("availabilityDefault All does not open the reserved classes", func(t *testing.T) {
		check(t, []v1alpha1.GrantResource{{AvailabilityDefault: v1alpha1.AvailabilityAll}}, open)
	})
	t.Run("allowed with availabilityDefault All opens only the named reserved class", func(t *testing.T) {
		check(t, []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: v1alpha1.AvailabilityAll}},
			append(slices.Clone(open), "production-high"))
	})
	t.Run("allowed without availabilityDefault narrows to the named classes", func(t *testing.T) {
		check(t, []v1alpha1.GrantResource{{Allowed: []string{"deployment-machinery", "develop"}}},
			[]string{"deployment-machinery", "develop"})
	})
	t.Run("a policy cannot open an excluded class", func(t *testing.T) {
		check(t, []v1alpha1.GrantResource{{Allowed: []string{"system-cluster-critical", "cluster-low"}, AvailabilityDefault: v1alpha1.AvailabilityAll}},
			open)
	})
}
