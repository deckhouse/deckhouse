// Copyright 2023 Flant JSC
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

package utils

import (
	"errors"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

const (
	SyncedPollPeriod = 100 * time.Millisecond
)

// GenerateRegistryOptionsFromModuleSource fetches settings from ModuleSource and generate registry options from them
func GenerateRegistryOptionsFromModuleSource(ms *v1alpha1.ModuleSource, clusterUUID string) []cr.Option {
	rconf := &RegistryConfig{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    clusterUUID,
	}

	return GenerateRegistryOptions(rconf)
}

type RegistryConfig struct {
	DockerConfig string
	CA           string
	Scheme       string
	UserAgent    string
}

func GenerateRegistryOptions(ri *RegistryConfig) []cr.Option {
	if ri.UserAgent == "" {
		if log.IsLevelEnabled(log.DebugLevel) {
			loggerCopy := *log.StandardLogger()
			loggerCopy.ReportCaller = true
			loggerCopy.Debugln("got empty user agent")
		}

		ri.UserAgent = "deckhouse-controller"
	}

	opts := []cr.Option{
		cr.WithAuth(ri.DockerConfig),
		cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(ri.CA),
		cr.WithInsecureSchema(strings.ToLower(ri.Scheme) == "http"),
	}

	return opts
}

type DeckhouseRegistrySecret struct {
	DockerConfig          string
	Address               string
	ClusterIsBootstrapped string
	ImageRegistry         string
	Path                  string
	Scheme                string
	CA                    string
}

var ErrCAFieldIsNotFound = errors.New("secret has no ca field")

func ParseDeckhouseRegistrySecret(data map[string][]byte) (*DeckhouseRegistrySecret, error) {
	var err error

	dockerConfig, ok := data[".dockerconfigjson"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no .dockerconfigjson field"))
	}

	address, ok := data["address"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no address field"))
	}

	clusterIsBootstrapped, ok := data["clusterIsBootstrapped"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no clusterIsBootstrapped field"))
	}

	imagesRegistry, ok := data["imagesRegistry"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no imagesRegistry field"))
	}

	path, ok := data["path"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no path field"))
	}

	scheme, ok := data["scheme"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no scheme field"))
	}

	ca, ok := data["ca"]
	if !ok {
		err = errors.Join(err, ErrCAFieldIsNotFound)
	}

	return &DeckhouseRegistrySecret{
		DockerConfig:          string(dockerConfig),
		Address:               string(address),
		ClusterIsBootstrapped: string(clusterIsBootstrapped),
		ImageRegistry:         string(imagesRegistry),
		Path:                  string(path),
		Scheme:                string(scheme),
		CA:                    string(ca),
	}, err
}
