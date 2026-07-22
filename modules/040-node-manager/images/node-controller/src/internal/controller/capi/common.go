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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	capiNamespace                = "d8-cloud-instance-manager"
	cloudProviderSecretName      = "d8-node-manager-cloud-provider"
	cloudProviderSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	clusterUUIDConfigMapName     = "d8-cluster-uuid"
	clusterUUIDConfigMapNS       = "kube-system"
)

type BaseWithReader struct {
	register.Base
	APIReader  client.Reader
	RestConfig *rest.Config
	Clientset  kubernetes.Interface
}

func (b *BaseWithReader) Setup(mgr ctrl.Manager) error {
	b.APIReader = mgr.GetAPIReader()
	b.RestConfig = mgr.GetConfig()

	// Typed clientset for the kubeconfig CertificateSigningRequest flow.
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	b.Clientset = cs
	return nil
}

func newUnstructured(group, version, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return u
}
