/*
Copyright 2021 Flant JSC

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

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NamespacedToCluster(namespaced PodLoggingConfig) ClusterLoggingConfig {
	return ClusterLoggingConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s_%s", namespaced.Namespace, namespaced.Name),
		},
		Spec: ClusterLoggingConfigSpec{
			Type:            SourceKubernetesPods,
			LabelFilters:    namespaced.Spec.LabelFilters,
			LogFilters:      namespaced.Spec.LogFilters,
			MultiLineParser: namespaced.Spec.MultiLineParser,

			KubernetesPods: KubernetesPodsSpec{
				NamespaceSelector: NamespaceSelector{MatchNames: []string{namespaced.Namespace}},
				LabelSelector:     namespaced.Spec.LabelSelector,
			},
			DestinationRefs: namespaced.Spec.ClusterDestinationRefs,
		},
		Status: ClusterLoggingConfigStatus{},
	}
}
