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

package registry

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scache "k8s.io/client-go/tools/cache"
)

func TestRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Suite")
}

type testRestRegistryParams struct {
	hitCache bool
}

type testRegistryFixture struct {
	cache        k8scache.ThreadSafeStore
	restRegistry *REST
	params       *testRestRegistryParams
	resourceName string
	renderVal    runtime.Object
	cacheVal     runtime.Object
}

func setupTestRestRegistry(p *testRestRegistryParams) *testRegistryFixture {
	cachesManager := NewCachesManager()
	cache := cachesManager.GetCache()

	renderVal := &fakeObj{v: "render"}
	cacheVal := &fakeObj{v: "cache"}

	resourceName := "some"

	if p.hitCache {
		cache.Add(resourceName, renderVal)
	}

	render := &testTemplateRender{
		returnedVal: renderVal,
		errVal:      nil,
	}

	restRegistry := NewREST(render, cache)

	return &testRegistryFixture{
		resourceName: resourceName,
		cache:        cache,
		restRegistry: restRegistry,
		params:       p,
		renderVal:    renderVal,
		cacheVal:     cacheVal,
	}
}

var _ = Describe("Module :: node-manager :: bashible-apiserver :: rest registry", func() {
	Context("getting", func() {
		It("returns from cache if cache contains object", func() {
			fixtures := setupTestRestRegistry(&testRestRegistryParams{
				hitCache: true,
			})

			obj, _ := fixtures.restRegistry.Get(context.TODO(), fixtures.resourceName, &v1.GetOptions{})

			Expect(obj).To(BeEquivalentTo(fixtures.renderVal))
		})

		Context("cache missing", func() {
			It("returns from renderer if cache miss", func() {
				fixtures := setupTestRestRegistry(&testRestRegistryParams{
					hitCache: false,
				})

				obj, _ := fixtures.restRegistry.Get(context.TODO(), fixtures.resourceName, &v1.GetOptions{})
				Expect(obj).To(BeEquivalentTo(fixtures.renderVal))
			})

			It("sets obj from renderer to cache if cache miss", func() {
				fixtures := setupTestRestRegistry(&testRestRegistryParams{
					hitCache: false,
				})

				_, err := fixtures.restRegistry.Get(context.TODO(), fixtures.resourceName, &v1.GetOptions{})
				cachedObj, exists := fixtures.cache.Get(fixtures.resourceName)

				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
				Expect(cachedObj).To(BeEquivalentTo(fixtures.renderVal))
			})
		})
	})
})

type testTemplateRender struct {
	returnedVal runtime.Object
	errVal      error
}

func (r *testTemplateRender) Render(_ string) (runtime.Object, error) {
	return r.returnedVal, r.errVal
}

func (r *testTemplateRender) New() runtime.Object {
	return nil
}

func (r *testTemplateRender) NewList() runtime.Object {
	return nil
}

type fakeObj struct {
	v string
}
type fakeObjKind struct{}

func (f *fakeObj) GetObjectKind() schema.ObjectKind {
	return &fakeObjKind{}
}

func (f *fakeObj) DeepCopyObject() runtime.Object {
	return f
}

func (f *fakeObjKind) SetGroupVersionKind(kind schema.GroupVersionKind) {}
func (f *fakeObjKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "Baz"}
}
