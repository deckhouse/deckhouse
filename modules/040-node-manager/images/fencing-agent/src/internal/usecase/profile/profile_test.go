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
	"net/url"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

type getterStub struct {
	calls   int
	results []func() (*v1alpha1.FencingSLAProfile, error)
}

func (g *getterStub) GetSLAProfile(_ context.Context, _ string) (*v1alpha1.FencingSLAProfile, error) {
	g.calls++

	next := g.results[0]
	if len(g.results) > 1 {
		g.results = g.results[1:]
	}

	return next()
}

func notFound() error {
	return apierrors.NewNotFound(schema.GroupResource{Group: "node-manager.deckhouse.io", Resource: "fencingslaprofiles"}, "medium")
}

func TestLoadReturnsValidatedProfile(t *testing.T) {
	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) { return validProfile(), nil },
	}}

	sla, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if sla.Rejoin.Interval.Duration != time.Second {
		t.Errorf("unexpected SLA %+v", sla)
	}
}

// NotFound is deterministic: the profile is either shipped or referenced
// wrongly, waiting inside the process would only hide the failure.
func TestLoadDoesNotRetryNotFound(t *testing.T) {
	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) { return nil, notFound() },
	}}

	if _, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond); err == nil {
		t.Fatal("expected an error")
	}

	if g.calls != 1 {
		t.Errorf("NotFound must not be retried, got %d calls", g.calls)
	}
}

func TestLoadDoesNotRetryInvalidProfile(t *testing.T) {
	broken := validProfile()
	broken.Spec.Memberlist.ProbeInterval = dur(0)

	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) { return broken, nil },
	}}

	if _, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond); err == nil {
		t.Fatal("expected an error")
	}

	if g.calls != 1 {
		t.Errorf("an invalid profile must not be retried, got %d calls", g.calls)
	}
}

// A malformed duration fails while the response is being decoded, so the error
// carries no API status. Retrying would re-read the same unparseable object,
// and the agent must say the profile is invalid instead of blaming the API.
func TestLoadDoesNotRetryDecodeFailure(t *testing.T) {
	decodeErr := errors.New(`get fencingslaprofile "medium": error unmarshaling JSON: time: invalid duration "soon"`)

	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) { return nil, decodeErr },
	}}

	_, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond)
	if err == nil {
		t.Fatal("expected an error")
	}

	if g.calls != 1 {
		t.Errorf("a decode failure must not be retried, got %d calls", g.calls)
	}

	if !strings.Contains(err.Error(), "is invalid") {
		t.Errorf("error %q must report the profile as invalid", err)
	}
}

// Denied access is deterministic: RBAC will not change while the process waits.
func TestLoadDoesNotRetryForbidden(t *testing.T) {
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "node-manager.deckhouse.io", Resource: "fencingslaprofiles"},
		"medium", errors.New("no rule"),
	)

	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) { return nil, forbidden },
	}}

	if _, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond); err == nil {
		t.Fatal("expected an error")
	}

	if g.calls != 1 {
		t.Errorf("Forbidden must not be retried, got %d calls", g.calls)
	}
}

func TestLoadRetriesTransientErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "server unavailable", err: apierrors.NewServiceUnavailable("try later")},
		{name: "connection refused", err: &url.Error{Op: "Get", URL: "https://api", Err: errors.New("connection refused")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
				func() (*v1alpha1.FencingSLAProfile, error) { return nil, tt.err },
				func() (*v1alpha1.FencingSLAProfile, error) { return validProfile(), nil },
			}}

			if _, err := load(t.Context(), g, v1alpha1.ProfileMedium, log.NewNop(), time.Millisecond); err != nil {
				t.Fatalf("load after a transient error: %v", err)
			}

			if g.calls != 2 {
				t.Errorf("expected exactly 2 calls, got %d", g.calls)
			}
		})
	}
}

func TestLoadStopsOnContextExpiry(t *testing.T) {
	g := &getterStub{results: []func() (*v1alpha1.FencingSLAProfile, error){
		func() (*v1alpha1.FencingSLAProfile, error) {
			return nil, apierrors.NewServiceUnavailable("try later")
		},
	}}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if _, err := load(ctx, g, v1alpha1.ProfileMedium, log.NewNop(), 5*time.Millisecond); err == nil {
		t.Fatal("expected an error after the budget expired")
	}
}
