// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vcd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/version"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/vmware/go-vcloud-director/v3/govcd"
)

func VersionContentProvider(ctx context.Context, settings settings.ProviderSettings, metaConfig *config.MetaConfig, logger log.Logger) ([]byte, string, error) {
	client, err := newVcdCloudClient(metaConfig, logger)
	if err != nil {
		return nil, "", err
	}
	return versionContentProviderWithClient(ctx, client, settings, logger)
}

type cloudClient interface {
	GetVersion(ctx context.Context) (string, error)
}

type vcdCloudClient struct {
	client *govcd.VCDClient
}

func newVcdCloudClient(m *config.MetaConfig, _ log.Logger) (cloudClient, error) {
	if m.ClusterType != config.CloudClusterType || len(m.ProviderClusterConfig) == 0 {
		return nil, fmt.Errorf("current cluster type is not a cloud type")
	}

	if m.ProviderName != ProviderName {
		return nil, fmt.Errorf("current provider type is not VCD")
	}

	var providerConfiguration providerConfig
	if err := json.Unmarshal(m.ProviderClusterConfig["provider"], &providerConfiguration); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider configuration: %v", err)
	}

	vcdUrl, err := url.ParseRequestURI(fmt.Sprintf("%s/api", providerConfiguration.Server))
	if err != nil {
		return nil, fmt.Errorf("unable to parse VCD provider url: %v", err)
	}
	insecure := providerConfiguration.Insecure

	vcdClient := govcd.NewVCDClient(
		*vcdUrl,
		insecure,
	)

	vcdClient.Client.APIVCDMaxVersionIs("")
	vcdClient.Client.MaxRetryTimeout = 10 // seconds

	return &vcdCloudClient{client: vcdClient}, nil
}

func (v *vcdCloudClient) GetVersion(context.Context) (string, error) {
	apiVersion, err := v.client.Client.MaxSupportedVersion()
	if err != nil {
		return "", fmt.Errorf("unable to get VCD API version: %v", err)
	}

	return apiVersion, nil
}

func versionConstraintAction(apiVersion string, logger log.Logger, action func(legacy bool) error) error {
	ver, err := semver.NewVersion(apiVersion)
	if err != nil {
		return fmt.Errorf("failed to parse VCD API version '%s': %v", apiVersion, err)
	}

	logger.LogDebugF("VCD API version '%s'\n", apiVersion)

	const versionConstraintStr = "<37.2"

	versionConstraint, err := semver.NewConstraint(versionConstraintStr)
	if err != nil {
		return fmt.Errorf("failed to parse version constraint '%s': %v", versionConstraint, err)
	}

	if versionConstraint.Check(ver) {
		logger.LogDebugF("Use legacy VCD version %s (%s). Use legacy mode as true\n", ver, versionConstraintStr)
		return action(true)
	}

	logger.LogDebugF("Use latest VCD version %s (%s)e\n", ver, versionConstraintStr)
	return action(false)
}

func versionContentProviderWithClient(ctx context.Context, client cloudClient, settings settings.ProviderSettings, logger log.Logger) ([]byte, string, error) {
	apiVersion, err := client.GetVersion(ctx)
	if err != nil {
		return nil, "", err
	}

	var content []byte
	var resultVersion string

	err = versionConstraintAction(apiVersion, logger, func(legacy bool) error {
		versions := settings.Versions()
		if len(versions) != 2 {
			return fmt.Errorf("expected 2 versions, got %d", len(versions))
		}

		ver := legacyVersion
		if !legacy {
			for _, v := range versions {
				if v != legacyVersion {
					ver = v
				}
			}
		}

		resultVersion = ver
		content = version.GetVersionContent(settings, ver)

		return nil
	})

	if err != nil {
		return nil, "", err
	}

	return content, resultVersion, nil
}
