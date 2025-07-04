/*
Copyright 2025 Flant JSC

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

package staticpod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
)

const (
	authConfigPath         = "/etc/kubernetes/registry/auth/config.yaml"
	distributionConfigPath = "/etc/kubernetes/registry/distribution/config.yaml"
	pkiConfigDirectoryPath = "/etc/kubernetes/registry/pki"
	mirrorerConfigPath     = "/etc/kubernetes/registry/mirrorer/config.yaml"

	registryStaticPodConfigPath = "/etc/kubernetes/manifests/registry.yaml"
)

type servicesManager struct {
	m        sync.Mutex
	log      *slog.Logger
	settings AppSettings
}

func (manager *servicesManager) applyConfig(config NodeServicesConfigModel) (changesModel, error) {
	var (
		changes changesModel
		err     error
	)

	// Lock to prevent concurrent config changes
	manager.m.Lock()
	defer manager.m.Unlock()

	sum := sha256.New()
	var hash string

	// Sync the PKI files
	if changes.PKI, hash, err = syncPKIFiles(
		pkiConfigDirectoryPath,
		config.Config,
	); err != nil {
		err = fmt.Errorf("error saving PKI files: %w", err)
		return changes, err
	}
	sum.Write([]byte(hash))

	// Process the templates with the given data and create the static pod and configuration files
	if changes.Auth, hash, err = processTemplate(
		config.toAuthConfig(),
		authConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Auth template: %w", err)
		return changes, err
	}
	sum.Write([]byte(hash))

	if changes.Distribution, hash, err = processTemplate(
		config.toDistributionConfig(manager.settings.HostIP),
		distributionConfigPath,
	); err != nil {
		err = fmt.Errorf("error processing Distribution template: %w", err)
		return changes, err
	}
	sum.Write([]byte(hash))

	mirrorer := config.toMirrorerConfig(manager.settings.HostIP)
	hasMirrorer := mirrorer != nil && len(mirrorer.Upstreams) > 0

	if hasMirrorer {
		if changes.Mirrorer, hash, err = processTemplate(
			mirrorer,
			mirrorerConfigPath,
		); err != nil {
			err = fmt.Errorf("error processing Mirrorer template: %w", err)
			return changes, err
		}
		sum.Write([]byte(hash))
	} else {
		// Delete the mirrorer config file
		if changes.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
			err = fmt.Errorf("error deleting Mirrorer config file: %w", err)
			return changes, err
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
		return changes, err
	}

	return changes, err
}

func (manager *servicesManager) StopServices() (changesModel, error) {
	var (
		changes changesModel
		err     error
	)

	// Lock to prevent concurrent config changes
	manager.m.Lock()
	defer manager.m.Unlock()

	// Delete the static pod file
	if changes.Pod, err = deleteFile(registryStaticPodConfigPath); err != nil {
		err = fmt.Errorf("error deleting static pod file: %w", err)
		return changes, err
	}

	// Delete the auth config file
	if changes.Auth, err = deleteFile(authConfigPath); err != nil {
		err = fmt.Errorf("error deleting Auth config file: %w", err)
		return changes, err
	}

	// Delete the distribution config file
	if changes.Distribution, err = deleteFile(distributionConfigPath); err != nil {
		err = fmt.Errorf("error deleting Distribution config file: %w", err)
		return changes, err
	}

	// Delete the mirrorer config file
	if changes.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
		err = fmt.Errorf("error deleting Mirrorer config file: %w", err)
		return changes, err
	}

	if changes.PKI, err = deleteDirectory(pkiConfigDirectoryPath); err != nil {
		err = fmt.Errorf("error deleting registry PKI directory: %w", err)
		return changes, err
	}

	return changes, err
}
