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

package startup

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	crdGrantPolicies = "clusterresourcegrantpolicies.multitenancy.deckhouse.io"
	crdProjects      = "projects.deckhouse.io"
	crdTemplates     = "projecttemplates.deckhouse.io"
)

var testConvertedCRDs = []string{crdGrantPolicies, crdProjects, crdTemplates}

// TestCRDConversion_Wait_HoldsUntilWebhook covers the starts of a controller whose CRDs do not convert
// through the webhook yet: Wait holds while they do not and returns at the first check after the
// webhook-handler of the deckhouse module registers the conversion.
func TestCRDConversion_Wait_HoldsUntilWebhook(t *testing.T) {
	tests := []struct {
		name string
		crds []client.Object
		// expectedPending is what the check reports while the controller is held.
		expectedPending []string
	}{
		{
			// The module was disabled: the webhook-handler reset the conversion of the deleted
			// ConversionWebhook resources to None. Enabled again, the Helm release brings them back, and
			// the controller it rolls out waits for the webhook-handler to register them.
			name: "the module is enabled again",
			crds: []client.Object{
				newCRD(crdGrantPolicies, withStrategy(apiextensionsv1.NoneConverter)),
				newCRD(crdProjects, withStrategy(apiextensionsv1.NoneConverter)),
				newCRD(crdTemplates, withStrategy(apiextensionsv1.NoneConverter)),
			},
			expectedPending: []string{
				crdGrantPolicies + " (conversion strategy None)",
				crdProjects + " (conversion strategy None)",
				crdTemplates + " (conversion strategy None)",
			},
		},
		{
			// Upgrade from a release where ClusterResourceGrantPolicy had one version: the installer
			// added the new version and kept the CRD without a conversion, which the API server reads
			// as None. The CRDs that had a conversion already keep it.
			name: "upgrade from a release where a CRD had one version",
			crds: []client.Object{
				newCRD(crdGrantPolicies),
				newCRD(crdProjects, withStrategy(apiextensionsv1.WebhookConverter)),
				newCRD(crdTemplates, withStrategy(apiextensionsv1.WebhookConverter)),
			},
			expectedPending: []string{crdGrantPolicies + " (conversion strategy None)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				reader := newCRDReader(t, interceptor.Funcs{}, tt.crds...)
				sink := &levelSink{}
				c := newCRDConversion(reader, sink)

				pending, err := c.pending(t.Context())
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPending, pending)

				var waitErr error
				done := make(chan struct{})
				go func() {
					defer close(done)
					waitErr = c.Wait(t.Context())
				}()

				// Several checks later the conversion is not registered, and Wait still holds.
				time.Sleep(3 * testInterval)
				synctest.Wait()
				select {
				case <-done:
					require.FailNow(t, "Wait returned while a CRD converted without the webhook")
				default:
				}

				// The webhook-handler registers the conversion of every ConversionWebhook.
				for _, name := range testConvertedCRDs {
					registerWebhookConversion(t, reader, name)
				}
				start := time.Now()

				<-done
				require.NoError(t, waitErr)
				assert.Equal(t, testInterval, time.Since(start), "Wait returns at the next check")
				assert.Equal(t, []string{
					"hold the controller until the webhook-handler of the deckhouse module registers the conversion webhook of the CRDs",
					"the CRDs convert through the webhook",
				}, sink.infoLines(), "the hold is logged once, and so is its end")
			})
		})
	}
}

