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

package etcd

import (
	"net"
	"path/filepath"
	"strconv"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
)

// GetPeerURL creates an HTTPS URL that uses the configured advertise
// address and peer port for the API controller
func GetPeerURL(ip string) string {
	return "https://" + net.JoinHostPort(ip, strconv.Itoa(constants.EtcdListenPeerPort))
}

// GetStaticPodFilepath returns the location on the disk where the Static Pod should be present
func GetStaticPodFilepath(componentName, manifestsDir string) string {
	return filepath.Join(manifestsDir, componentName+".yaml")
}
