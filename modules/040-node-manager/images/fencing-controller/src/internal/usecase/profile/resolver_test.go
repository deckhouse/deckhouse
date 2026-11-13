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
	"fmt"
	"os"
	"path/filepath"
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

// profileTimings are the timings profileOf resolves to.
var profileTimings = fsm.Params{
	HeartbeatTTL:           5 * time.Second,
	AttestedNodeTaintAfter: 7400 * time.Millisecond,
	HeartbeatInterval:      time.Second,
	SilentNodeTaintAfter:   10500 * time.Millisecond,
}

// TestResolveReadsEveryBuiltinProfile covers the three names the CRD enum admits,
// including the lower-cased object name each of them resolves through.
func TestResolveReadsEveryBuiltinProfile(t *testing.T) {
	for _, name := range v1alpha1.ProfileNames() {
		t.Run(string(name), func(t *testing.T) {
			getter := &stubGetter{profiles: map[string]*v1alpha1.FencingSLAProfile{
				name.ObjectName(): profileOf(name.ObjectName()),
			}}

			got, err := NewResolver(getter).Resolve(context.Background(), incident(name))
			if err != nil {
				t.Fatalf("resolve %s: %v", name, err)
			}

			want := profileTimings
			want.Profile = name
			want.Armed = true

			if got != want {
				t.Errorf("resolved %+v, want %+v", got, want)
			}
		})
	}
}

// TestResolveReadsTheShippedProfiles feeds the resolver the objects the module
// actually ships, so a template that the controller cannot act on fails here.
func TestResolveReadsTheShippedProfiles(t *testing.T) {
	resolver := NewResolver(&stubGetter{profiles: shippedProfiles(t)})

	for _, name := range v1alpha1.ProfileNames() {
		t.Run(string(name), func(t *testing.T) {
			if _, err := resolver.Resolve(context.Background(), incidentOn("worker-"+name.ObjectName(), name)); err != nil {
				t.Fatalf("resolve shipped profile %s: %v", name, err)
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

// TestResolveReportsEachMissingProfile covers a missing profile of each name.
// No other profile replaces it.
func TestResolveReportsEachMissingProfile(t *testing.T) {
	for _, name := range v1alpha1.ProfileNames() {
		t.Run(string(name), func(t *testing.T) {
			profiles := shippedProfiles(t)
			delete(profiles, name.ObjectName())

			got, err := NewResolver(&stubGetter{profiles: profiles}).Resolve(context.Background(), incident(name))
			if !errors.Is(err, ErrConfiguration) || got.Profile != name {
				t.Fatalf("resolve of a missing %s profile returned %+v, %v, want a %v for %s", name, got, err, ErrConfiguration, name)
			}
		})
	}
}

// TestResolveGivesAnUnarmedNodeAFixedHeartbeatTTL covers an incident without
// spec.profileRef. The controller never proves the failure of an unarmed node,
// so it reads no profile and no ConfigMap. The incident stays in FallbackAlive
// for 20s after a heartbeat.
func TestResolveGivesAnUnarmedNodeAFixedHeartbeatTTL(t *testing.T) {
	getter := &stubGetter{err: errors.New("the resolver must not read the API")}

	got, err := NewResolver(getter).Resolve(context.Background(), unarmed())
	if err != nil {
		t.Fatalf("resolve an unarmed node: %v", err)
	}

	if want := (fsm.Params{HeartbeatTTL: 20 * time.Second}); got != want {
		t.Errorf("resolved %+v, want %+v", got, want)
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

func TestResolveRejectsNoObject(t *testing.T) {
	if _, err := NewResolver(&stubGetter{}).Resolve(context.Background(), nil); err == nil {
		t.Error("resolve of a nil object succeeded, want an error")
	}
}

type stubGetter struct {
	profiles map[string]*v1alpha1.FencingSLAProfile
	err      error
}

func (s *stubGetter) GetSLAProfile(_ context.Context, name string) (*v1alpha1.FencingSLAProfile, error) {
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

func unarmed() *v1alpha1.FencingFailedNodeState {
	state := incident(v1alpha1.ProfileCritical)
	state.Spec.Armed = false
	state.Spec.ProfileRef = nil

	return state
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
			Armed:      true,
			ProfileRef: &v1alpha1.ProfileRef{Name: name},
		},
	}
}

// profileOf returns a profile with every value the controller reads. It
// resolves to profileTimings.
func profileOf(name string) *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FencingSLAProfileSpec{
			ControlPlaneLostAfter: metav1.Duration{Duration: 2 * time.Second},
			Watchdog: v1alpha1.FencingSLAProfileWatchdog{
				Timeout: metav1.Duration{Duration: 7 * time.Second},
				Margin:  metav1.Duration{Duration: 400 * time.Millisecond},
			},
			Heartbeat: v1alpha1.FencingSLAProfileHeartbeat{
				Interval:   metav1.Duration{Duration: time.Second},
				APITimeout: metav1.Duration{Duration: 3 * time.Second},
			},
		},
	}
}

func readShippedProfiles(dir string) ([]byte, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no profiles in %s", dir)
	}

	var raw []byte

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		raw = append(raw, "\n---\n"...)
		raw = append(raw, content...)
	}

	return raw, nil
}

// shippedProfiles loads the built-in profiles from the module templates, with the
// Helm lines dropped so the manifests parse as plain YAML.
func shippedProfiles(t *testing.T) map[string]*v1alpha1.FencingSLAProfile {
	t.Helper()

	const templates = "../../../../../../templates/fencing-agent/sla-profiles"

	raw, err := readShippedProfiles(templates)
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
