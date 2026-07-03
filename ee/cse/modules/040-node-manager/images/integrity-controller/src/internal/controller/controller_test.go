/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")
	golden     bool
	scheme     = runtime.NewScheme()
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deckhousev1alpha1.AddToScheme(scheme))
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestContainerdIntegrityPolicyControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ContainerdIntegrityPolicyControllerTestSuite))
}

type ContainerdIntegrityPolicyControllerTestSuite struct {
	suite.Suite

	ctx context.Context

	client      client.Client
	reconciler  *Reconciler
	policyNames []string

	testDataFileName string
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) TestReconcile() {
	suite.Run("When namespaces match selector", func() {
		suite.setupController(suite.fetchTestFileData("match-labels.yaml"))

		_, err := suite.reconcilePolicies()
		require.NoError(suite.T(), err)
	})
	suite.Run("When status is already up to date", func() {
		suite.setupController(suite.fetchTestFileData("status-already-synced.yaml"))

		_, err := suite.reconcilePolicies()
		require.NoError(suite.T(), err)
	})
	suite.Run("When no namespaces match selector", func() {
		suite.setupController(suite.fetchTestFileData("no-matching-namespaces.yaml"))

		_, err := suite.reconcilePolicies()
		require.NoError(suite.T(), err)
	})
	suite.Run("When status is stale", func() {
		suite.setupController(suite.fetchTestFileData("update-stale-status.yaml"))

		_, err := suite.reconcilePolicies()
		require.NoError(suite.T(), err)
	})
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	if suite.testDataFileName == "" {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0o666)
		require.NoError(suite.T(), err)
	} else {
		got := singleDocToManifests(gotB)
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := singleDocToManifests(expB)
		assert.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
		for i := range got {
			assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
		}
	}
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) setupController(yamlDoc string) {
	ctx := context.Background()

	manifests := singleDocToManifests([]byte(yamlDoc))
	initObjects := make([]client.Object, 0, len(manifests))
	policyNames := make([]string, 0)

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)

		if policy, ok := obj.(*deckhousev1alpha1.ContainerdIntegrityPolicy); ok {
			policyNames = append(policyNames, policy.Name)
		}
	}

	sort.Strings(policyNames)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjects...).
		WithStatusSubresource(&deckhousev1alpha1.ContainerdIntegrityPolicy{}).
		Build()

	suite.reconciler = &Reconciler{
		Client: k8sClient,
		Scheme: scheme,
	}
	suite.client = k8sClient
	suite.ctx = ctx
	suite.policyNames = policyNames
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) reconcilePolicies() (reconcile.Result, error) {
	var lastResult reconcile.Result
	var lastErr error

	for _, name := range suite.policyNames {
		result, err := suite.reconciler.Reconcile(suite.ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name},
		})
		if err != nil {
			return result, err
		}

		lastResult = result
		lastErr = err
	}

	return lastResult, lastErr
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) assembleInitObject(strObj string) client.Object {
	var res client.Object
	metaType := new(runtime.TypeMeta)

	err := yaml.Unmarshal([]byte(strObj), &metaType)
	require.NoError(suite.T(), err)

	switch metaType.Kind {
	case "Namespace":
		namespace := new(corev1.Namespace)
		err = yaml.Unmarshal([]byte(strObj), namespace)
		require.NoError(suite.T(), err)
		res = namespace
	case "ContainerdIntegrityPolicy":
		policy := new(deckhousev1alpha1.ContainerdIntegrityPolicy)
		err = yaml.Unmarshal([]byte(strObj), policy)
		require.NoError(suite.T(), err)
		res = policy
	default:
		suite.T().Fatalf("unsupported kind: %s", metaType.Kind)
	}

	return res
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) fetchTestFileData(filename string) string {
	dir := "./testdata/cases"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return string(data)
}

func (suite *ContainerdIntegrityPolicyControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	policies := new(deckhousev1alpha1.ContainerdIntegrityPolicyList)
	require.NoError(suite.T(), suite.client.List(suite.ctx, policies))

	sort.Slice(policies.Items, func(i, j int) bool {
		return policies.Items[i].Name < policies.Items[j].Name
	})

	for _, policy := range policies.Items {
		policy := policy.DeepCopy()

		policy.ResourceVersion = ""
		policy.UID = ""
		policy.ManagedFields = nil
		policy.Generation = 0

		got, err := yaml.Marshal(policy)
		require.NoError(suite.T(), err)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func singleDocToManifests(doc []byte) []string {
	split := mDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(split))
	for i := range split {
		if split[i] != "" {
			s := strings.TrimSpace(split[i])
			if s != "" {
				result = append(result, s)
			}
		}
	}

	return result
}
