/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
)

const (
	authConfigPath         = "/etc/kubernetes/system-registry/auth_config/config.yaml"
	distributionConfigPath = "/etc/kubernetes/system-registry/distribution_config/config.yaml"
	pkiConfigDirectoryPath = "/etc/kubernetes/system-registry/pki"
	mirrorerConfigPath     = "/etc/kubernetes/system-registry/mirrorer/config.yaml"

	registryStaticPodConfigPath = "/etc/kubernetes/manifests/system-registry.yaml"
)

type servicesManager struct {
	m        sync.Mutex
	log      *slog.Logger
	settings AppSettings
}

func (manager *servicesManager) applyConfig(config NodeServicesConfigModel) (changes changesModel, err error) {
	// Lock to prevent concurrent config changes
	manager.m.Lock()
	defer manager.m.Unlock()

	sum := sha256.New()
	var hash string

	// Sync the PKI files
	if changes.PKI, hash, err = syncPKIFiles(
		pkiConfigDirectoryPath,
		config.Config.PKI,
	); err != nil {
		err = fmt.Errorf("error saving PKI files: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	// Process the templates with the given data and create the static pod and configuration files
	if changes.Auth, hash, err = processTemplate(
		config.toAuthConfig(),
		authConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Auth template: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	if changes.Distribution, hash, err = processTemplate(
		config.toDistributionConfig(manager.settings.HostIP),
		distributionConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Distribution template: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	mirrorer := config.toMirrorerConfig(manager.settings.RegistryAddress)
	hasMirrorer := mirrorer != nil && len(mirrorer.Upstreams) > 0

	if hasMirrorer {
		if changes.Mirrorer, hash, err = processTemplate(
			mirrorer,
			mirrorerConfigPath,
		); err != nil {
			err = fmt.Errorf("error processing Mirrorer template: %w", err)
			return
		} else {
			sum.Write([]byte(hash))
		}
	} else {
		// Delete the mirrorer config file
		if changes.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
			err = fmt.Errorf("error deleting Mirrorer config file: %w", err)
			return
		}
	}

	hashBytes := sum.Sum([]byte{})
	hash = hex.EncodeToString(hashBytes)

	images := staticPodImagesModel{
		Auth:         manager.settings.ImageAuth,
		Distribution: manager.settings.ImageDistribution,
		Mirrorer:     manager.settings.ImageMirrorer,
	}

	if changes.Pod, _, err = processTemplate(
		config.toStaticPodConfig(images, hash, hasMirrorer),
		registryStaticPodConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing static pod template: %w", err)
		return
	}

	return
}

func (manager *servicesManager) StopServices() (changes changesModel, err error) {
	// Lock to prevent concurrent config changes
	manager.m.Lock()
	defer manager.m.Unlock()

	// Delete the static pod file
	if changes.Pod, err = deleteFile(registryStaticPodConfigPath); err != nil {
		err = fmt.Errorf("error deleting static pod file: %w", err)
		return
	}

	// Delete the auth config file
	if changes.Auth, err = deleteFile(authConfigPath); err != nil {
		err = fmt.Errorf("error deleting Auth config file: %w", err)
		return
	}

	// Delete the distribution config file
	if changes.Distribution, err = deleteFile(distributionConfigPath); err != nil {
		err = fmt.Errorf("error deleting Distribution config file: %w", err)
		return
	}

	// Delete the mirrorer config file
	if changes.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
		err = fmt.Errorf("error deleting Mirrorer config file: %w", err)
		return
	}

	if changes.PKI, err = deleteDirectory(pkiConfigDirectoryPath); err != nil {
		err = fmt.Errorf("error deleting registry PKI directory: %w", err)
		return
	}

	return
}
