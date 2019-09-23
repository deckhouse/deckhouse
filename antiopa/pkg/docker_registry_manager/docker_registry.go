package docker_registry_manager

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	registryclient "github.com/flant/docker-registry-client/registry"
	"github.com/romana/rlog"
)

type DockerImageInfo struct {
	Registry   string // url для registry
	Repository string // репозиторий в registry (antiopa)
	Tag        string // tag репозитория (master, stable, ea, etc.)
	FullName   string // полное имя образа для лога
}

// KubeDigestRe regexp detects if imageID field contains docker image digest and not docker image id
var KubeDigestRe = regexp.MustCompile("docker-pullable://.*@sha256:[a-fA-F0-9]{64}")

// DockerImageDigestRe regexp extracts docker image digest from string
var DockerImageDigestRe = regexp.MustCompile("(sha256:?)?[a-fA-F0-9]{64}")

//var KubeImageIdRe = regexp.MustCompile("docker://sha256:[a-fA-F0-9]{64}")

// Отправить запрос в registry, из заголовка ответа достать digest.
// Если произошла какая-то ошибка, то сообщить в лог и вернуть пустую
// строку — метод нужно вызывать в цикле, пока registry не ответит успешно.
//
// Запрос к registry может паниковать — проблема где-то в docker-registry-client,
// но случается очень редко, предположительно когда registry становится
// недоступен из куба — трудно диагностируемо.
// Поэтому проще тут поймать panic и вывести в Debug лог.
func DockerRegistryGetImageDigest(imageInfo DockerImageInfo, dockerRegistry *registryclient.Registry) (digest string, err error) {
	defer func() {
		if r := recover(); r != nil {
			rlog.Debugf("REGISTRY: manifest digest request panic: %s", r)
			rlog.Debugf("%s", debug.Stack())
		}
	}()

	// Получить digest образа
	imageDigest, err := dockerRegistry.ManifestDigestV2(imageInfo.Repository, imageInfo.Tag)
	if err != nil {
		rlog.Debugf("REGISTRY: manifest digest request error for %s/%s:%s: %v", imageInfo.Registry, imageInfo.Repository, imageInfo.Tag, err)
		return "", err
	}

	digest = imageDigest.String()
	rlog.Debugf("REGISTRY: imageDigest='%s' for %s:%s", digest, imageInfo.Repository, imageInfo.Tag)

	return digest, nil
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
		FullName:   imageName,
	}

	rlog.Debugf("REGISTRY image %s parsed to reg=%s repo=%s tag=%s", imageName, imageInfo.Registry, imageInfo.Repository, imageInfo.Tag)

	return
}

// Поиск digest в строке.
// Учитывается специфика kubernetes — если есть префикс docker-pullable://, то в строке digest.
// Если префикс docker:// или нет префикса, то скорее всего там imageId, который нельзя
// применить для обновления, поэтому возвращается ошибка
// Пример строки с digest из kubernetes: docker-pullable://registry/repo:tag@sha256:DIGEST-HASH
func FindImageDigest(imageId string) (image string, err error) {
	if !KubeDigestRe.MatchString(imageId) {
		err = fmt.Errorf("Pod status contains image_id and not digest. Antiopa update process not working in clusters with Docker 1.11 or earlier.")
		return "", err
	}
	image = DockerImageDigestRe.FindString(imageId)
	return image, nil
}

// Проверка, что строка это docker digest
func IsValidImageDigest(imageId string) bool {
	return DockerImageDigestRe.MatchString(imageId)
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
