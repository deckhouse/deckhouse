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

func (manager *servicesManager) applyConfig(config NodeServicesConfigModel) (changes ChangesModel, err error) {
	// Lock to prevent concurrent config changes
	manager.m.Lock()
	defer manager.m.Unlock()

	model := templateModel{
		Config:  config.Config,
		Version: config.Version,
		Address: manager.settings.HostIP,
		Images: Images{
			Distribution: manager.settings.ImageDistribution,
			Auth:         manager.settings.ImageAuth,
			Mirrorer:     manager.settings.ImageMirrorer,
		},
	}

	sum := sha256.New()
	var hash string

	// Sync the PKI files
	if changes.PKI, hash, err = model.PKI.syncPKIFiles(
		pkiConfigDirectoryPath,
	); err != nil {
		err = fmt.Errorf("error saving PKI files: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	// Process the templates with the given data and create the static pod and configuration files
	if changes.Auth, hash, err = model.processTemplate(
		authConfigTemplateName,
		authConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Auth template: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	if changes.Distribution, hash, err = model.processTemplate(
		distributionConfigTemplateName,
		distributionConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Distribution template: %w", err)
		return
	} else {
		sum.Write([]byte(hash))
	}

	if model.Registry.Mirrorer != nil {
		if changes.Mirrorer, hash, err = model.processTemplate(
			mirrorerConfigTemplateName,
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
	model.Hash = hex.EncodeToString(hashBytes)
	if changes.Pod, _, err = model.processTemplate(
		registryStaticPodTemplateName,
		registryStaticPodConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing static pod template: %w", err)
		return
	}

	return
}

func (manager *servicesManager) StopServices() (changes ChangesModel, err error) {
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
