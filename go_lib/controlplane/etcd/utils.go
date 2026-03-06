package etcd

import (
	"net"
	"path/filepath"
	"strconv"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
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
