/*
Copyright 2026 Flant JSC

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

package nodebootstrap

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	templatesv1alpha1 "github.com/deckhouse/node-controller/api/templates.internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodeconfig"
)

// TemplateStorage serves NodeConfigTemplate: the config a machine an operator
// installs by hand needs, one object per NodeGroup that has such machines.
// Nothing is stored — every read renders from the cluster as it is now, so the
// bootstrap token in the answer is a live one and no backup ever holds it.
type TemplateStorage struct {
	rest.TableConvertor
	client client.Client
}

// NewTemplateStorage returns the storage the aggregated API server installs
// under nodeconfigtemplates. The client is read live, not from a cache: a
// template is read by hand, rarely, and must carry an unexpired token.
func NewTemplateStorage(cl client.Client) *TemplateStorage {
	return &TemplateStorage{
		TableConvertor: rest.NewDefaultTableConvertor(templateGroupResource()),
		client:         cl,
	}
}

func (s *TemplateStorage) New() k8sruntime.Object {
	return &templatesv1alpha1.NodeConfigTemplate{}
}

func (s *TemplateStorage) NewList() k8sruntime.Object {
	return &templatesv1alpha1.NodeConfigTemplateList{}
}

func (s *TemplateStorage) Destroy() {}

func (s *TemplateStorage) NamespaceScoped() bool { return false }

func (s *TemplateStorage) GetSingularName() string { return "nodeconfigtemplate" }

// Get renders the template of the NodeGroup with this name. A group whose nodes
// the cluster provisions itself has no template: answering with one would hand
// an operator a config no machine of that group will ever read.
func (s *TemplateStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (k8sruntime.Object, error) {
	ng := &v1.NodeGroup{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: name}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(templateGroupResource(), name)
		}
		return nil, fmt.Errorf("read NodeGroup %s: %w", name, err)
	}
	if !machineOwnedConfig(ng) {
		return nil, apierrors.NewNotFound(templateGroupResource(), name)
	}
	tokens, err := nodecommon.BootstrapTokens(ctx, s.client)
	if err != nil {
		return nil, err
	}
	return s.render(ctx, ng, tokens[ng.Name])
}

// List renders a template for every group that has hand-installed machines. A
// group that cannot be rendered fails the whole list: the same fail-closed rule
// the render itself follows, a half-list reads as "this group needs no machines".
func (s *TemplateStorage) List(ctx context.Context, _ *metainternalversion.ListOptions) (k8sruntime.Object, error) {
	groups := &v1.NodeGroupList{}
	if err := s.client.List(ctx, groups); err != nil {
		return nil, fmt.Errorf("list NodeGroups: %w", err)
	}

	tokens, err := nodecommon.BootstrapTokens(ctx, s.client)
	if err != nil {
		return nil, err
	}

	list := &templatesv1alpha1.NodeConfigTemplateList{}
	for i := range groups.Items {
		ng := &groups.Items[i]
		if !machineOwnedConfig(ng) {
			continue
		}
		template, err := s.render(ctx, ng, tokens[ng.Name])
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, *template)
	}
	return list, nil
}

// render fills in the cluster's half of the config and blanks the machine's:
// the interfaces and the disks are what the operator writes in, and a rendered
// guess (eth0 with DHCP, the first disk over 2Gi) handed over as a template is
// one nobody notices is wrong. The node name is theirs to pick too.
func (s *TemplateStorage) render(ctx context.Context, ng *v1.NodeGroup, bootstrapToken string) (*templatesv1alpha1.NodeConfigTemplate, error) {
	spec, err := nodeconfig.RenderBootstrapSpec(ctx, s.client, s.client, ng, "")
	if err != nil {
		return nil, fmt.Errorf("render the node config of %s: %w", ng.Name, err)
	}

	// Without it kubelet has nothing to present on first contact, and the
	// operator has nowhere else to take a token from.
	if bootstrapToken == "" {
		return nil, fmt.Errorf("no valid bootstrap token for NodeGroup %s", ng.Name)
	}
	spec.Kubelet.BootstrapToken = bootstrapToken

	spec.Network = internalv1alpha1.Network{}
	spec.Storage = internalv1alpha1.Storage{}

	return &templatesv1alpha1.NodeConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: ng.Name},
		Spec:       spec,
	}, nil
}

// machineOwnedConfig reports whether this group's nodes bring their own
// configuration. A static immutable node is provisioned by hand — the operator
// writes its network and disks on the machine — and the cluster knows neither.
func machineOwnedConfig(ng *v1.NodeGroup) bool {
	return ng.Spec.NodeType == v1.NodeTypeStatic && ng.Spec.SystemType == v1.SystemTypeImmutable
}

func templateGroupResource() schema.GroupResource {
	return schema.GroupResource{Group: templatesv1alpha1.GroupVersion.Group, Resource: templatesv1alpha1.NodeConfigTemplateResource}
}
