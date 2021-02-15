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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

type TemplateStorage interface {
	// Render renders object by name
	Render(name string) (runtime.Object, error)

	// In order to comply with expected interfaces, we must at least implement rest.Creator and rest.Lister.
	// Instead, we just provide workarounds that are enough to serve pur purposes.
	New() runtime.Object
	NewList() runtime.Object
}

// RESTInPeace is just a simple function that panics on error. Otherwise returns
// the given storage object. It is meant to be a wrapper for bashible
// registries. One can use REST struct (above) in the first arg.
func RESTInPeace(storage TemplateStorage, err error) *REST {
	if err != nil {
		err = fmt.Errorf("unable to create REST storage for a resource due to %v, will die", err)
		panic(err)
	}
	return NewREST(storage)
}

// REST implements a RESTStorage for API services against etcd
type REST struct {
	storage TemplateStorage
}

func NewREST(storage TemplateStorage) *REST {
	return &REST{storage: storage}
}

// --------------------------------------------------------------------------------
// Actually used methods
//

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	// TODO cache me maybe https://github.com/deckhouse/deckhouse/issues/1291
	obj, err := r.storage.Render(name)
	if err != nil {
		return nil, err // TODO form status error
	}
	return obj, nil
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

// --------------------------------------------------------------------------------
// Nonsense methods
//

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(schema.GroupResource{Resource: "simple"}).ConvertToTable(ctx, obj, tableOptions)
}

func (r *REST) Delete(ctx context.Context, id string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("forbidden")
}

func (r *REST) NewList() runtime.Object {
	return r.storage.NewList()
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return obj, fmt.Errorf("forbidden")

}

func (r *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.forbiddenBool()
}

func (r *REST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *REST) Watcher() *watch.FakeWatcher {
	return nil
}
