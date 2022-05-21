/*
Copyright 2022 Flant JSC

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

package ensure_crds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/hashicorp/go-multierror"
	yamlv3 "gopkg.in/yaml.v3"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func RegisterEnsureCRDsHook(crdsGlob string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnStartup: &go_hook.OrderedConfig{Order: 5},
	}, dependency.WithExternalDependencies(EnsureCRDsHandler(crdsGlob)))
}

func EnsureCRDsHandler(crdsGlob string) func(input *go_hook.HookInput, dc dependency.Container) error {
	return func(input *go_hook.HookInput, dc dependency.Container) error {
		result := new(multierror.Error)

		crds, err := filepath.Glob(crdsGlob)
		if err != nil {
			return err
		}

		for _, crdFilePath := range crds {
			if match := strings.HasPrefix(filepath.Base(crdFilePath), "doc-"); match {
				continue
			}

			content, err := loadCRDsFromFile(crdFilePath)
			if err != nil {
				result = multierror.Append(result, err)
				continue
			}

			crdYAMLs, err := splitYAML(content)
			if err != nil {
				result = multierror.Append(result, err)
				continue
			}

			for _, crdYAML := range crdYAMLs {
				if len(crdYAML) == 0 {
					continue
				}
				err = putCRDToCluster(input, dc, crdYAML)
				if err != nil {
					result = multierror.Append(result, err)
					continue
				}
			}
		}

		if result.ErrorOrNil() != nil {
			input.LogEntry.WithError(result).Error("ensure_crds failed")
		}

		return result.ErrorOrNil()
	}
}

func putCRDToCluster(input *go_hook.HookInput, dc dependency.Container, crdYAML []byte) error {

	var (
		crd            interface{}
		specConversion *apiextensions.CustomResourceConversion
	)

	res := &unstructured.Unstructured{}
	err := yaml.Unmarshal(crdYAML, &res)
	if err != nil {
		return err
	}

	c, err := getCRDFromCluster(dc, res.GetName())
	if err == nil && c.Spec.Conversion != nil {
		sc := &apiextensions.CustomResourceConversion{}
		err := v1.Convert_v1_CustomResourceConversion_To_apiextensions_CustomResourceConversion(c.Spec.Conversion, sc, nil)
		if err != nil {
			return err
		}
		specConversion = sc
	}

	switch res.GetAPIVersion() {
	case v1.SchemeGroupVersion.String():
		crdv1 := &v1.CustomResourceDefinition{}
		err = sdk.FromUnstructured(res, crdv1)
		if err != nil {
			return err
		}
		if specConversion != nil {
			c := &v1.CustomResourceConversion{}
			err := v1.Convert_apiextensions_CustomResourceConversion_To_v1_CustomResourceConversion(specConversion, c, nil)
			if err != nil {
				return err
			}
			crdv1.Spec.Conversion = c
		}
		crd = crdv1
	case v1beta1.SchemeGroupVersion.String():
		crdv1beta1 := &v1beta1.CustomResourceDefinition{}
		err = sdk.FromUnstructured(res, crdv1beta1)
		if err != nil {
			return err
		}
		if specConversion != nil {
			c := &v1beta1.CustomResourceConversion{}
			err := v1beta1.Convert_apiextensions_CustomResourceConversion_To_v1beta1_CustomResourceConversion(specConversion, c, nil)
			if err != nil {
				return err
			}
			crdv1beta1.Spec.Conversion = c
		}
		crd = crdv1beta1
	default:
		return fmt.Errorf("unsupported crd apiversion: %v", res.GetAPIVersion())
	}

	input.PatchCollector.Create(crd, object_patch.UpdateIfExists())
	return nil
}

func getCRDFromCluster(dc dependency.Container, crdName string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return nil, err
	}

	o, err := k8sClient.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).Get(context.TODO(), crdName, apimachineryv1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = sdk.FromUnstructured(o, &crd)
	if err != nil {
		return nil, err
	}

	return crd, nil
}

func splitYAML(resources []byte) ([][]byte, error) {

	dec := yamlv3.NewDecoder(bytes.NewReader(resources))

	var res [][]byte
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		valueBytes, err := yamlv3.Marshal(value)
		if err != nil {
			return nil, err
		}
		res = append(res, valueBytes)
	}
	return res, nil
}

func loadCRDsFromFile(crdFilePath string) ([]byte, error) {
	crdFile, err := os.Open(crdFilePath)
	if err != nil {
		return nil, err
	}

	defer crdFile.Close()

	content, err := ioutil.ReadAll(crdFile)
	if err != nil {
		return nil, err
	}

	return content, nil
}
