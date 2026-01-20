/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"bashible-apiserver/pkg/requestlog"
)

type TemplateStorage interface {
	// Render renders object by name
	Render(name string) (runtime.Object, error)

	// In order to comply with expected interfaces, we must at least implement rest.Creator and rest.Lister.
	// Instead, we just provide workarounds that are enough to serve pur purposes.
	New() runtime.Object
	NewList() runtime.Object
}

// REST implements a RESTStorage for API services against etcd
type REST struct {
	storage TemplateStorage
	cache   cache.ThreadSafeStore
}

// RESTBootstrap implements RESTStorage without caching
type RESTBootstrap struct {
	REST
}

func RESTInPeace(storage TemplateStorage, err error, cache cache.ThreadSafeStore) *REST {
	if err != nil {
		err = fmt.Errorf("unable to create REST storage for a resource due to %v, will die", err)
		panic(err)
	}
	return NewREST(storage, cache)
}

func RESTBootstrapInPeace(storage TemplateStorage, err error, cache cache.ThreadSafeStore) *RESTBootstrap {
	if err != nil {
		err = fmt.Errorf("unable to create REST storage for a resource due to %v, will die", err)
		panic(err)
	}
	return NewRESTBootstrap(storage, cache)
}

func NewREST(storage TemplateStorage, cache cache.ThreadSafeStore) *REST {
	return &REST{
		storage: storage,
		cache:   cache,
	}
}

func NewRESTBootstrap(storage TemplateStorage, cache cache.ThreadSafeStore) *RESTBootstrap {
	return &RESTBootstrap{
		REST{
			storage: storage,
			cache:   cache,
		},
	}
}

// --------------------------------------------------------------------------------
// Actually used methods
//

func (r *REST) GetSingularName() string { return "" }

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, exists := r.cache.Get(name)
	if !exists {
		var err error
		obj, err = r.storage.Render(name)

		if err != nil {
			requestlog.LogRenderResult(ctx, nil, exists, err)
			return nil, err // TODO form status error
		}
		r.cache.Add(name, obj)
	}

	runtimeObj := obj.(runtime.Object)
	requestlog.LogRenderResult(ctx, runtimeObj, exists, nil)

	return runtimeObj, nil
}

func (r *RESTBootstrap) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := r.storage.Render(name)
	if err != nil {
		requestlog.LogRenderResult(ctx, nil, false, err)
		return nil, err // TODO form status error
	}

	runtimeObj := obj.(runtime.Object)
	requestlog.LogRenderResult(ctx, runtimeObj, false, nil)

	return runtimeObj, nil
}

func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, nil
}

// --------------------------------------------------------------------------------
// Meaningful methods
//

func (r *REST) New() runtime.Object {
	return r.storage.New()
}

func (r *REST) Destroy() {}

func (r *REST) NamespaceScoped() bool {
	return false
}

// --------------------------------------------------------------------------------
// Helper methods
//

func (r *REST) forbidden() (runtime.Object, error) {
	return nil, fmt.Errorf("forbidden")
}

func (r *REST) forbiddenBool() (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("forbidden")
}
