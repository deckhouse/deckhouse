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

package capi

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	capiNamespace                = "d8-cloud-instance-manager"
	cloudProviderSecretName      = common.CloudProviderSecretName
	cloudProviderSecretNamespace = common.CloudProviderSecretNamespace
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	// Aliased from internal/common, which owns the read of this object.
	clusterUUIDConfigMapName = common.ClusterUUIDConfigMapName
	clusterUUIDConfigMapNS   = common.ClusterUUIDConfigMapNamespace
)

type BaseWithReader struct {
	register.Base
	APIReader client.Reader
	// Cache backs the deferred InstanceClass watches (common.LazyInstanceClassSource): the
	// kind and version are data in the provider registration Secret, which may appear only
	// after this controller started.
	Cache cache.Cache
}

func (b *BaseWithReader) Setup(_ context.Context, mgr ctrl.Manager) error {
	b.APIReader = mgr.GetAPIReader()
	b.Cache = mgr.GetCache()
	return nil
}

func newUnstructured(group, version, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return u
}
