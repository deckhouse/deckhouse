/*
Copyright 2021 Flant JSC

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

package cr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocitools "github.com/sylabs/oci-tools/pkg/mutate"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/otel"
)

//go:generate minimock -i Client -o cr_mock.go

const (
	defaultTimeout = 120 * time.Second

	tracerName = "container-registry-client"
)

type Client interface {
	Image(ctx context.Context, tag string) (crv1.Image, error)
	Digest(ctx context.Context, tag string) (string, error)
	ListTags(ctx context.Context) ([]string, error)
}

type client struct {
	registryURL string
	authConfig  authn.AuthConfig
	options     *registryOptions
}

// NewClient creates container registry client using `repo` as prefix for tags passed to methods. If insecure flag is set to true, then no cert validation is performed.
// Repo example: "cr.example.com/ns/app"
func NewClient(repo string, options ...Option) (Client, error) {
	timeout := defaultTimeout
	// make possible to rewrite timeout in runtime
	if t := os.Getenv("REGISTRY_TIMEOUT"); t != "" {
		var err error

		timeout, err = time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("parse duration: %w", err)
		}
	}

	opts := &registryOptions{
		timeout: timeout,
	}

	for _, opt := range options {
		opt(opts)
	}

	r := &client{
		registryURL: repo,
		options:     opts,
	}

	if !opts.withoutAuth {
		authConfig, err := readAuthConfig(repo, opts.dockerCfg)
		if err != nil {
			return nil, fmt.Errorf("read auth config: %w", err)
		}

		r.authConfig = authConfig
	}

	return r, nil
}

func (r *client) Image(ctx context.Context, tag string) (crv1.Image, error) {
	imageURL := r.registryURL + ":" + tag

	var nameOpts []name.Option
	if r.options.useHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	ref, err := name.ParseReference(imageURL, nameOpts...) // parse options available: weak validation, etc.
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	imageOptions := make([]remote.Option, 0)
	imageOptions = append(imageOptions, remote.WithUserAgent(r.options.userAgent))
	if !r.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(r.authConfig)))
	}

	if r.options.ca != "" {
		imageOptions = append(imageOptions, remote.WithTransport(GetHTTPTransport(r.options.ca)))
	}

	if r.options.timeout > 0 {
		// add default timeout to prevent endless request on a huge image
		// Warning!: don't use cancel() in the defer func here. Otherwise *v1.Image outside this function would be inaccessible due to cancelled context, while reading layers, for example.
		ctxWTO, cancel := context.WithTimeout(ctx, r.options.timeout)
		_ = cancel

		imageOptions = append(imageOptions, remote.WithContext(ctxWTO))
	} else {
		imageOptions = append(imageOptions, remote.WithContext(ctx))
	}

	image, err := remote.Image(ref, imageOptions...)
	if err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}

	return image, nil
}

func (r *client) ListTags(ctx context.Context) ([]string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ListTags")
	defer span.End()

	var nameOpts []name.Option
	if r.options.useHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	listOptions := make([]remote.Option, 0)
	if !r.options.withoutAuth {
		listOptions = append(listOptions, remote.WithAuth(authn.FromConfig(r.authConfig)))
	}
	if r.options.ca != "" {
		listOptions = append(listOptions, remote.WithTransport(GetHTTPTransport(r.options.ca)))
	}

	repo, err := name.NewRepository(r.registryURL, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", r.registryURL, err)
	}

	if r.options.timeout > 0 {
		// add default timeout to prevent endless request on a huge amount of tags
		ctxWTO, cancel := context.WithTimeout(ctx, r.options.timeout)
		// here we can use cancel because we return the []strings, not []*v1.Image
		defer cancel()

		listOptions = append(listOptions, remote.WithContext(ctxWTO))
	} else {
		listOptions = append(listOptions, remote.WithContext(ctx))
	}

	list, err := remote.List(repo, listOptions...)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return list, nil
}

func (r *client) Digest(ctx context.Context, tag string) (string, error) {
	image, err := r.Image(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("image: %w", err)
	}

	d, err := image.Digest()
	if err != nil {
		return "", fmt.Errorf("extract digest: %w", err)
	}

	return d.String(), nil
}

func readAuthConfig(repo, dockerCfgBase64 string) (authn.AuthConfig, error) {
	r, err := parse(repo)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("parse repo: %w", err)
	}

	dockerCfg, err := base64.StdEncoding.DecodeString(dockerCfgBase64)
	if err != nil {
		// if base64 decoding failed, try to use input as it is
		dockerCfg = []byte(dockerCfgBase64)
	}
	auths := gjson.Get(string(dockerCfg), "auths").Map()
	authConfig := authn.AuthConfig{}

	// The config should have at least one .auths.* entry
	for repoName, repoAuth := range auths {
		repoNameURL, err := parse(repoName)
		if err != nil {
			return authn.AuthConfig{}, fmt.Errorf("parse repo name: %w", err)
		}

		if repoNameURL.Host == r.Host {
			err := json.Unmarshal([]byte(repoAuth.Raw), &authConfig)
			if err != nil {
				return authn.AuthConfig{}, fmt.Errorf("unmarshal json: %w", err)
			}
			return authConfig, nil
		}
	}

	return authn.AuthConfig{}, fmt.Errorf("%q credentials not found in the dockerCfg", repo)
}

func GetHTTPTransport(ca string) http.RoundTripper {
	if ca == "" {
		return http.DefaultTransport
	}
	caPool, err := x509.SystemCertPool()
	if err != nil {
		panic(fmt.Errorf("cannot get system cert pool: %v", err))
	}

	caPool.AppendCertsFromPEM([]byte(ca))

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultTimeout,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{RootCAs: caPool},
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}

type registryOptions struct {
	ca          string
	useHTTP     bool
	withoutAuth bool
	dockerCfg   string
	userAgent   string
	timeout     time.Duration
}

type Option func(options *registryOptions)

// WithCA use custom CA certificate
func WithCA(ca string) Option {
	return func(options *registryOptions) {
		options.ca = ca
	}
}

// WithInsecureSchema use http schema instead of https
func WithInsecureSchema(insecure bool) Option {
	return func(options *registryOptions) {
		options.useHTTP = insecure
	}
}

// WithAuth use docker config base64 as authConfig
// if dockerCfg is empty - will use client without auth
func WithAuth(dockerCfg string) Option {
	return func(options *registryOptions) {
		options.dockerCfg = dockerCfg
		if dockerCfg == "" {
			options.withoutAuth = true
		}
	}
}

// WithUserAgent adds ua string to the User-Agent header
func WithUserAgent(ua string) Option {
	return func(options *registryOptions) {
		options.userAgent = ua
	}
}

// WithTimeout limit and request to a registry with a timeout
// default timeout is 30 seconds
func WithTimeout(timeout time.Duration) Option {
	return func(options *registryOptions) {
		options.timeout = timeout
	}
}

// parse parses url without scheme://
// if we pass url without scheme ve've got url back with two leading slashes
func parse(rawURL string) (*url.URL, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return url.ParseRequestURI(rawURL)
	}
	return url.Parse("//" + rawURL)
}

// Extract flattens the image to a single layer and returns ReadCloser for fetching the content
func Extract(image crv1.Image) (io.ReadCloser, error) {
	flattenedImage, err := ocitools.Squash(image)
	if err != nil {
		return nil, fmt.Errorf("flattening image to a single layer: %w", err)
	}

	imageLayers, err := flattenedImage.Layers()
	if err != nil {
		return nil, fmt.Errorf("getting the image's layers: %w", err)
	}

	if len(imageLayers) != 1 {
		return nil, fmt.Errorf("unexpected number of layers: %w", err)
	}

	rc, err := imageLayers[0].Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("uncompress the layer: %w", err)
	}

	return rc, nil
}
