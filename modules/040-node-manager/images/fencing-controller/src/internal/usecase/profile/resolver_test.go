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

package profile

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// shippedTimings are the fallback TTL and the evacuation delay of every built-in
// profile, as the ADR and the module templates define them.
var shippedTimings = map[v1alpha1.ProfileName]fsm.Params{
	v1alpha1.ProfileCritical: {FallbackTTL: time.Second, EvacuationDelay: 1200 * time.Millisecond},
	v1alpha1.ProfileMedium:   {FallbackTTL: 4 * time.Second, EvacuationDelay: 6 * time.Second},
	v1alpha1.ProfileModerate: {FallbackTTL: 10 * time.Second, EvacuationDelay: 20 * time.Second},
	v1alpha1.ProfileSlow:     {FallbackTTL: 20 * time.Second, EvacuationDelay: 45 * time.Second},
}

// TestResolveReadsEveryBuiltinProfile covers the four names the CRD enum admits,
// including the lower-cased object name each of them resolves through.
func TestResolveReadsEveryBuiltinProfile(t *testing.T) {
	for name, want := range shippedTimings {
		t.Run(string(name), func(t *testing.T) {
			getter := &stubGetter{profiles: map[string]*v1alpha1.FencingSLAProfile{
				name.ObjectName(): profileWith(name.ObjectName(), want),
			}}

			got, err := NewResolver(getter).Resolve(context.Background(), incident(name))
			if err != nil {
				t.Fatalf("resolve %s: %v", name, err)
			}

			if got != want {
				t.Errorf("resolved %+v, want %+v", got, want)
			}
		})
	}
}

// TestResolveReadsTheShippedProfiles feeds the resolver the objects the module
// actually ships, so a template edit that breaks a timing the controller depends
// on fails here.
func TestResolveReadsTheShippedProfiles(t *testing.T) {
	getter := &stubGetter{profiles: shippedProfiles(t)}
	resolver := NewResolver(getter)

	for _, name := range v1alpha1.ProfileNames() {
		t.Run(string(name), func(t *testing.T) {
			// One resolver serves incidents of every profile at the same time,
			// so each Node below is fenced under its own timings.
			got, err := resolver.Resolve(context.Background(), incidentOn("worker-"+name.ObjectName(), name))
			if err != nil {
				t.Fatalf("resolve shipped profile %s: %v", name, err)
			}

			if want := shippedTimings[name]; got != want {
				t.Errorf("shipped profile %s resolved to %+v, want %+v", name, got, want)
			}
		})
	}
}

// TestResolveReportsAMissingProfileAsConfiguration is the degraded configuration
// case: the reconciler tells it apart by ErrConfiguration and keeps the incident
// out of the eviction path.
func TestResolveReportsAMissingProfileAsConfiguration(t *testing.T) {
	getter := &stubGetter{profiles: map[string]*v1alpha1.FencingSLAProfile{}}

	_, err := NewResolver(getter).Resolve(context.Background(), incident(v1alpha1.ProfileCritical))
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("resolve of a missing profile returned %v, want a %v", err, ErrConfiguration)
	}

	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("error %q does not name the profile the operator has to restore", err)
	}
}

func TestResolveReportsANameThatIsNotBuiltIn(t *testing.T) {
	getter := &stubGetter{profiles: shippedProfiles(t)}

	_, err := NewResolver(getter).Resolve(context.Background(), incident(v1alpha1.ProfileName("Instant")))
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("resolve of an unknown name returned %v, want a %v", err, ErrConfiguration)
	}

	if getter.calls != 0 {
		t.Errorf("the resolver made %d API calls for a name that cannot exist, want none", getter.calls)
	}
}

func TestResolveReportsUnusableTimings(t *testing.T) {
	getter := &stubGetter{profiles: map[string]*v1alpha1.FencingSLAProfile{
		"critical": profileWith("critical", fsm.Params{EvacuationDelay: 1200 * time.Millisecond}),
	}}

	_, err := NewResolver(getter).Resolve(context.Background(), incident(v1alpha1.ProfileCritical))
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("resolve of a profile without a fallback ttl returned %v, want a %v", err, ErrConfiguration)
	}
}

// TestResolveKeepsTransientErrorsRetryable checks an unavailable API is not
// reported as a configuration error: the incident has to be retried, not blamed.
func TestResolveKeepsTransientErrorsRetryable(t *testing.T) {
	getter := &stubGetter{err: apierrors.NewServiceUnavailable("etcd leader changed")}
	resolver := NewResolver(getter)

	_, err := resolver.Resolve(context.Background(), incident(v1alpha1.ProfileCritical))
	if err == nil {
		t.Fatal("resolve succeeded while the API was unavailable")
	}

	if errors.Is(err, ErrConfiguration) {
		t.Errorf("an unavailable API was reported as %v", ErrConfiguration)
	}

	getter.err = nil
	getter.profiles = shippedProfiles(t)

	if _, err := resolver.Resolve(context.Background(), incident(v1alpha1.ProfileCritical)); err != nil {
		t.Fatalf("resolve after the API came back: %v", err)
	}
}