// TestCRDConversion_Wait_ReturnsAtOnce covers the starts with nothing to wait for.
func TestCRDConversion_Wait_ReturnsAtOnce(t *testing.T) {
	tests := []struct {
		name string
		crds []client.Object
	}{
		{
			name: "every CRD converts through the webhook",
			crds: []client.Object{
				newCRD(crdGrantPolicies, withStrategy(apiextensionsv1.WebhookConverter)),
				newCRD(crdProjects, withStrategy(apiextensionsv1.WebhookConverter)),
				newCRD(crdTemplates, withStrategy(apiextensionsv1.WebhookConverter)),
			},
		},
		{
			name: "a CRD that is not installed is not waited for",
			crds: []client.Object{
				newCRD(crdProjects, withStrategy(apiextensionsv1.WebhookConverter)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sink := &levelSink{}
				c := newCRDConversion(newCRDReader(t, interceptor.Funcs{}, tt.crds...), sink)

				start := time.Now()
				require.NoError(t, c.Wait(t.Context()))

				assert.Zero(t, time.Since(start))
				assert.Equal(t, []string{"the CRDs already convert through the webhook"}, sink.infoLines())
			})
		})
	}
}

// TestCRDConversion_Wait_GetFails checks that a failed check is retried and does not end the wait, and
// that the failures are logged at error level once for each new error text, with one line when the
// checks succeed again.
func TestCRDConversion_Wait_GetFails(t *testing.T) {
	tests := []struct {
		name string
		// errs are the errors of the failing checks, in order; the checks after them succeed.
		errs []string
		// expectedErrorLines is the number of lines logged at error level.
		expectedErrorLines int
	}{
		{
			name:               "the same error at every failed check",
			errs:               []string{"the API server is not reachable", "the API server is not reachable", "the API server is not reachable"},
			expectedErrorLines: 1,
		},
		{
			name:               "the error changes",
			errs:               []string{"the API server is not reachable", "the API server is not reachable", "the request timed out"},
			expectedErrorLines: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				failures := slices.Clone(tt.errs)
				reader := newCRDReader(t, interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if len(failures) > 0 {
							err := errors.New(failures[0])
							failures = failures[1:]
							return err
						}
						return c.Get(ctx, key, obj, opts...)
					},
				}, newCRD(crdProjects, withStrategy(apiextensionsv1.WebhookConverter)))
				sink := &levelSink{}
				c := newCRDConversion(reader, sink)

				start := time.Now()
				require.NoError(t, c.Wait(t.Context()))

				assert.Equal(t, time.Duration(len(tt.errs))*testInterval, time.Since(start), "Wait returns at the first check that succeeds")
				assert.Equal(t, tt.expectedErrorLines, sink.errorLines())
				assert.Equal(t, []string{
					"the conversion of the CRDs can be checked again",
					"the CRDs already convert through the webhook",
				}, sink.infoLines())
			})
		})
	}
}

// TestCRDConversion_Wait_Cancelled checks that Wait returns the error of its context when the
// process stops while it holds.
func TestCRDConversion_Wait_Cancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newCRDConversion(newCRDReader(t, interceptor.Funcs{}, newCRD(crdGrantPolicies)), &levelSink{})
		ctx, cancel := context.WithTimeout(t.Context(), 2*testInterval)
		defer cancel()

		err := c.Wait(ctx)

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func newCRDConversion(reader client.Reader, sink *levelSink) *CRDConversion {
	return &CRDConversion{
		Reader:   reader,
		Logger:   logr.New(sink),
		Names:    testConvertedCRDs,
		Interval: testInterval,
	}
}

func newCRDReader(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithInterceptorFuncs(funcs).Build()
}

// newCRD returns a CRD without a conversion, the way a CRD with one version is installed.
func newCRD(name string, mutators ...func(*apiextensionsv1.CustomResourceDefinition)) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name}}
	for _, mutate := range mutators {
		mutate(crd)
	}
	return crd
}

func withStrategy(strategy apiextensionsv1.ConversionStrategyType) func(*apiextensionsv1.CustomResourceDefinition) {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: strategy}
	}
}

// registerWebhookConversion does what the webhook-handler does for a ConversionWebhook: it sets the
// conversion of the CRD to its webhook.
func registerWebhookConversion(t *testing.T, c client.Client, name string) {
	t.Helper()
	crd := new(apiextensionsv1.CustomResourceDefinition)
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Name: name}, crd))
	withStrategy(apiextensionsv1.WebhookConverter)(crd)
	require.NoError(t, c.Update(t.Context(), crd))
}
