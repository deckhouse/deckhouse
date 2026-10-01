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

package validation

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// recordingRegisterer records the paths handlers are registered on.
type recordingRegisterer struct {
	paths map[string]http.Handler
}

func (r *recordingRegisterer) Register(path string, handler http.Handler) {
	r.paths[path] = handler
}

// TestRegisterAdmissionHandlersV2 pins the Module v2 route set: a route the chart points at with
// failurePolicy Fail must be served, and the addon-operator-only Module guard must not be.
func TestRegisterAdmissionHandlersV2(t *testing.T) {
	reg := &recordingRegisterer{paths: make(map[string]http.Handler)}
	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))

	RegisterAdmissionHandlersV2(
		reg,
		fake.NewClientBuilder().Build(),
		&fakeModulePackageManager{},
		metricStorage,
		config.NewSchemaStore(nil),
		helpers.NewDeckhouseSettingsContainer(helpers.DefaultDeckhouseSettings(), metricStorage),
		nil,
	)

	for _, path := range []string{
		"/validate/v1/deckhouse-registry-secret",
		"/validate/v1alpha1/module-configs",
		"/validate/v1beta1/modules",
		"/validate/v1/configuration-secret",
		"/validate/v1/provider-configuration-secret",
		"/validate/v1/static-configuration-secret",
		"/validate/v1alpha1/update-policies",
		"/validate/v1alpha1/deckhouse-releases",
		"/validate/v1alpha1/applications",
	} {
		require.Contains(t, reg.paths, path)
		assert.NotNil(t, reg.paths[path], path)
	}

	assert.NotContains(t, reg.paths, "/validate/v1alpha1/modules", "the v1 Module guard forbids every user edit")
}
