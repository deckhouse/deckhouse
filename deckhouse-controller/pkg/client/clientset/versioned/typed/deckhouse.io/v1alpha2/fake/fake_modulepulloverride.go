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
	"context"

	v1alpha2 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeModulePullOverrides implements ModulePullOverrideInterface
type FakeModulePullOverrides struct {
	Fake *FakeDeckhouseV1alpha2
}

var modulepulloverridesResource = v1alpha2.SchemeGroupVersion.WithResource("modulepulloverrides")

var modulepulloverridesKind = v1alpha2.SchemeGroupVersion.WithKind("ModulePullOverride")

// Get takes name of the modulePullOverride, and returns the corresponding modulePullOverride object, and an error if there is any.
func (c *FakeModulePullOverrides) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.ModulePullOverride, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(modulepulloverridesResource, name), &v1alpha2.ModulePullOverride{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ModulePullOverride), err
}

// List takes label and field selectors, and returns the list of ModulePullOverrides that match those selectors.
func (c *FakeModulePullOverrides) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.ModulePullOverrideList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(modulepulloverridesResource, modulepulloverridesKind, opts), &v1alpha2.ModulePullOverrideList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha2.ModulePullOverrideList{ListMeta: obj.(*v1alpha2.ModulePullOverrideList).ListMeta}
	for _, item := range obj.(*v1alpha2.ModulePullOverrideList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested modulePullOverrides.
func (c *FakeModulePullOverrides) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(modulepulloverridesResource, opts))
}

// Create takes the representation of a modulePullOverride and creates it.  Returns the server's representation of the modulePullOverride, and an error, if there is any.
func (c *FakeModulePullOverrides) Create(ctx context.Context, modulePullOverride *v1alpha2.ModulePullOverride, opts v1.CreateOptions) (result *v1alpha2.ModulePullOverride, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(modulepulloverridesResource, modulePullOverride), &v1alpha2.ModulePullOverride{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ModulePullOverride), err
}

// Update takes the representation of a modulePullOverride and updates it. Returns the server's representation of the modulePullOverride, and an error, if there is any.
func (c *FakeModulePullOverrides) Update(ctx context.Context, modulePullOverride *v1alpha2.ModulePullOverride, opts v1.UpdateOptions) (result *v1alpha2.ModulePullOverride, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(modulepulloverridesResource, modulePullOverride), &v1alpha2.ModulePullOverride{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ModulePullOverride), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeModulePullOverrides) UpdateStatus(ctx context.Context, modulePullOverride *v1alpha2.ModulePullOverride, opts v1.UpdateOptions) (*v1alpha2.ModulePullOverride, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(modulepulloverridesResource, "status", modulePullOverride), &v1alpha2.ModulePullOverride{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ModulePullOverride), err
}

// Delete takes name of the modulePullOverride and deletes it. Returns an error if one occurs.
func (c *FakeModulePullOverrides) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(modulepulloverridesResource, name, opts), &v1alpha2.ModulePullOverride{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeModulePullOverrides) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(modulepulloverridesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha2.ModulePullOverrideList{})
	return err
}

// Patch applies the patch and returns the patched modulePullOverride.
func (c *FakeModulePullOverrides) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.ModulePullOverride, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(modulepulloverridesResource, name, pt, data, subresources...), &v1alpha2.ModulePullOverride{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha2.ModulePullOverride), err
}