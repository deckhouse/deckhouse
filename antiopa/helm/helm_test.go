package helm

import (
	"fmt"
	"github.com/romana/rlog"
	"reflect"
	"testing"

	uuid "gopkg.in/satori/go.uuid.v1"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/antiopa/kube"
)

// Для теста требуется kubernetes + helm, поэтому skip
func TestHelm(t *testing.T) {
	t.Skip()

	var releaseName string
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
		t.Error(err)
	}

	stdout, stderr, err = helm.Cmd("init", "--upgrade", "--wait")
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

	releaseName = "asdf"
	_, _, err = helm.LastReleaseStatus(releaseName)
	if err == nil {
		t.Error(err)
	}
	isExists, err = helm.IsReleaseExists(releaseName)
	if err != nil {
		t.Error(err)
	}
	if isExists {
		t.Errorf("Release '%s' should not exist", releaseName)
	}
	err = helm.DeleteRelease(releaseName)
	if err == nil {
		t.Errorf("Should fail when trying to delete unexisting release '%s'", releaseName)
	}

	releaseName = "some-module"
	stdout, stderr, err = helm.Cmd("install", "stable/redis", "--name", releaseName, "--namespace", helm.TillerNamespace())
	if err != nil {
		t.Errorf("Cannot install test release: %s\n%s %s", err, stdout, stderr)
	}
	isExists, err = helm.IsReleaseExists(releaseName)
	if err != nil {
		t.Error(err)
	}
	if !isExists {
		t.Errorf("Release '%s' should exist", releaseName)
	}

	releases, err = helm.ListReleases()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual([]string{"some-module"}, releases) {
		t.Errorf("Got unexpected releases list: %+v", releases)
	}

	err = helm.DeleteRelease(releaseName)
	if err != nil {
		t.Errorf("Should succeed when trying to delete existing release '%s', got error: %s", releaseName, err)
	}
	isExists, err = helm.IsReleaseExists(releaseName)
	if err != nil {
		t.Error(err)
	}
	if isExists {
		t.Errorf("Release '%s' should not exist after deletion", releaseName)
	}

	releases, err = helm.ListReleases()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual([]string{}, releases) {
		t.Errorf("Expected empty releases list, got: %+v", releases)
	}

}
