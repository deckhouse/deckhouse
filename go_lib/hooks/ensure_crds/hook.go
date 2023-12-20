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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryYaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"

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
		result := EnsureCRDs(crdsGlob, dc)

		if result.ErrorOrNil() != nil {
			input.LogEntry.WithError(result).Error("ensure_crds failed")
		}

		return result.ErrorOrNil()
	}
}

func EnsureCRDs(crdsGlob string, dc dependency.Container) *multierror.Error {
	result := new(multierror.Error)

	client, err := dc.GetK8sClient()
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	cp, err := NewCRDsInstaller(client, crdsGlob)
	if err != nil {
		result = multierror.Append(result, err)
		return result
	}

	return cp.Run(context.TODO())
}

// CRDsInstaller simultaneously installs CRDs from specified directory
type CRDsInstaller struct {
	k8sClient    k8s.Client
	crdFilesPath []string
	buffer       []byte

	// concurrent tasks to create resource in a k8s cluster
	k8sTasks *multierror.Group
}

func (cp *CRDsInstaller) Run(ctx context.Context) *multierror.Error {
	result := new(multierror.Error)

	for _, crdFilePath := range cp.crdFilesPath {
		if match := strings.HasPrefix(filepath.Base(crdFilePath), "doc-"); match {
			continue
		}

		err := cp.processCRD(ctx, crdFilePath)
		if err != nil {
			err = fmt.Errorf("error occurred during processing %q file: %w", crdFilePath, err)
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

func (cp *CRDsInstaller) processCRD(ctx context.Context, crdFilePath string) error {
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

		data := cp.buffer[:n]
		if len(data) == 0 {
			// some empty yaml document, or empty string before separator
			continue
		}
		rd := bytes.NewReader(data)
		err = cp.putCRDToCluster(ctx, rd, n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cp *CRDsInstaller) putCRDToCluster(ctx context.Context, crdReader io.Reader, bufferSize int) error {
	var crd *v1.CustomResourceDefinition

	err := apimachineryYaml.NewYAMLOrJSONDecoder(crdReader, bufferSize).Decode(&crd)
	if err != nil {
		return err
	}

	// it could be a comment or some other peace of yaml file, skip it
	if crd == nil {
		return nil
	}

	if crd.APIVersion != v1.SchemeGroupVersion.String() && crd.Kind != "CustomResourceDefinition" {
		return fmt.Errorf("invalid CRD document apiversion/kind: '%s/%s'", crd.APIVersion, crd.Kind)
	}

	cp.k8sTasks.Go(func() error {
		return cp.updateOrInsertCRD(ctx, crd)
	})

	return nil
}

func (cp *CRDsInstaller) updateOrInsertCRD(ctx context.Context, crd *v1.CustomResourceDefinition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existCRD, err := cp.getCRDFromCluster(ctx, crd.GetName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				ucrd, err := sdk.ToUnstructured(crd)
				if err != nil {
					return err
				}

				_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Create(ctx, ucrd, apimachineryv1.CreateOptions{})
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

		_, err = cp.k8sClient.Dynamic().Resource(crdGVR).Update(ctx, ucrd, apimachineryv1.UpdateOptions{})
		return err
	})
}

func (cp *CRDsInstaller) getCRDFromCluster(ctx context.Context, crdName string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}

	o, err := cp.k8sClient.Dynamic().Resource(crdGVR).Get(ctx, crdName, apimachineryv1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = sdk.FromUnstructured(o, &crd)
	if err != nil {
		return nil, err
	}

	return crd, nil
}

// NewCRDsInstaller creates new installer for CRDs
// crdsGlob example: "/deckhouse/modules/002-deckhouse/crds/*.yaml"
func NewCRDsInstaller(client k8s.Client, crdsGlob string) (*CRDsInstaller, error) {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return nil, err
	}

	return &CRDsInstaller{
		k8sClient:    client,
		crdFilesPath: crds,
		// 1Mb - maximum size of kubernetes object
		// if we take less, we have to handle io.ErrShortBuffer error and increase the buffer
		// take more does not make any sense due to kubernetes limitations
		buffer:   make([]byte, 1*1024*1024),
		k8sTasks: &multierror.Group{},
	}, nil
}
