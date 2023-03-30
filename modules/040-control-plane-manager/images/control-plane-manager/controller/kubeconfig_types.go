/*
Copyright 2023 Flant JSC

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

package main

// Copied from https://github.com/giantswarm/kubeconfig/blob/master/types.go
// KubeConfigValue is a struct used to create a kubectl configuration YAML file.
type KubeConfigValue struct {
	APIVersion     string                   `yaml:"apiVersion"`
	Kind           string                   `yaml:"kind"`
	Clusters       []KubeconfigNamedCluster `yaml:"clusters"`
	Users          []KubeconfigUser         `yaml:"users"`
	Contexts       []KubeconfigNamedContext `yaml:"contexts"`
	CurrentContext string                   `yaml:"current-context"`
	Preferences    struct{}                 `yaml:"preferences"`
}

// KubeconfigUser is a struct used to create a kubectl configuration YAML file
type KubeconfigUser struct {
	Name string                `yaml:"name"`
	User KubeconfigUserKeyPair `yaml:"user"`
}

// KubeconfigUserKeyPair is a struct used to create a kubectl configuration YAML file
type KubeconfigUserKeyPair struct {
	ClientCertificateData string                 `yaml:"client-certificate-data"`
	ClientKeyData         string                 `yaml:"client-key-data"`
	AuthProvider          KubeconfigAuthProvider `yaml:"auth-provider,omitempty"`
}

// KubeconfigAuthProvider is a struct used to create a kubectl authentication provider
type KubeconfigAuthProvider struct {
	Name   string            `yaml:"name"`
	Config map[string]string `yaml:"config"`
}

// KubeconfigNamedCluster is a struct used to create a kubectl configuration YAML file
type KubeconfigNamedCluster struct {
	Name    string            `yaml:"name"`
	Cluster KubeconfigCluster `yaml:"cluster"`
}

// KubeconfigCluster is a struct used to create a kubectl configuration YAML file
type KubeconfigCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	CertificateAuthority     string `yaml:"certificate-authority"`
}

// KubeconfigNamedContext is a struct used to create a kubectl configuration YAML file
type KubeconfigNamedContext struct {
	Name    string            `yaml:"name"`
	Context KubeconfigContext `yaml:"context"`
}

// KubeconfigContext is a struct used to create a kubectl configuration YAML file
type KubeconfigContext struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace,omitempty"`
	User      string `yaml:"user"`
}
