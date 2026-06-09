/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

var testDeckhouseVersion = "v1.15.0"

func setupFakeController(
	t *testing.T,
	filename, values string,
	mup *v1alpha2.ModuleUpdatePolicySpec,
	options ...reconcilerOption,
) (*deckhouseReleaseReconciler, client.Client) {
	ds := &helpers.DeckhouseSettings{
		ReleaseChannel: mup.ReleaseChannel,
	}
	ds.Update.Mode = mup.Update.Mode
	ds.Update.Windows = mup.Update.Windows
	ds.Update.DisruptionApprovalMode = "Auto"
	return setupControllerSettings(t, filename, values, ds, options...)
}

type reconcilerOption func(r *deckhouseReleaseReconciler)

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *deckhouseReleaseReconciler) {
		r.dc = dc
	}
}

func setupControllerSettings(
	t *testing.T,
	filename,
	values string,
	ds *helpers.DeckhouseSettings,
	options ...reconcilerOption,
) (*deckhouseReleaseReconciler, client.Client) {
	yamlDoc := fetchTestFileData(t, filename, values)

	sc, err := project.Scheme()
	require.NoError(t, err)

	initObjects, err := reconcilertest.Decode(sc, []byte(yamlDoc))
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(initObjects...).
		WithStatusSubresource(&v1alpha1.DeckhouseRelease{}).
		Build()
	dc := dependency.NewDependencyContainer()
	metricStorage := metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop()))
	rec := &deckhouseReleaseReconciler{
		client:           cl,
		deckhouseVersion: testDeckhouseVersion,
		dc:               dc,
		logger:           log.NewNop(),
		moduleManager:    stubModulesManager{},
		updateSettings:   helpers.NewDeckhouseSettingsContainer(ds, metricStorage),
		metricStorage:    metricStorage,
		metricsUpdater:   releaseUpdater.NewMetricsUpdater(metricstorage.NewMetricStorage(metricstorage.WithNewRegistry(), metricstorage.WithLogger(log.NewNop())), releaseUpdater.D8ReleaseBlockedMetricName),
		exts:             extenders.NewExtendersStack(new(d8edition.Edition), nil, log.NewNop()),
	}
	rec.clusterUUID = rec.getClusterUUID(context.Background())

	for _, option := range options {
		option(rec)
	}

	return rec, cl
}

func fetchTestFileData(t *testing.T, filename, valuesJSON string) string {
	data := []byte("")
	if filename != "" {
		dir := "./testdata"
		var err error
		data, err = os.ReadFile(filepath.Join(dir, filename))
		require.NoError(t, err)
	}

	deckhouseDiscovery := `---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-discovery
  namespace: d8-system
type: Opaque
data:
{{- if $.Values.global.discovery.clusterUUID }}
  clusterUUID: {{ $.Values.global.discovery.clusterUUID | b64enc }}
{{- end }}
`

	deckhouseRegistry := `---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
data:
  clusterIsBootstrapped: {{ .Values.global.clusterIsBootstrapped | quote | b64enc }}
  imagesRegistry: {{ b64enc .Values.global.modulesImages.registry.base }}
`

	deckhouseClusterConfiguration := `---
{{- $k8sv := cat "kubernetesVersion:" ( .Values.global.clusterConfiguration.kubernetesVersion | quote ) }}
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: {{ $k8sv | b64enc }}
`
	tmpl, err := template.New("manifest").
		Funcs(sprig.TxtFuncMap()).
		Parse(string(data) + deckhouseDiscovery + deckhouseRegistry + deckhouseClusterConfiguration)
	require.NoError(t, err)

	var values any
	err = json.Unmarshal([]byte(valuesJSON), &values)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{"Values": values})
	require.NoError(t, err)

	return buf.String()
}
