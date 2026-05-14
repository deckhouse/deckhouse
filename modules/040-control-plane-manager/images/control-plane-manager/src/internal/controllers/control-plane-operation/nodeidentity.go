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

package controlplaneoperation

import (
	"fmt"
	"os"

	"control-plane-manager/internal/constants"
)

// NodeIdentity holds node-level env and configuration once at startup.
type NodeIdentity struct {
	Name                string
	AdvertiseIP         string
	ClusterDomain       string
	ServiceSubnetCIDR   string
	HomeDir             string
	KubeconfigDir       string
	NodeAdminKubeconfig bool
}

func nodeIdentityFromEnv() (NodeIdentity, error) {
	name := os.Getenv(constants.NodeNameEnvVar)
	if name == "" {
		return NodeIdentity{}, fmt.Errorf("env %s is not set", constants.NodeNameEnvVar)
	}

	ip := os.Getenv("MY_IP")
	if ip == "" {
		return NodeIdentity{}, fmt.Errorf("env MY_IP is not set")
	}

	kubeconfigDir := os.Getenv(constants.KubeconfigDirEnvVar)
	if kubeconfigDir == "" {
		kubeconfigDir = constants.KubernetesConfigPath
	}

	homeDir, _ := os.LookupEnv("HOME")

	nodeAdminKubeconfig := true
	if v, ok := os.LookupEnv("NODE_ADMIN_KUBECONFIG"); ok && v == "false" {
		nodeAdminKubeconfig = false
	}

	return NodeIdentity{
		Name:                name,
		AdvertiseIP:         ip,
		ClusterDomain:       os.Getenv("CLUSTER_DOMAIN"),
		ServiceSubnetCIDR:   os.Getenv("SERVICE_SUBNET_CIDR"),
		HomeDir:             homeDir,
		KubeconfigDir:       kubeconfigDir,
		NodeAdminKubeconfig: nodeAdminKubeconfig,
	}, nil
}
