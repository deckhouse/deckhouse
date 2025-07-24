/*
Copyright 2025 Flant JSC

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

package releaseupdater

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	mDelimiter     = regexp.MustCompile("(?m)^---$")
	goldenMigrated bool
)

func init() {
	flag.BoolVar(&goldenMigrated, "golden", false, "generate golden files")
}

type MigratedModulesGoldenTestSuite struct {
	suite.Suite
	kubeClient       client.Client
	metricStorage    *metricstorage.MetricStorage
	exts             *extenders.ExtendersStack
	logger           *log.Logger
	enabledModules   []string
	testDataFileName string
}

func (suite *MigratedModulesGoldenTestSuite) SetupSuite() {
	suite.logger = log.NewNop()
	suite.enabledModules = []string{"prometheus", "cert-manager"}
	suite.metricStorage = metricstorage.NewMetricStorage(context.Background(), "", false, suite.logger)
	suite.exts = extenders.NewExtendersStack(nil, "", suite.logger)
}

func (suite *MigratedModulesGoldenTestSuite) SetupTest() {
	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	suite.kubeClient = fake.NewClientBuilder().WithScheme(sc).Build()
}

func (suite *MigratedModulesGoldenTestSuite) TearDownSubTest() {
	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	got := suite.fetchResults()

	if goldenMigrated {
		err := os.WriteFile(goldenFile, got, 0666)
		require.NoError(suite.T(), err)
	} else {
		gotManifests := suite.singleDocToManifests(got)
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		expManifests := suite.singleDocToManifests(expB)

		assert.Equal(suite.T(), len(expManifests), len(gotManifests), "different number of manifests")
		for i := range gotManifests {
			if i < len(expManifests) {
				assert.YAMLEq(suite.T(), expManifests[i], gotManifests[i], "manifest %d differs", i)
			}
		}
	}
}

func (suite *MigratedModulesGoldenTestSuite) singleDocToManifests(doc []byte) []string {
	manifests := mDelimiter.Split(string(doc), -1)
	result := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		if strings.TrimSpace(manifest) != "" {
			result = append(result, manifest)
		}
	}
	return result
}

func (suite *MigratedModulesGoldenTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	// List all DeckhouseReleases
	var releases v1alpha1.DeckhouseReleaseList
	err := suite.kubeClient.List(context.TODO(), &releases)
	require.NoError(suite.T(), err)

	for _, release := range releases.Items {
		got, _ := yaml.Marshal(release)
		result.WriteString("---\n")
		result.Write(got)
	}

	// List all ModuleSources
	var moduleSources v1alpha1.ModuleSourceList
	err = suite.kubeClient.List(context.TODO(), &moduleSources)
	require.NoError(suite.T(), err)

	for _, ms := range moduleSources.Items {
		got, _ := yaml.Marshal(ms)
		result.WriteString("---\n")
		result.Write(got)
	}

	// List all Secrets (like cluster configuration)
	var secrets corev1.SecretList
	err = suite.kubeClient.List(context.TODO(), &secrets)
	require.NoError(suite.T(), err)

	for _, secret := range secrets.Items {
		got, _ := yaml.Marshal(secret)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *MigratedModulesGoldenTestSuite) setupController(filename string) (*Checker[v1alpha1.DeckhouseRelease], error) {
	// Read YAML file with test data
	yamlDoc := suite.fetchTestFileData(filename)

	// Split into separate manifests
	manifests := suite.singleDocToManifests([]byte(yamlDoc))

	// Create Kubernetes objects
	initObjects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		if obj != nil {
			initObjects = append(initObjects, obj)
		}
	}

	// Create fake Kubernetes client with objects
	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	suite.kubeClient = fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(initObjects...).
		WithStatusSubresource(&v1alpha1.DeckhouseRelease{}).
		Build()

	// Create cluster configuration secret if not present
	suite.createClusterConfigSecret()

	// Create requirements checker
	return NewDeckhouseReleaseRequirementsChecker(suite.kubeClient, suite.enabledModules, suite.exts, suite.metricStorage, suite.logger)
}

func (suite *MigratedModulesGoldenTestSuite) createClusterConfigSecret() {
	clusterConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cluster-configuration",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte("kubernetesVersion: \"1.29\""),
		},
	}
	_ = suite.kubeClient.Create(context.TODO(), clusterConfigSecret)
}

func (suite *MigratedModulesGoldenTestSuite) fetchTestFileData(filename string) string {
	data, err := os.ReadFile(filepath.Join("./testdata", filename))
	require.NoError(suite.T(), err)
	return string(data)
}

func (suite *MigratedModulesGoldenTestSuite) assembleInitObject(objStr string) client.Object {
	var typ runtime.TypeMeta
	err := yaml.Unmarshal([]byte(objStr), &typ)
	require.NoError(suite.T(), err)

	var res client.Object
	switch typ.Kind {
	case "DeckhouseRelease":
		res = suite.unmarshalDeckhouseRelease(objStr)
	case "ModuleSource":
		res = suite.unmarshalModuleSource(objStr)
	case "Secret":
		res = suite.unmarshalSecret(objStr)
	default:
		suite.T().Fatalf("unknown object kind: %s", typ.Kind)
	}
	return res
}

func (suite *MigratedModulesGoldenTestSuite) unmarshalDeckhouseRelease(objStr string) *v1alpha1.DeckhouseRelease {
	var obj v1alpha1.DeckhouseRelease
	err := yaml.Unmarshal([]byte(objStr), &obj)
	require.NoError(suite.T(), err)
	return &obj
}

func (suite *MigratedModulesGoldenTestSuite) unmarshalModuleSource(objStr string) *v1alpha1.ModuleSource {
	var obj v1alpha1.ModuleSource
	err := yaml.Unmarshal([]byte(objStr), &obj)
	require.NoError(suite.T(), err)
	return &obj
}

func (suite *MigratedModulesGoldenTestSuite) unmarshalSecret(objStr string) *corev1.Secret {
	var obj corev1.Secret
	err := yaml.Unmarshal([]byte(objStr), &obj)
	require.NoError(suite.T(), err)
	return &obj
}

func (suite *MigratedModulesGoldenTestSuite) getDeckhouseRelease() *v1alpha1.DeckhouseRelease {
	var release v1alpha1.DeckhouseRelease
	err := suite.kubeClient.Get(context.TODO(), client.ObjectKey{Name: "v1.50.0"}, &release)
	require.NoError(suite.T(), err)
	return &release
}

// Test cases

func (suite *MigratedModulesGoldenTestSuite) TestNoMigratedModules() {
	suite.Run("NoMigratedModules", func() {
		suite.testDataFileName = "no-migrated-modules.yaml"

		checker, err := suite.setupController("no-migrated-modules.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Empty(suite.T(), reasons, "Release without migratedModules should pass")
	})
}

func (suite *MigratedModulesGoldenTestSuite) TestEmptyMigratedModules() {
	suite.Run("EmptyMigratedModules", func() {
		suite.testDataFileName = "empty-migrated-modules.yaml"

		checker, err := suite.setupController("empty-migrated-modules.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Empty(suite.T(), reasons, "Release with empty migratedModules should pass")
	})
}

func (suite *MigratedModulesGoldenTestSuite) TestModulesAvailable() {
	suite.Run("ModulesAvailable", func() {
		suite.testDataFileName = "modules-available.yaml"

		checker, err := suite.setupController("modules-available.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Empty(suite.T(), reasons, "Release with all modules available should pass")
	})
}

func (suite *MigratedModulesGoldenTestSuite) TestModuleMissing() {
	suite.Run("ModuleMissing", func() {
		suite.testDataFileName = "module-missing.yaml"

		checker, err := suite.setupController("module-missing.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Len(suite.T(), reasons, 1, "Should have one requirement failure")
		assert.Equal(suite.T(), "migrated modules check", reasons[0].Reason)
		assert.Contains(suite.T(), reasons[0].Message, "test-module-missing")
	})
}

func (suite *MigratedModulesGoldenTestSuite) TestModulePullError() {
	suite.Run("ModulePullError", func() {
		suite.testDataFileName = "module-pull-error.yaml"

		checker, err := suite.setupController("module-pull-error.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Len(suite.T(), reasons, 1, "Should have one requirement failure")
		assert.Equal(suite.T(), "migrated modules check", reasons[0].Reason)
		assert.Contains(suite.T(), reasons[0].Message, "test-module-1")
	})
}

func (suite *MigratedModulesGoldenTestSuite) TestMultipleSources() {
	suite.Run("MultipleSources", func() {
		suite.testDataFileName = "multiple-sources.yaml"

		checker, err := suite.setupController("multiple-sources.yaml")
		require.NoError(suite.T(), err)

		release := suite.getDeckhouseRelease()
		reasons := checker.MetRequirements(context.TODO(), release)

		assert.Empty(suite.T(), reasons, "Module should be found in second source")
	})
}

func TestMigratedModulesGolden(t *testing.T) {
	suite.Run(t, new(MigratedModulesGoldenTestSuite))
}
