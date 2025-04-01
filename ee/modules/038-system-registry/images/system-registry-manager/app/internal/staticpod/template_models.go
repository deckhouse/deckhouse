/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/node-services"
)

type authConfigModel struct {
	RO           authConfigUserModel
	RW           authConfigUserModel
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

	registry := value.Config.Registry

	model := authConfigModel{
		RO: mapUser(registry.UserRO),
		RW: mapUser(registry.UserRW),
	}

	mirrorer := registry.Mirrorer
	if mirrorer != nil {
		puller := mapUser(mirrorer.UserPuller)
		pusher := mapUser(mirrorer.UserPusher)

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
	HttpSecret    string
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
}

func (model distributionConfigModel) Render() ([]byte, error) {
	return renderTemplate("templates/distribution/config.yaml.tpl", model)
}

func (value NodeServicesConfigModel) toDistributionConfig(listenAddress string) distributionConfigModel {
	config := value.Config
	registry := config.Registry

	model := distributionConfigModel{
		ListenAddress: listenAddress,
		HttpSecret:    registry.HttpSecret,
		Ingress:       config.PKI.IngressClientCACert != "",
	}

	upstream := registry.Upstream
	if upstream != nil {
		model.Upstream = &distributionConfigUpstreamModel{
			Scheme:   upstream.Scheme,
			Host:     upstream.Host,
			Path:     upstream.Path,
			User:     upstream.User,
			Password: upstream.Password,
			TTL:      upstream.TTL,
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
	mirrorer := value.Config.Registry.Mirrorer

	if mirrorer == nil {
		return nil
	}

	model := mirrorerConfigModel{
		LocalAddress: localAddress,
		UserPuller: mirrorerConfigUserModel{
			Name:     mirrorer.UserPuller.Name,
			Password: mirrorer.UserPuller.Password,
		},
		UserPusher: mirrorerConfigUserModel{
			Name:     mirrorer.UserPusher.Name,
			Password: mirrorer.UserPusher.Password,
		},
	}

	if len(mirrorer.Upstreams) > 0 {
		model.Upstreams = make([]string, len(mirrorer.Upstreams))
		copy(model.Upstreams, mirrorer.Upstreams)
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
	Http    string
	Https   string
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

	proxy := config.Proxy
	if proxy != nil {
		model.Proxy = &staticPodProxyModel{
			Http:    proxy.Http,
			Https:   proxy.Https,
			NoProxy: proxy.Https,
		}
	}

	return model
}
