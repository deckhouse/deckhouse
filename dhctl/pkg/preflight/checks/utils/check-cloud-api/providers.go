// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkcloudapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

// EndpointFor returns the API endpoint of a provider, given its `provider:` section of the
// <Provider>ClusterConfiguration.
//
// Two of the eleven providers used to be here. For the other nine the check returned no config
// and the runner printed a ✓ — a green line for a question never asked, on exactly the failure
// (no egress from the node to the cloud API) that the check exists to catch and that otherwise
// surfaces as cloud-controller-manager and CSI quietly not working once the cluster is up.
//
// A provider with no entry returns (nil, nil), which the check reports as not applicable rather
// than as a pass.
func EndpointFor(provider string, providerClusterConfig []byte) (*CloudAPIConfig, error) {
	handler, ok := endpointHandlers[provider]
	if !ok {
		return nil, nil
	}
	return handler(providerClusterConfig)
}

// KnownProviders reports whether an endpoint is known for the named provider.
func KnownProviders(provider string) bool {
	_, ok := endpointHandlers[provider]
	return ok
}

var endpointHandlers = map[string]func([]byte) (*CloudAPIConfig, error){
	// The endpoint is a field of the provider configuration.
	"openstack":   HandleOpenStackProvider,
	"vsphere":     HandleVSphereProvider,
	"vcd":         handleFieldProvider("server", "VCDClusterConfiguration.provider.server"),
	"zvirt":       handleFieldProvider("server", "ZvirtClusterConfiguration.provider.server"),
	"dynamix":     handleFieldProvider("controllerUrl", "DynamixClusterConfiguration.provider.controllerUrl"),
	"huaweicloud": handleFieldProvider("authURL", "HuaweiCloudClusterConfiguration.provider.authURL"),

	// The endpoint is fixed, or derived from the region.
	"aws":    handleAWSProvider,
	"gcp":    handleFixedProvider("https://compute.googleapis.com"),
	"azure":  handleFixedProvider("https://management.azure.com"),
	"yandex": handleFixedProvider("https://api.cloud.yandex.net"),

	// The endpoint is inside a kubeconfig.
	"dvp": handleDVPProvider,
}

func HandleOpenStackProvider(providerClusterConfig []byte) (*CloudAPIConfig, error) {
	var openStackConfig OpenStackProvider
	if err := json.Unmarshal(providerClusterConfig, &openStackConfig); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for OpenStack: %w", err)
	}

	endpoint, err := urlParse(openStackConfig.AuthURL)
	if err != nil {
		return nil, fmt.Errorf("OpenStackClusterConfiguration.provider.authURL: %w", err)
	}

	return &CloudAPIConfig{
		URL:      endpoint,
		CACert:   openStackConfig.CACert,
		Insecure: false,
		Field:    "OpenStackClusterConfiguration.provider.authURL",
	}, nil
}

func HandleVSphereProvider(providerClusterConfig []byte) (*CloudAPIConfig, error) {
	var vsphereConfig VSphereProvider
	if err := json.Unmarshal(providerClusterConfig, &vsphereConfig); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for vSphere: %w", err)
	}

	endpoint, err := urlParse(vsphereConfig.Server)
	if err != nil {
		return nil, fmt.Errorf("VsphereClusterConfiguration.provider.server: %w", err)
	}

	// caBundle is base64 in the schema, and was ignored entirely: a vSphere behind a private CA
	// failed verification here while the cluster itself, which is given the bundle, would not.
	caCert, err := decodeCABundle(vsphereConfig.CABundle)
	if err != nil {
		return nil, fmt.Errorf("VsphereClusterConfiguration.provider.caBundle: %w", err)
	}

	return &CloudAPIConfig{
		URL:      endpoint,
		CACert:   caCert,
		Insecure: vsphereConfig.Insecure,
		Field:    "VsphereClusterConfiguration.provider.server",
	}, nil
}

// handleFieldProvider reads the endpoint out of one string field of the provider configuration.
func handleFieldProvider(field, fieldPath string) func([]byte) (*CloudAPIConfig, error) {
	return func(providerClusterConfig []byte) (*CloudAPIConfig, error) {
		var raw map[string]any
		if err := json.Unmarshal(providerClusterConfig, &raw); err != nil {
			return nil, fmt.Errorf("unable to unmarshal provider config: %w", err)
		}

		value, _ := raw[field].(string)
		endpoint, err := urlParse(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fieldPath, err)
		}

		insecure, _ := raw["insecure"].(bool)
		caCert, err := decodeCABundle(stringField(raw, "caBundle"))
		if err != nil {
			return nil, fmt.Errorf("provider.caBundle: %w", err)
		}

		return &CloudAPIConfig{URL: endpoint, Insecure: insecure, CACert: caCert, Field: fieldPath}, nil
	}
}

