/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

// AccessibleNamespaceStorage implements REST storage for AccessibleNamespace.
// This is a read-only, computed resource that returns namespaces accessible to the requesting user.
//
// LIMITATIONS:
// - Watch is NOT supported - clients must poll for updates
// - resourceVersion is always empty ("") - the list is computed at request time
// - This is similar to OpenShift's Project API concept
type AccessibleNamespaceStorage struct {
	resolver *resolver.NamespaceResolver
}

// NewAccessibleNamespaceStorage creates a new storage for AccessibleNamespace.
func NewAccessibleNamespaceStorage(nsResolver *resolver.NamespaceResolver) *AccessibleNamespaceStorage {
	return &AccessibleNamespaceStorage{
		resolver: nsResolver,
	}
}

// Interface compliance
var _ rest.Lister = &AccessibleNamespaceStorage{}
var _ rest.Getter = &AccessibleNamespaceStorage{}
var _ rest.Scoper = &AccessibleNamespaceStorage{}
var _ rest.Storage = &AccessibleNamespaceStorage{}
var _ rest.SingularNameProvider = &AccessibleNamespaceStorage{}

// New returns a new AccessibleNamespace
func (s *AccessibleNamespaceStorage) New() runtime.Object {
	return &v1alpha1.AccessibleNamespace{}
}

// Destroy cleans up resources on shutdown
func (s *AccessibleNamespaceStorage) Destroy() {}

// NamespaceScoped returns false because AccessibleNamespace is cluster-scoped
func (s *AccessibleNamespaceStorage) NamespaceScoped() bool {
	return false
}

// GetSingularName returns the singular name of the resource
func (s *AccessibleNamespaceStorage) GetSingularName() string {
	return "accessiblenamespace"
}

// NewList returns a new AccessibleNamespaceList
func (s *AccessibleNamespaceStorage) NewList() runtime.Object {
	return &v1alpha1.AccessibleNamespaceList{}
}

// List returns all namespaces accessible to the requesting user.
// The list is computed at request time based on RBAC and multi-tenancy rules.
// resourceVersion is always empty - watch is not supported.
func (s *AccessibleNamespaceStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		klog.V(4).Info("AccessibleNamespaceStorage.List: no user info in context")
		return &v1alpha1.AccessibleNamespaceList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "AccessibleNamespaceList",
			},
			ListMeta: metav1.ListMeta{
				ResourceVersion: "", // Explicitly empty - watch not supported
			},
			Items: []v1alpha1.AccessibleNamespace{},
		}, nil
	}

	klog.V(4).Infof("AccessibleNamespaceStorage.List: resolving namespaces for user=%s groups=%v",
		userInfo.GetName(), userInfo.GetGroups())

	namespaces, err := s.resolver.ResolveAccessibleNamespaces(userInfo)
	if err != nil {
		klog.Errorf("AccessibleNamespaceStorage.List: failed to resolve namespaces: %v", err)
		return nil, errors.NewInternalError(err)
	}

	items := make([]v1alpha1.AccessibleNamespace, len(namespaces))
	for i, ns := range namespaces {
		items[i] = v1alpha1.AccessibleNamespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "AccessibleNamespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				// ResourceVersion is intentionally empty
			},
		}
	}

	klog.V(4).Infof("AccessibleNamespaceStorage.List: returning %d accessible namespaces for user=%s",
		len(items), userInfo.GetName())

	return &v1alpha1.AccessibleNamespaceList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "AccessibleNamespaceList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "", // Explicitly empty - watch not supported
		},
		Items: items,
	}, nil
}

// Get returns a single AccessibleNamespace if accessible, or NotFound error.
// This avoids namespace existence disclosure - unauthorized namespaces return NotFound.
func (s *AccessibleNamespaceStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		klog.V(4).Infof("AccessibleNamespaceStorage.Get(%s): no user info in context", name)
		return nil, errors.NewNotFound(v1alpha1.Resource("accessiblenamespaces"), name)
	}

	klog.V(4).Infof("AccessibleNamespaceStorage.Get(%s): checking access for user=%s", name, userInfo.GetName())

	accessible, err := s.resolver.IsNamespaceAccessible(userInfo, name)
	if err != nil {
		klog.Errorf("AccessibleNamespaceStorage.Get(%s): failed to check access: %v", name, err)
		return nil, errors.NewInternalError(err)
	}

	if !accessible {
		// Return NotFound to avoid existence disclosure
		klog.V(4).Infof("AccessibleNamespaceStorage.Get(%s): namespace not accessible for user=%s", name, userInfo.GetName())
		return nil, errors.NewNotFound(v1alpha1.Resource("accessiblenamespaces"), name)
	}

	klog.V(4).Infof("AccessibleNamespaceStorage.Get(%s): namespace is accessible for user=%s", name, userInfo.GetName())

	return &v1alpha1.AccessibleNamespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "AccessibleNamespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			// ResourceVersion is intentionally empty
		},
	}, nil
}

// ConvertToTable implements the TableConvertor interface for kubectl get output.
func (s *AccessibleNamespaceStorage) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table

	table.ColumnDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Namespace name"},
	}

	switch t := object.(type) {
	case *v1alpha1.AccessibleNamespace:
		table.Rows = []metav1.TableRow{
			{
				Cells:  []interface{}{t.Name},
				Object: runtime.RawExtension{Object: t},
			},
		}
	case *v1alpha1.AccessibleNamespaceList:
		table.Rows = make([]metav1.TableRow, len(t.Items))
		for i, item := range t.Items {
			table.Rows[i] = metav1.TableRow{
				Cells:  []interface{}{item.Name},
				Object: runtime.RawExtension{Object: &t.Items[i]},
			}
		}
	}

	return &table, nil
}

