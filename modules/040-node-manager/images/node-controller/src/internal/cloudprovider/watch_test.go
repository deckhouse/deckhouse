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

package cloudprovider

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsInputSecret(t *testing.T) {
	for _, testCase := range []struct {
		namespace string
		name      string
		want      bool
	}{
		{namespace: "kube-system", name: "d8-node-manager-cloud-provider", want: true},
		{namespace: "kube-system", name: "d8-cluster-configuration", want: true},
		{namespace: "kube-system", name: "d8-cloud-provider-openstack-capi", want: true},
		{namespace: "kube-system", name: "d8-cloud-provider-aws-mcm", want: true},
		{namespace: "d8-system", name: "d8-node-manager-cloud-provider", want: false},
		{namespace: "kube-system", name: "other", want: false},
	} {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: testCase.namespace, Name: testCase.name}}
		require.Equal(t, testCase.want, IsInputSecret(secret))
	}
}
