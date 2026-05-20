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

package rpp

import (
	"errors"
	"time"
)

const (
	connectTimeout        = 10 * time.Second
	responseHeaderTimeout = 60 * time.Second

	defaultInstallWorkers  = 4
	packageInstallAttempts = 10

	scriptExecTimeout     = 10 * time.Minute
	archiveExtractTimeout = 5 * time.Minute
)

const (
	resultInstalled = "installed"
	resultSkipped   = "skipped"
	resultRemoved   = "removed"
)

var (
	packageScripts = []string{"install", "uninstall"}

	errInvalidDigest = errors.New("digest must be <algorithm>:<value>, both parts must contain only lowercase letters and digits")
	errNoEndpoints   = errors.New("no RPP endpoints configured")
	errNoToken       = errors.New("no RPP token configured")
)
