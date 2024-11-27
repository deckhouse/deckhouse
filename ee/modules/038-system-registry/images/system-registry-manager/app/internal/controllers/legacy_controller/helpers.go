/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import corev1 "k8s.io/api/core/v1"

func hasMasterLabel(node *corev1.Node) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	return isMaster
}
