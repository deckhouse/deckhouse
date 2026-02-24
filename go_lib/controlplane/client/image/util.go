package image

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/etcdconfig"
	"k8s.io/klog"
)

// GetEtcdImage generates and returns the image for etcd
func GetEtcdImage(config *etcdconfig.EtcdConfig) string {
	// Etcd uses default image repository by default
	etcdImageRepository := config.ImageRepository
	// unless an override is specified
	if config.LocalEtcd != nil && config.LocalEtcd.ImageRepository != "" {
		etcdImageRepository = config.LocalEtcd.ImageRepository
	}
	etcdImageTag := GetEtcdImageTag(config)
	return GetGenericImage(etcdImageRepository, constants.Etcd, etcdImageTag)
}

// GetEtcdImageTag generates and returns the image tag for etcd
func GetEtcdImageTag(config *etcdconfig.EtcdConfig) string {
	// Etcd uses an imageTag that corresponds to the etcd version matching the Kubernetes version
	etcdImageTag := constants.DefaultEtcdVersion
	etcdVersion, warning, err := constants.EtcdSupportedVersion(constants.SupportedEtcdVersion, config.KubernetesVersion)
	if err == nil {
		etcdImageTag = etcdVersion.String()
	}
	if warning != nil {
		klog.V(1).Infof("WARNING: %v", warning)
	}
	// unless an override is specified
	if config.LocalEtcd != nil && config.LocalEtcd.ImageTag != "" {
		etcdImageTag = config.LocalEtcd.ImageTag
	}
	return etcdImageTag
}

// GetGenericImage generates and returns a platform agnostic image (backed by manifest list)
func GetGenericImage(prefix, image, tag string) string {
	return fmt.Sprintf("%s/%s:%s", prefix, image, tag)
}