// handleFixedProvider is for the public clouds whose API endpoint is the same for everyone. The
// check is still worth running: what is being asked is whether the node has egress to it.
func handleFixedProvider(endpoint string) func([]byte) (*CloudAPIConfig, error) {
	return func([]byte) (*CloudAPIConfig, error) {
		parsed, err := urlParse(endpoint)
		if err != nil {
			return nil, err
		}
		return &CloudAPIConfig{URL: parsed, Field: "the provider's public API endpoint"}, nil
	}
}

func handleAWSProvider(providerClusterConfig []byte) (*CloudAPIConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal(providerClusterConfig, &raw); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for AWS: %w", err)
	}

	region := stringField(raw, "region")
	if region == "" {
		return nil, fmt.Errorf("AWSClusterConfiguration.provider.region is empty")
	}

	endpoint, err := urlParse(fmt.Sprintf("https://ec2.%s.amazonaws.com", region))
	if err != nil {
		return nil, err
	}
	return &CloudAPIConfig{URL: endpoint, Field: "AWSClusterConfiguration.provider.region"}, nil
}

// handleDVPProvider reads the API server out of the kubeconfig DVP is configured with. The
// kubeconfig is not otherwise validated here: a malformed one is a configuration error, and
// saying so is better than reporting the cloud API as unreachable.
func handleDVPProvider(providerClusterConfig []byte) (*CloudAPIConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal(providerClusterConfig, &raw); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for DVP: %w", err)
	}

	encoded := stringField(raw, "kubeconfigDataBase64")
	if encoded == "" {
		return nil, fmt.Errorf("DVPClusterConfiguration.provider.kubeconfigDataBase64 is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("DVPClusterConfiguration.provider.kubeconfigDataBase64 is not valid base64: %w", err)
	}

	var kubeconfig dvpKubeconfig
	if err := yaml.Unmarshal(decoded, &kubeconfig); err != nil {
		// Deliberately without %w: the unmarshal error is a dump of the Go type it failed to
		// build, which tells the reader nothing about their file.
		return nil, fmt.Errorf(
			"the value in DVPClusterConfiguration.provider.kubeconfigDataBase64 decodes, but is not a kubeconfig")
	}
	if len(kubeconfig.Clusters) == 0 {
		return nil, fmt.Errorf("the kubeconfig in DVPClusterConfiguration.provider.kubeconfigDataBase64 names no cluster")
	}

	cluster := kubeconfig.Clusters[0].Cluster
	endpoint, err := urlParse(cluster.Server)
	if err != nil {
		return nil, fmt.Errorf("the kubeconfig in DVPClusterConfiguration.provider.kubeconfigDataBase64: %w", err)
	}

	caCert, err := decodeCABundle(cluster.CertificateAuthorityData)
	if err != nil {
		return nil, fmt.Errorf("certificate-authority-data in the DVP kubeconfig: %w", err)
	}

	return &CloudAPIConfig{
		URL:      endpoint,
		CACert:   caCert,
		Insecure: cluster.InsecureSkipTLSVerify,
		Field:    "DVPClusterConfiguration.provider.kubeconfigDataBase64",
	}, nil
}

// The part of a kubeconfig this needs. Named rather than anonymous so that when yaml fails to
// build it, the error names a type instead of printing its whole shape.
type dvpKubeconfig struct {
	Clusters []dvpKubeconfigCluster `json:"clusters"`
}

type dvpKubeconfigCluster struct {
	Cluster dvpClusterEntry `json:"cluster"`
}

type dvpClusterEntry struct {
	Server                   string `json:"server"`
	CertificateAuthorityData string `json:"certificate-authority-data"`
	InsecureSkipTLSVerify    bool   `json:"insecure-skip-tls-verify"`
}

func stringField(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

// decodeCABundle accepts the bundle either base64-encoded, as the schemas describe it, or as raw
// PEM, which is what operators paste often enough to be worth accepting.
func decodeCABundle(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 10 && value[:10] == "-----BEGIN" {
		return value, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("expected base64-encoded PEM: %w", err)
	}
	return string(decoded), nil
}
