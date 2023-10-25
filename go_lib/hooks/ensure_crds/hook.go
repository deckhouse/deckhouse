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
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"k8s.io/client-go/util/retry"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryYaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var (
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
)

func RegisterEnsureCRDsHook(crdsGlob string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnStartup: &go_hook.OrderedConfig{Order: 5},
	}, dependency.WithExternalDependencies(EnsureCRDsHandler(crdsGlob)))
}

func EnsureCRDsHandler(crdsGlob string) func(input *go_hook.HookInput, dc dependency.Container) error {
	return func(input *go_hook.HookInput, dc dependency.Container) error {
		result := EnsureCRDs(crdsGlob, input, dc)

		if result.ErrorOrNil() != nil {
			input.LogEntry.WithError(result).Error("ensure_crds failed")
		}

		return result.ErrorOrNil()
	}
}

func EnsureCRDs(crdsGlob string, input *go_hook.HookInput, dc dependency.Container) *multierror.Error {
	result := new(multierror.Error)

	client, err := dc.GetK8sClient()
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	cp := newCRDsProcessor(client, crds)

	return cp.Run()
}

type crdsProcessor struct {
	k8sClient    k8s.Client
	crdFilesPath []string
	buffer       []byte

	// concurrent tasks to create resource in a k8s cluster
	k8sTasks *multierror.Group
}

func (cp *crdsProcessor) Run() *multierror.Error {
	result := new(multierror.Error)

	for _, crdFilePath := range cp.crdFilesPath {
		if match := strings.HasPrefix(filepath.Base(crdFilePath), "doc-"); match {
			continue
		}

		err := cp.processCRD(crdFilePath)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	errs := cp.k8sTasks.Wait()
	if errs.ErrorOrNil() != nil {
		result = multierror.Append(result, errs.Errors...)
	}

	return result
}

func (cp *crdsProcessor) processCRD(crdFilePath string) error {
	crdFileReader, err := os.Open(crdFilePath)
	if err != nil {
		return err
	}
	defer crdFileReader.Close()

	crdReader := apimachineryYaml.NewDocumentDecoder(crdFileReader)

	for {
		n, err := crdReader.Read(cp.buffer)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		rd := bytes.NewReader(cp.buffer[:n])
		err = cp.putCRDToCluster(rd, n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cp *crdsProcessor) putCRDToCluster(crdReader io.Reader, bufferSize int) error {
	var crd *v1.CustomResourceDefinition

	err := apimachineryYaml.NewYAMLOrJSONDecoder(crdReader, bufferSize).Decode(&crd)
	if err != nil {
		return err
	}
	if crd == nil || crd.APIVersion != v1.SchemeGroupVersion.String() {
		return fmt.Errorf("invalid CRD: %v", crd)
	}

	cp.k8sTasks.Go(func() error {
		return cp.updateOrInsertCRD(crd)
	})

	return nil
}

func (cp *crdsProcessor) updateOrInsertCRD(crd *v1.CustomResourceDefinition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existCRD, err := cp.getCRDFromCluster(crd.GetName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				ucrd, err := sdk.ToUnstructured(crd)
				if err != nil {
					return err
				}

				_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Create(context.TODO(), ucrd, apimachineryv1.CreateOptions{})
				return err
			}

			return err
		}

		if existCRD.Spec.Conversion != nil {
			crd.Spec.Conversion = existCRD.Spec.Conversion
		}

		if reflect.DeepEqual(existCRD.Spec, crd.Spec) {
			return nil
		}

		existCRD.Spec = crd.Spec

		ucrd, err := sdk.ToUnstructured(existCRD)
		if err != nil {
			return err
		}

		_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Update(context.TODO(), ucrd, apimachineryv1.UpdateOptions{})
		return err
	})
}

func (cp *crdsProcessor) getCRDFromCluster(crdName string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}

	o, err := cp.k8sClient.Dynamic().Resource(crdGVR).Get(context.TODO(), crdName, apimachineryv1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = sdk.FromUnstructured(o, &crd)
	if err != nil {
		return nil, err
	}

	return crd, nil
}

func newCRDsProcessor(client k8s.Client, paths []string) *crdsProcessor {
	return &crdsProcessor{
		k8sClient:    client,
		crdFilesPath: paths,
		// 1Mb - maximum size of kubernetes object
		// if we take less, we have to handle io.ErrShortBuffer error and increase the buffer
		// take more does not make any sense due to kubernetes limitations
		buffer:   make([]byte, 1*1024*1024),
		k8sTasks: &multierror.Group{},
	}
}
