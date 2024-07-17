package deckhouse_release

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"text/template"

	"github.com/Masterminds/semver/v3"
	"github.com/Masterminds/sprig/v3"
	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

func TestReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseTestSuite))
}

type ReleaseTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *deckhouseReleaseReconciler
}

func (suite *ReleaseTestSuite) SetupSuite() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	dependency.TestDC.CRClient = cr.NewClientMock(suite.T())
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)
}

func (suite *ReleaseTestSuite) setupController(values string, mup *v1alpha1.ModuleUpdatePolicySpec) {
	ds := &helpers.DeckhouseSettings{
		ReleaseChannel: mup.ReleaseChannel,
	}
	ds.Update.Mode = mup.Update.Mode
	ds.Update.Windows = mup.Update.Windows
	ds.Update.DisruptionApprovalMode = "Auto"

	suite.setupControllerSettings(values, ds)
}

func (suite *ReleaseTestSuite) setupControllerSettings(values string, ds *helpers.DeckhouseSettings) {
	yamlDoc := suite.fetchTestFileData(values)
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
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
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *ReleaseTestSuite) assembleInitObject(obj string) client.Object {
	var res client.Object
	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(suite.T(), err)

	switch typ.Kind {
	case "Secret":
		res = unmarshalRelease[corev1.Secret](obj, suite)
	case "Pod":
		res = unmarshalRelease[corev1.Pod](obj, suite)
	case "Deployment":
		res = unmarshalRelease[appsv1.Deployment](obj, suite)
	case "DeckhouseRelease":
		res = unmarshalRelease[v1alpha1.DeckhouseRelease](obj, suite)
	case "ConfigMap":
		res = unmarshalRelease[corev1.ConfigMap](obj, suite)

	default:
		require.Fail(suite.T(), "unknown Kind:"+typ.Kind)
	}

	return res
}

func (suite *ReleaseTestSuite) fetchTestFileData(valuesJSON string) string {
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
		Parse(deckhouseDiscovery + deckhouseRegistry)
	require.NoError(suite.T(), err)

	var values any
	err = json.Unmarshal([]byte(valuesJSON), &values)
	require.NoError(suite.T(), err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{"Values": values})
	require.NoError(suite.T(), err)

	return buf.String()
}

func unmarshalRelease[T any](manifest string, suite *ReleaseTestSuite) *T {
	var obj T
	err := yaml.Unmarshal([]byte(manifest), &obj)
	require.NoError(suite.T(), err)
	return &obj
}

func (suite *ReleaseTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var releaseList v1alpha1.DeckhouseReleaseList
	err := suite.kubeClient.List(context.TODO(), &releaseList)
	require.NoError(suite.T(), err)

	for _, item := range releaseList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var podsList corev1.PodList
	err = suite.kubeClient.List(context.TODO(), &podsList)
	require.NoError(suite.T(), err)

	for _, item := range podsList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var deploymentList appsv1.DeploymentList
	err = suite.kubeClient.List(context.TODO(), &deploymentList)
	require.NoError(suite.T(), err)

	for _, item := range deploymentList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var cmList corev1.ConfigMapList
	err = suite.kubeClient.List(context.TODO(), &cmList)
	require.NoError(suite.T(), err)

	for _, item := range cmList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}
func (suite *ReleaseTestSuite) TestCheckRelease() {
	var initValues = `{
 "global": {
   "modulesImages": {
     "registry": {
       "base": "my.registry.com/deckhouse"
     }
   },
   "discovery": {
     "clusterUUID": "21da7734-77a7-45ad-a795-ea0b629ee930"
   }
 },
 "deckhouse":{
   "bundle": "Default",
   "releaseChannel": "Stable",
   "internal":{
     "releaseVersionImageHash":"zxczxczxc"
   }
 }
}`

	suite.Run("CheckNextVersion", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{
			"v1.31.0",
			"v1.31.1",
			"v1.32.0",
			"v1.32.1",
			"v1.32.2",
			"v1.32.3",
			"v1.33.0",
			"v1.33.1",
		}, nil)

		suite.setupController(initValues, embeddedMUP)

		rc, err := NewDeckhouseReleaseChecker([]cr.Option{}, suite.ctr.logger, suite.ctr.dc,
			suite.ctr.moduleManager, "", "")
		require.NoError(suite.T(), err)

		var v *semver.Version
		actual := semver.New(1, 31, 0, "", "")
		target := semver.New(1, 31, 1, "", "")
		v, err = rc.nextVersion(
			actual,
			target,
		)
		require.NoError(suite.T(), err)

		if !cmp.Equal(v, target) {
			suite.T().Fatalf("version is not equal: %v", cmp.Diff(v, target))
		}
	})
}
