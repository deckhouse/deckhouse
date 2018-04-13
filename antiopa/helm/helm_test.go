package helm

import (
	"fmt"
	"github.com/romana/rlog"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	uuid "gopkg.in/satori/go.uuid.v1"
	v1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/rbac/v1beta1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

func getTestDirectoryPath(testName string) string {
	_, testFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(testFile), "testdata", testName)
}

func shouldDeleteRelease(helm HelmClient, releaseName string) (err error) {
	err = helm.DeleteRelease(releaseName)
	if err != nil {
		return fmt.Errorf("Should delete existing release '%s' successfully, got error: %s", releaseName, err)
	}
	isExists, err := helm.IsReleaseExists(releaseName)
	if err != nil {
		return err
	}
	if isExists {
		return fmt.Errorf("Release '%s' should not exist after deletion", releaseName)
	}

	return nil
}

func releasesListShouldEqual(helm HelmClient, expectedList []string) (err error) {
	releases, err := helm.ListReleases()
	if err != nil {
		return err
	}

	sortedExpectedList := make([]string, len(expectedList))
	copy(sortedExpectedList, expectedList)
	sort.Strings(sortedExpectedList)

	if !reflect.DeepEqual(sortedExpectedList, releases) {
		return fmt.Errorf("Expected %+v releases list, got %+v", expectedList, releases)
	}

	return nil
}

func shouldUpgradeRelease(helm HelmClient, releaseName string, chart string, valuesPaths []string) (err error) {
	err = helm.UpgradeRelease(releaseName, chart, []string{}, helm.TillerNamespace())
	if err != nil {
		return fmt.Errorf("Cannot install test release: %s", err)
	}
	isExists, err := helm.IsReleaseExists(releaseName)
	if err != nil {
		return err
	}
	if !isExists {
		return fmt.Errorf("Release '%s' should exist", releaseName)
	}
	return nil
}

func TestHelm(t *testing.T) {
	// Для теста требуется kubernetes + helm, поэтому skip
	t.Skip()

	var err error
	var stdout, stderr string
	var isExists bool
	var releases []string

	helm := &CliHelm{tillerNamespace: fmt.Sprintf("antiopa-test-%s", uuid.NewV4())}
	rlog.Infof("Testing tiller in '%s' namespace", helm.TillerNamespace())

	kube.InitKube()
	kube.KubernetesAntiopaNamespace = helm.TillerNamespace()

	testNs := &v1.Namespace{}
	testNs.Name = helm.TillerNamespace()
	_, err = kube.KubernetesClient.CoreV1().Namespaces().Create(testNs)
	if err != nil {
		t.Fatal(err)
	}

	sa := &v1.ServiceAccount{}
	sa.Name = "tiller"
	_, err = kube.KubernetesClient.CoreV1().ServiceAccounts(helm.TillerNamespace()).Create(sa)
	if err != nil {
		t.Fatal(err)
	}

	role := &v1beta1.Role{}
	role.Name = "tiller-role"
	role.Rules = []v1beta1.PolicyRule{
		v1beta1.PolicyRule{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
	}
	_, err = kube.KubernetesClient.RbacV1beta1().Roles(helm.TillerNamespace()).Create(role)
	if err != nil {
		t.Fatal(err)
	}

	rb := &v1beta1.RoleBinding{}
	rb.Name = "tiller-binding"
	rb.RoleRef.Kind = "Role"
	rb.RoleRef.Name = "tiller-role"
	rb.RoleRef.APIGroup = "rbac.authorization.k8s.io"
	rb.Subjects = []v1beta1.Subject{
		v1beta1.Subject{Kind: "ServiceAccount", Name: "tiller", Namespace: helm.TillerNamespace()},
	}
	_, err = kube.KubernetesClient.RbacV1beta1().RoleBindings(helm.TillerNamespace()).Create(rb)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err = helm.Cmd("init", "--upgrade", "--wait", "--service-account", "tiller")
	if err != nil {
		t.Errorf("Cannot init test tiller in '%s' namespace: %s\n%s %s", helm.TillerNamespace(), err, stdout, stderr)
	}

	releases, err = helm.ListReleases()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual([]string{}, releases) {
		t.Errorf("Expected empty releases list, got: %+v", releases)
	}

	_, _, err = helm.LastReleaseStatus("asfd")
	if err == nil {
		t.Error(err)
	}
	isExists, err = helm.IsReleaseExists("asdf")
	if err != nil {
		t.Error(err)
	}
	if isExists {
		t.Errorf("Release '%s' should not exist", "asdf")
	}
	err = helm.DeleteRelease("asdf")
	if err == nil {
		t.Errorf("Should fail when trying to delete unexisting release '%s'", "asdf")
	}

	err = shouldUpgradeRelease(helm, "test-redis", "stable/redis", []string{})
	if err != nil {
		t.Error(err)
	}

	err = shouldUpgradeRelease(helm, "test-local-chart", filepath.Join(getTestDirectoryPath("test_helm"), "chart"), []string{})
	if err != nil {
		t.Error(err)
	}

	err = releasesListShouldEqual(helm, []string{"test-local-chart", "test-redis"})
	if err != nil {
		t.Error(err)
	}

	err = shouldDeleteRelease(helm, "test-redis")
	if err != nil {
		t.Error(err)
	}

	err = releasesListShouldEqual(helm, []string{"test-local-chart"})
	if err != nil {
		t.Error(err)
	}

	err = shouldDeleteRelease(helm, "test-local-chart")
	if err != nil {
		t.Error(err)
	}

	err = releasesListShouldEqual(helm, []string{})
	if err != nil {
		t.Error(err)
	}

	err = helm.UpgradeRelease("hello", "no-such-chart", []string{}, helm.TillerNamespace())
	if err == nil {
		t.Errorf("Expected helm upgrade to fail, got no error from helm client")
	}
}
