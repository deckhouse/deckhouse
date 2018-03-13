package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"
)

type DockerImageInfo struct {
	Registry   string
	Repository string
	Tag        string
}

func DockerRegistryGetImageId(imageInfo DockerImageInfo, dockerRegistry *registryclient.Registry) (string, error) {
	// Получить описание образа
	antiopaManifest, err := dockerRegistry.ManifestV2(imageInfo.Repository, imageInfo.Tag)
	if err != nil {
		rlog.Errorf("REGISTRY cannot get manifest for %s:%s: %v", imageInfo.Repository, imageInfo.Tag, err)
		return "", err
	}

	imageID := antiopaManifest.Config.Digest.String()
	rlog.Debugf("REGISTRY id=%s for %s:%s", imageID, imageInfo.Repository, imageInfo.Tag)

	return imageID, nil
}

func DockerParseImageName(imageName string) (imageInfo DockerImageInfo, err error) {
	namedRef, err := reference.ParseNormalizedNamed(imageName)
	switch {
	case err != nil:
		return
	case reference.IsNameOnly(namedRef):
		// Если имя без тэга, то docker добавляет latest
		namedRef = reference.TagNameOnly(namedRef)
	}

	tag := ""
	if tagged, ok := namedRef.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	imageInfo = DockerImageInfo{
		Registry:   reference.Domain(namedRef),
		Repository: reference.Path(namedRef),
		Tag:        tag,
	}

	rlog.Debugf("REGISTRY image %s parsed to reg=%s repo=%s tag=%s", imageName, imageInfo.Registry, imageInfo.Repository, imageInfo.Tag)

	return
}

func RegistryClientLogCallback(format string, args ...interface{}) {
	rlog.Debugf(format, args...)
}

// NewDockerRegistry - ручной конструктор клиента, как рекомендовано в комментариях
// к registryclient.New.
// Этот конструктор не запускает registry.Ping и логирует события через rlog.
func NewDockerRegistry(registryUrl, username, password string) *registryclient.Registry {
	url := strings.TrimSuffix(registryUrl, "/")

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSNextProto:          make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}

	wrappedTransport := registryclient.WrapTransport(transport, url, username, password)

	return &registryclient.Registry{
		URL: url,
		Client: &http.Client{
			Transport: wrappedTransport,
		},
		Logf: RegistryClientLogCallback,
	}
}
