package docker_registry_watcher

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"flant/deckhouse/pkg/app"
)

func ImageDigest(ref name.Reference) (string, error) {
	img, err := GetImage(ref)
	if err != nil {
		return "", err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", err
	}

	return digest.String(), nil
}

func GetImage(ref name.Reference) (v1.Image, error) {

	img, err := remote.Image(ref,
		remote.WithAuthFromKeychain(NewKeychain()),
		remote.WithTransport(GetHttpTransport()))

	if err != nil {
		return nil, fmt.Errorf("reading image %q: %v", ref, err)
	}

	return img, nil
}

func ParseReferenceOptions() []name.Option {
	var options []name.Option
	options = append(options, name.WeakValidation)

	if app.InsecureRegistry == "yes" {
		options = append(options, name.Insecure)
	}

	return options
}

func GetHttpTransport() (transport http.RoundTripper) {
	if app.SkipTlsVerifyRegistry == "yes" {
		// default http transport with InsecureSkipVerify
		return &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		}
	} else {
		return http.DefaultTransport
	}
}
