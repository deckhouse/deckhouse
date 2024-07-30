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
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func setupFakeController(
	t *testing.T,
	filename, values string,
	mup *v1alpha1.ModuleUpdatePolicySpec,
) (*deckhouseReleaseReconciler, client.Client) {
	ds := &helpers.DeckhouseSettings{
		ReleaseChannel: mup.ReleaseChannel,
	}
	ds.Update.Mode = mup.Update.Mode
	ds.Update.Windows = mup.Update.Windows
	ds.Update.DisruptionApprovalMode = "Auto"
	return setupControllerSettings(t, filename, values, ds)
}

func setupControllerSettings(
	t *testing.T,
	filename,
	values string,
	ds *helpers.DeckhouseSettings,
) (*deckhouseReleaseReconciler, client.Client) {
	yamlDoc := fetchTestFileData(t, filename, values)
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := assembleInitObject(t, manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = appsv1.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(initObjects...).
		WithStatusSubresource(&v1alpha1.DeckhouseRelease{}).
		Build()
	dc := dependency.NewDependencyContainer()

	rec := &deckhouseReleaseReconciler{
		client:         cl,
		dc:             dc,
		logger:         log.New(),
		moduleManager:  stubModulesManager{},
		updateSettings: helpers.NewDeckhouseSettingsContainer(ds),
		metricStorage:  metric_storage.NewMetricStorage(context.Background(), "", true),
	}

	return rec, cl
}

func assembleInitObject(t *testing.T, obj string) client.Object {
	var res client.Object
	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(t, err)

	switch typ.Kind {
	case "Secret":
		res = unmarshalRelease[corev1.Secret](obj, t)
	case "Pod":
		res = unmarshalRelease[corev1.Pod](obj, t)
	case "Deployment":
		res = unmarshalRelease[appsv1.Deployment](obj, t)
	case "DeckhouseRelease":
		res = unmarshalRelease[v1alpha1.DeckhouseRelease](obj, t)
	case "ConfigMap":
		res = unmarshalRelease[corev1.ConfigMap](obj, t)

	default:
		require.Fail(t, "unknown Kind:"+typ.Kind)
	}

	return res
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

	tmpl, err := template.New("manifest").
		Funcs(sprig.TxtFuncMap()).
		Parse(string(data) + deckhouseDiscovery + deckhouseRegistry)
	require.NoError(t, err)

	var values any
	err = json.Unmarshal([]byte(valuesJSON), &values)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{"Values": values})
	require.NoError(t, err)

	return buf.String()
}

func unmarshalRelease[T any](manifest string, t *testing.T) *T {
	var obj T
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(t, err)
	return &obj
}
