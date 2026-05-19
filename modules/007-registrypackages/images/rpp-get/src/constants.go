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
	modeFetch     = "fetch"
	modeInstall   = "install"
	modeUninstall = "uninstall"
)

const (
	kubeRetries    = 30
	kubeRetryDelay = 5 * time.Second
)

const (
	defaultRetries    = 30
	defaultRetryDelay = 5 * time.Second
)

var errNoBootstrapAPIServerEndpoints = errors.New("bootstrap-token mode requires kube-apiserver endpoints")
