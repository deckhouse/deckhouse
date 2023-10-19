// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apis

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type kubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	ApiExt() apiextv1.ApiextensionsV1Interface
}

// EnsureCRDs installs or update primary CRDs for deckhouse-controller
func EnsureCRDs(ctx context.Context, client kubeClient, crdsGlob string) error {
	crds, err := filepath.Glob(crdsGlob)
	if err != nil {
		return err
	}

	for _, crdFilePath := range crds {
		if match := strings.HasPrefix(filepath.Base(crdFilePath), "doc-"); match {
			continue
		}

		rc, err := loadCRDFromFile(crdFilePath)
		if err != nil {
			return err
		}

		err = processCRDFileContent(ctx, client, rc)
		if err != nil {
			return err
		}
	}

	return nil
}

func processCRDFileContent(ctx context.Context, client kubeClient, rc io.ReadCloser) error {
	defer rc.Close()

	return putCRDToCluster(ctx, client, rc)
}

func loadCRDFromFile(crdFilePath string) (io.ReadCloser, error) {
	crdFile, err := os.Open(crdFilePath)
	if err != nil {
		return nil, err
	}

	return crdFile, nil
}

func putCRDToCluster(ctx context.Context, client kubeClient, crdReader io.Reader) error {
	var (
		crd *v1.CustomResourceDefinition
	)

	err := yaml.NewYAMLToJSONDecoder(crdReader).Decode(&crd)
	if err != nil {
		return err
	}

	oldCrd, err := getCRDFromCluster(ctx, client, crd.GetName())
	if err == nil && oldCrd.Spec.Conversion != nil {
		sc := &apiextensions.CustomResourceConversion{}
		err := v1.Convert_v1_CustomResourceConversion_To_apiextensions_CustomResourceConversion(oldCrd.Spec.Conversion, sc, nil)
		if err != nil {
			return err
		}
	}

	if apierrors.IsNotFound(err) {
		_, err = client.ApiExt().CustomResourceDefinitions().Create(ctx, crd, apimachineryv1.CreateOptions{})
		return err
	}

	if reflect.DeepEqual(oldCrd.Spec, crd.Spec) {
		return nil
	}

	oldCrd.Spec = crd.Spec

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = client.ApiExt().CustomResourceDefinitions().Update(ctx, oldCrd, apimachineryv1.UpdateOptions{})
		return err
	})

	return retryErr
}

func getCRDFromCluster(ctx context.Context, client kubeClient, crdName string) (*v1.CustomResourceDefinition, error) {
	crd, err := client.ApiExt().CustomResourceDefinitions().Get(ctx, crdName, apimachineryv1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return crd, nil
}
