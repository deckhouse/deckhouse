/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "bashible-apiserver/pkg/apis/bashible/v1alpha1"
	bashiblev1alpha1 "bashible-apiserver/pkg/generated/applyconfiguration/bashible/v1alpha1"
	"context"
	json "encoding/json"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBootstraps implements BootstrapInterface
type FakeBootstraps struct {
	Fake *FakeBashibleV1alpha1
}

var bootstrapsResource = schema.GroupVersionResource{Group: "bashible.deckhouse.io", Version: "v1alpha1", Resource: "bootstraps"}

var bootstrapsKind = schema.GroupVersionKind{Group: "bashible.deckhouse.io", Version: "v1alpha1", Kind: "Bootstrap"}

// Get takes name of the bootstrap, and returns the corresponding bootstrap object, and an error if there is any.
func (c *FakeBootstraps) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Bootstrap, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(bootstrapsResource, name), &v1alpha1.Bootstrap{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Bootstrap), err
}

// List takes label and field selectors, and returns the list of Bootstraps that match those selectors.
func (c *FakeBootstraps) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.BootstrapList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(bootstrapsResource, bootstrapsKind, opts), &v1alpha1.BootstrapList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.BootstrapList{ListMeta: obj.(*v1alpha1.BootstrapList).ListMeta}
	for _, item := range obj.(*v1alpha1.BootstrapList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested bootstraps.
func (c *FakeBootstraps) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(bootstrapsResource, opts))
}

// Create takes the representation of a bootstrap and creates it.  Returns the server's representation of the bootstrap, and an error, if there is any.
func (c *FakeBootstraps) Create(ctx context.Context, bootstrap *v1alpha1.Bootstrap, opts v1.CreateOptions) (result *v1alpha1.Bootstrap, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(bootstrapsResource, bootstrap), &v1alpha1.Bootstrap{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Bootstrap), err
}

// Update takes the representation of a bootstrap and updates it. Returns the server's representation of the bootstrap, and an error, if there is any.
func (c *FakeBootstraps) Update(ctx context.Context, bootstrap *v1alpha1.Bootstrap, opts v1.UpdateOptions) (result *v1alpha1.Bootstrap, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(bootstrapsResource, bootstrap), &v1alpha1.Bootstrap{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Bootstrap), err
}

// Delete takes name of the bootstrap and deletes it. Returns an error if one occurs.
func (c *FakeBootstraps) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(bootstrapsResource, name, opts), &v1alpha1.Bootstrap{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBootstraps) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(bootstrapsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.BootstrapList{})
	return err
}

// Patch applies the patch and returns the patched bootstrap.
func (c *FakeBootstraps) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Bootstrap, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(bootstrapsResource, name, pt, data, subresources...), &v1alpha1.Bootstrap{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Bootstrap), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied bootstrap.
func (c *FakeBootstraps) Apply(ctx context.Context, bootstrap *bashiblev1alpha1.BootstrapApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.Bootstrap, err error) {
	if bootstrap == nil {
		return nil, fmt.Errorf("bootstrap provided to Apply must not be nil")
	}
	data, err := json.Marshal(bootstrap)
	if err != nil {
		return nil, err
	}
	name := bootstrap.Name
	if name == nil {
		return nil, fmt.Errorf("bootstrap.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(bootstrapsResource, *name, types.ApplyPatchType, data), &v1alpha1.Bootstrap{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Bootstrap), err
}
