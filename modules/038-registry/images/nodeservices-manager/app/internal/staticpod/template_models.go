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
	nodeservices "github.com/deckhouse/deckhouse/go_lib/registry/models/node-services"
)

type authConfigModel struct {
	RO           authConfigUserModel
	RW           *authConfigUserModel
	MirrorPuller *authConfigUserModel
	MirrorPusher *authConfigUserModel
}

func (model authConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/auth/config.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toAuthConfig() authConfigModel {
	config := value.Config
	mapUser := func(user nodeservices.User) authConfigUserModel {
		return authConfigUserModel{
			Name:         user.Name,
			PasswordHash: user.PasswordHash,
		}
	}

	model := authConfigModel{
		RO: mapUser(config.UserRO),
	}

	if config.LocalMode != nil {
		rw := mapUser(config.LocalMode.UserRW)
		puller := mapUser(config.LocalMode.UserPuller)
		pusher := mapUser(config.LocalMode.UserPusher)

		model.RW = &rw
		model.MirrorPuller = &puller
		model.MirrorPusher = &pusher
	}

	return model
}

type authConfigUserModel struct {
	Name         string
	PasswordHash string
}

type distributionConfigModel struct {
	ListenAddress string
	HTTPSecret    string
	Ingress       bool
	Upstream      *distributionConfigUpstreamModel
}

type distributionConfigUpstreamModel struct {
	Scheme   string
	Host     string
	Path     string
	User     string
	Password string
	TTL      *string
	CA       bool
}

func (model distributionConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/distribution/config.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toDistributionConfig(listenAddress string) distributionConfigModel {
	config := value.Config

	model := distributionConfigModel{
		ListenAddress: listenAddress,
		HTTPSecret:    config.HTTPSecret,
	}

	if config.LocalMode != nil {
		model.Ingress = config.LocalMode.IngressClientCACert != ""
	} else if config.ProxyMode != nil {
		upstream := config.ProxyMode.Upstream

		model.Upstream = &distributionConfigUpstreamModel{
			Scheme:   upstream.Scheme,
			Host:     upstream.Host,
			Path:     upstream.Path,
			User:     upstream.User,
			Password: upstream.Password,
			TTL:      upstream.TTL,
			CA:       config.ProxyMode.UpstreamRegistryCACert != "",
		}
	}

	return model
}

type mirrorerConfigModel struct {
	UserPuller   mirrorerConfigUserModel
	UserPusher   mirrorerConfigUserModel
	LocalAddress string
	Upstreams    []string
}

type mirrorerConfigUserModel struct {
	Name     string
	Password string
}

func (model mirrorerConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/mirrorer/config.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toMirrorerConfig(localAddress string) *mirrorerConfigModel {
	if value.Config.LocalMode == nil {
		return nil
	}

	config := value.Config.LocalMode
	model := mirrorerConfigModel{
		LocalAddress: localAddress,
		UserPuller: mirrorerConfigUserModel{
			Name:     config.UserPuller.Name,
			Password: config.UserPuller.Password,
		},
		UserPusher: mirrorerConfigUserModel{
			Name:     config.UserPusher.Name,
			Password: config.UserPusher.Password,
		},
	}

	if len(config.Upstreams) > 0 {
		model.Upstreams = make([]string, len(config.Upstreams))
		copy(model.Upstreams, config.Upstreams)
	}

	return &model
}

type staticPodConfigModel struct {
	Hash        string
	Version     string
	Images      staticPodImagesModel
	HasMirrorer bool
	ProxyEnvs   *staticPodProxyEnvsModel
}

type staticPodProxyEnvsModel struct {
	HTTP    string
	HTTPS   string
	NoProxy string
}

func (m *staticPodProxyEnvsModel) hasAny() bool {
	if m.HTTP != "" {
		return true
	}

	if m.HTTPS != "" {
		return true
	}

	if m.NoProxy != "" {
		return true
	}

	return false
}

type staticPodImagesModel struct {
	Distribution string
	Auth         string
	Mirrorer     string
}

func (model staticPodConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/static_pods/registry-nodeservices.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toStaticPodConfig(images staticPodImagesModel, proxyEnvs staticPodProxyEnvsModel, hash string, hasMirrorer bool) staticPodConfigModel {
	model := staticPodConfigModel{
		Hash:        hash,
		Version:     value.Version,
		Images:      images,
		HasMirrorer: hasMirrorer,
	}

	// proxyEnvs only for proxy mode
	if value.Config.ProxyMode != nil {
		if proxyEnvs.hasAny() {
			model.ProxyEnvs = &proxyEnvs
		}
	}
	return model
}
