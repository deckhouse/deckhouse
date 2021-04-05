package nodegroupbundle

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"bashible-apiserver/pkg/apis/bashible"
	"bashible-apiserver/pkg/template"
)

// NewStorage returns a RESTStorage object that will work against API services.
func NewStorage(rootDir string, bashibleContext template.Context) (*StorageWithK8sBundles, error) {
	ngRenderer := template.NewStepsRenderer(bashibleContext, rootDir, "node-group", template.GetNodegroupContextKey)
	k8sRenderer := template.NewStepsRenderer(bashibleContext, rootDir, "all", template.GetVersionContextKey)

	return &StorageWithK8sBundles{
		ngRenderer:      ngRenderer,
		k8sRenderer:     k8sRenderer,
		bashibleContext: bashibleContext,
	}, nil
}

type StorageWithK8sBundles struct {
	ngRenderer  *template.StepsRenderer
	k8sRenderer *template.StepsRenderer
	bashibleContext template.Context
}

// Render renders single script content by name which is expected to be of form {bundle}.{node-group-name}
// with hyphens as delimiters, e.g. `ubuntu-lts.master`.
func (s StorageWithK8sBundles) Render(name string) (runtime.Object, error) {
	ngBundleData, err := s.ngRenderer.Render(name)
	if err != nil {
		return nil, err
	}

	k8sBundleName, err := s.getK8sBundleName(name)
	if err != nil {
		return nil, err
	}

	k8sBundleData, err := s.k8sRenderer.Render(k8sBundleName)
	if err != nil {
		return nil, err
	}

	data, err := s.merge(ngBundleData, k8sBundleData)
	if err != nil {
		return nil, err
	}

	obj := bashible.NodeGroupBundle{}
	obj.ObjectMeta.Name = name
	obj.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now())
	obj.Data = data

	return &obj, nil
}

func (s StorageWithK8sBundles) New() runtime.Object {
	return &bashible.NodeGroupBundle{}
}

func (s StorageWithK8sBundles) NewList() runtime.Object {
	return &bashible.NodeGroupBundleList{}
}

func (s StorageWithK8sBundles) getK8sBundleName(name string) (string, error) {
	contextKey, err := template.GetNodegroupContextKey(name)
	if err != nil {
		return "", err
	}

	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return "", err
	}

	versionObj, versionPresent := context["kubernetesVersion"]
	if !versionPresent {
		return "", fmt.Errorf("kubernetesVersion does not present in bundle context %s", contextKey)
	}
	k8sVer := strings.ReplaceAll(versionObj.(string), ".", "-")

	os, _, err := template.ParseName(name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", os, k8sVer), nil
}

func (s StorageWithK8sBundles) merge(ngBundleData, k8sBundleData map[string]string) (map[string]string, error) {
	for k, v := range k8sBundleData {
		if _, keyPresent := ngBundleData[k]; keyPresent {
			return nil, fmt.Errorf("%s already present in node-group bundle", k)
		}
		ngBundleData[k] = v
	}

	return ngBundleData, nil
}
