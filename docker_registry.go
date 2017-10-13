package main

import (
	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/docker/distribution/reference"
	"github.com/romana/rlog"
)

// TODO данные для доступа к registry серверам нужно хранить в secret-ах.
// TODO по imageInfo.Registry брать данные и подключаться к нужному registry.
// Пока известно, что будет только registry.flant.com

var DockerRegistryInfo = map[string]map[string]string{
	"registry.flant.com": map[string]string{
		"url": "https://registry.flant.com",
		"user": "oauth2",
		"password": "qweqwe",
	},
	// minikube specific
	"localhost:5000": map[string]string{
		"url": "http://kube-registry.kube-system.svc.cluster.local:5000",
	},
}


//const DockerRegistryUrl = "https://registry.flant.com"
//const DockerRegistryUser = "oauth2"
//const DockerRegistryToken = ""

type DockerImageInfo struct {
	Registry string
	Repository string
	Tag string
}

func DockerRegistryGetImageId(image string) (string, error) {
	imageInfo, err := DockerParseImageName(image)
	if err != nil {
		rlog.Errorf("REGISTRY Problem parsing image %s: %v", image, err)
		return "", err
	}

	url := ""
	user := ""
	password := ""
	if info, has_info := DockerRegistryInfo[imageInfo.Registry]; has_info {
		url = info["url"]
		user = info["user"]
		password = info["password"]
	}

	// Установить соединение с registry
	registry, err := registryclient.New(url, user, password)
	if err != nil {
		return "", err
	}
	// Получить описание образа
	antiopaManifest, err := registry.ManifestV2(imageInfo.Repository, imageInfo.Tag)
	if err != nil {
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
		Registry: reference.Domain(namedRef),
		Repository: reference.Path(namedRef),
		Tag: tag,
	}

	rlog.Debugf("REGISTRY image %s parsed to reg=%s repo=%s tag=%s", imageName, imageInfo.Registry, imageInfo.Repository, imageInfo.Tag)

	return
}

