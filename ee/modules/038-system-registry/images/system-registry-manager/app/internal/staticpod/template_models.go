/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/node-services"
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
	mapUser := func(user nodeservices.User) authConfigUserModel {
		return authConfigUserModel{
			Name:         user.Name,
			PasswordHash: user.PasswordHash,
		}
	}

	config := value.Config

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
	Proxy       *staticPodProxyModel
}

type staticPodProxyModel struct {
	HTTP    string
	HTTPS   string
	NoProxy string
}

type staticPodImagesModel struct {
	Distribution string
	Auth         string
	Mirrorer     string
}

func (model staticPodConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/static_pods/system-registry.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toStaticPodConfig(images staticPodImagesModel, hash string, hasMirrorer bool) staticPodConfigModel {
	config := value.Config

	model := staticPodConfigModel{
		Hash:        hash,
		Version:     value.Version,
		Images:      images,
		HasMirrorer: hasMirrorer,
	}

	proxy := config.ProxyConfig
	if proxy != nil {
		model.Proxy = &staticPodProxyModel{
			HTTP:    proxy.HTTP,
			HTTPS:   proxy.HTTPS,
			NoProxy: proxy.NoProxy,
		}
	}

	return model
}
