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

package main

import (
	"errors"
	"time"
)

const (
	defaultTempDir        = "/opt/deckhouse/tmp"
	defaultInstalledStore = "/var/cache/registrypackages"
)

const (
	defaultKubeconfigPath     = "/etc/kubernetes/kubelet.conf"
	defaultBootstrapTokenPath = "/var/lib/bashible/bootstrap-token"
	defaultBootstrapCAPath    = "/var/lib/bashible/ca.crt"
)

const (
	modeFetch     = "fetch"
	modeInstall   = "install"
	modeUninstall = "uninstall"
)

const (
	rppConnectTimeout        = 10 * time.Second
	rppResponseHeaderTimeout = 60 * time.Second
)

const (
	kubeRequestTimeout = 10 * time.Second

	defaultRPPNamespace       = "d8-cloud-instance-manager"
	defaultRPPTokenSecretName = "registry-packages-proxy-token"
	defaultRPPLabelSelector   = "app=registry-packages-proxy"
	defaultRPPPort            = 4219
)

const (
	kubeRetries    = 30
	kubeRetryDelay = 5 * time.Second
)

const (
	defaultRetries         = 30
	defaultRetryDelay      = 5 * time.Second
	defaultInstallWorkers  = 4
	packageInstallAttempts = 5
)

const (
	scriptExecTimeout     = 10 * time.Minute
	archiveExtractTimeout = 5 * time.Minute
)

var (
	packageScripts                   = []string{"install", "uninstall"}
	errInvalidDigest                 = errors.New("digest must be <algorithm>:<value>, both parts must contain only lowercase letters and digits")
	errNoEndpoints                   = errors.New("no RPP endpoints configured")
	errNoToken                       = errors.New("no RPP token configured")
	errNoKubeAPIConfig               = errors.New("can't configure kube-api client: no kubelet.conf or bootstrap-token found")
	errNoBootstrapAPIServerEndpoints = errors.New("bootstrap-token mode requires kube-apiserver endpoints")
	errEmptyBootstrapToken           = errors.New("bootstrap-token file is empty")
)
