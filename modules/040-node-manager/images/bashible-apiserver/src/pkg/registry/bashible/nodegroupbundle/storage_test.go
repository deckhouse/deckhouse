/*
Copyright 2023 Flant JSC

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

package nodegroupbundle

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNodegroupbundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nodegroupbundle Suite")
}

type testStorageParams struct {
	k8sVer string
}

type testStorageFixture struct {
	storage *StorageWithK8sBundles
}

func setupTestStorage(p *testStorageParams) *testStorageFixture {
	ctx := &testTemplateContext{
		returnedVal: map[string]interface{}{
			"kubernetesVersion": p.k8sVer,
		},
	}

	storage, err := NewStorage("", nil, ctx)
	Expect(err).To(BeNil())

	return &testStorageFixture{
		storage: storage,
	}
}

type testTemplateContext struct {
	returnedVal map[string]interface{}
	errVal      error
}

func (c *testTemplateContext) Get(_ string) (map[string]interface{}, error) {
	return c.returnedVal, c.errVal
}

func (c *testTemplateContext) GetBootstrapContext(string) (map[string]interface{}, error) {
	return c.returnedVal, c.errVal
}