// TestResolveKeepsTheTimingsOfAnIncident is the snapshot rule of the ADR: an
// edited profile must not move the deadlines of an incident being processed.
func TestResolveKeepsTheTimingsOfAnIncident(t *testing.T) {
	getter := &stubGetter{profiles: shippedProfiles(t)}
	resolver := NewResolver(getter)
	state := incident(v1alpha1.ProfileCritical)

	first, err := resolver.Resolve(context.Background(), state)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	getter.profiles["critical"] = profileWith("critical", fsm.Params{FallbackTTL: time.Hour, EvacuationDelay: time.Hour})

	second, err := resolver.Resolve(context.Background(), state)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	if second != first {
		t.Errorf("the incident moved from %+v to %+v after the profile was edited", first, second)
	}

	if getter.calls != 1 {
		t.Errorf("the resolver read the profile %d times, want once per incident", getter.calls)
	}
}

// TestResolveRereadsARecreatedObject checks the snapshot is pinned to the object:
// a new incident on the same Node is resolved again.
func TestResolveRereadsARecreatedObject(t *testing.T) {
	getter := &stubGetter{profiles: shippedProfiles(t)}
	resolver := NewResolver(getter)

	if _, err := resolver.Resolve(context.Background(), incident(v1alpha1.ProfileCritical)); err != nil {
		t.Fatalf("resolve the first incident: %v", err)
	}

	recreated := incident(v1alpha1.ProfileCritical)
	recreated.UID = types.UID("9f8e7d6c-9999-8888-7777-666655554444")

	if _, err := resolver.Resolve(context.Background(), recreated); err != nil {
		t.Fatalf("resolve the incident of a recreated object: %v", err)
	}

	if getter.calls != 2 {
		t.Errorf("the resolver read the profile %d times, want one read per incident", getter.calls)
	}
}

func TestForgetDropsTheSnapshot(t *testing.T) {
	getter := &stubGetter{profiles: shippedProfiles(t)}
	resolver := NewResolver(getter)
	state := incident(v1alpha1.ProfileCritical)

	if _, err := resolver.Resolve(context.Background(), state); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	resolver.Forget(state.Name)

	if _, err := resolver.Resolve(context.Background(), state); err != nil {
		t.Fatalf("resolve after forget: %v", err)
	}

	if getter.calls != 2 {
		t.Errorf("the resolver read the profile %d times, want a read after the snapshot was dropped", getter.calls)
	}
}

func TestResolveRejectsNoObject(t *testing.T) {
	if _, err := NewResolver(&stubGetter{}).Resolve(context.Background(), nil); err == nil {
		t.Error("resolve of a nil object succeeded, want an error")
	}
}

type stubGetter struct {
	profiles map[string]*v1alpha1.FencingSLAProfile
	err      error
	calls    int
}

func (s *stubGetter) GetSLAProfile(_ context.Context, name string) (*v1alpha1.FencingSLAProfile, error) {
	s.calls++

	if s.err != nil {
		return nil, s.err
	}

	profile, ok := s.profiles[name]
	if !ok {
		return nil, apierrors.NewNotFound(
			schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "fencingslaprofiles"}, name)
	}

	return profile, nil
}

func incident(name v1alpha1.ProfileName) *v1alpha1.FencingFailedNodeState {
	return incidentOn("worker-3", name)
}

func incidentOn(node string, name v1alpha1.ProfileName) *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name: node,
			UID:  types.UID("1a2b3c4d-1111-2222-3333-444455556666"),
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: v1alpha1.ProfileRef{Name: name},
		},
	}
}

func profileWith(name string, params fsm.Params) *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FencingSLAProfileSpec{
			Fallback:   v1alpha1.FencingSLAProfileFallback{TTL: metav1.Duration{Duration: params.FallbackTTL}},
			Evacuation: v1alpha1.FencingSLAProfileEvacuation{Delay: metav1.Duration{Duration: params.EvacuationDelay}},
		},
	}
}

// shippedProfiles loads the built-in profiles from the module templates, with the
// Helm lines dropped so the manifests parse as plain YAML.
func shippedProfiles(t *testing.T) map[string]*v1alpha1.FencingSLAProfile {
	t.Helper()

	const templates = "../../../../../../templates/fencing-agent/sla-profiles.yaml"

	raw, err := os.ReadFile(templates)
	if err != nil {
		// The file exists in a repo checkout; a build sandbox may cut the tree at src/.
		t.Skipf("shipped profiles are not reachable: %v", err)
	}

	var plain []string

	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "{{") {
			plain = append(plain, line)
		}
	}

	profiles := make(map[string]*v1alpha1.FencingSLAProfile)

	for _, doc := range strings.Split(strings.Join(plain, "\n"), "\n---\n") {
		if !strings.Contains(doc, "kind: FencingSLAProfile") {
			continue
		}

		var profile v1alpha1.FencingSLAProfile
		if err := yaml.Unmarshal([]byte(doc), &profile); err != nil {
			t.Fatalf("unmarshal shipped profile: %v\n%s", err, doc)
		}

		profiles[profile.Name] = &profile
	}

	if len(profiles) != len(v1alpha1.ProfileNames()) {
		t.Fatalf("the module ships %d profiles, the API admits %d names", len(profiles), len(v1alpha1.ProfileNames()))
	}

	return profiles
}
