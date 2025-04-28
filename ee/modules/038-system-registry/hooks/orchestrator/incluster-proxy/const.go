/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	PodVersionAnnotation = "registry.deckhouse.io/incluster-proxy-version"
)

var (
	InClusterProxyPodsMatchLabels = &v1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "registry-incluster-proxy",
		},
	}
)
