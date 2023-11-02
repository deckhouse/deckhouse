package registryclient

import (
	"fmt"
	registryscaner "registry-modules-watcher/internal/backends/pkg/registry-scaner"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type registryOptions struct {
	ca          string
	useHTTP     bool
	withoutAuth bool
	dockerCfg   string
}

type Option func(options *registryOptions)

type client struct {
	registryURL string // registry.deckhouse.io/deckhouse/fe/modules
	authConfig  authn.AuthConfig
	options     *registryOptions
}

// NewClient creates container registry client using `repo` as prefix for tags passed to methods. If insecure flag is set to true, then no cert validation is performed.
// Repo example: "cr.example.com/ns/app"
func NewClient(repo string, options ...Option) (registryscaner.Client, error) {
	opts := &registryOptions{}

	for _, opt := range options {
		opt(opts)
	}

	client := &client{
		registryURL: repo,
		options:     opts,
	}

	if !opts.withoutAuth {
		authConfig, err := readAuthConfig(repo, opts.dockerCfg)
		if err != nil {
			return nil, err
		}
		client.authConfig = authConfig
	}

	return client, nil
}

func (r *client) Name() string {
	return r.registryURL // TODO
}

func (r *client) ReleaseImage(moduleName, tag string) (v1.Image, error) {
	imageURL := r.registryURL + "/" + moduleName + "/release" + ":" + tag
	return r.image(imageURL)
}

func (r *client) Image(moduleName, tag string) (v1.Image, error) {
	imageURL := r.registryURL + "/" + moduleName + ":" + tag
	return r.image(imageURL)
}

func (r *client) image(imageURL string) (v1.Image, error) {
	var nameOpts []name.Option
	if r.options.useHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	ref, err := name.ParseReference(imageURL, nameOpts...) // parse options available: weak validation, etc.
	if err != nil {
		return nil, err
	}

	imageOptions := make([]remote.Option, 0)
	if !r.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(r.authConfig)))
	}
	if r.options.ca != "" {
		imageOptions = append(imageOptions, remote.WithTransport(getHTTPTransport(r.options.ca)))
	}

	return remote.Image(
		ref,
		imageOptions...,
	)
}

func (r *client) Modules() ([]string, error) {
	var nameOpts []name.Option
	if r.options.useHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	imageOptions := make([]remote.Option, 0)
	if !r.options.withoutAuth { // TODO обрати внимание на этот флаг
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(r.authConfig)))
	}
	if r.options.ca != "" {
		imageOptions = append(imageOptions, remote.WithTransport(getHTTPTransport(r.options.ca)))
	}

	repo, err := name.NewRepository(r.registryURL, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", r.registryURL, err)
	}

	return remote.List(repo, imageOptions...)
}

func (r *client) ListTags(moduleName string) ([]string, error) {
	var nameOpts []name.Option
	if r.options.useHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	imageOptions := make([]remote.Option, 0)
	if !r.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(r.authConfig)))
	}
	if r.options.ca != "" {
		imageOptions = append(imageOptions, remote.WithTransport(getHTTPTransport(r.options.ca)))
	}
	url := r.registryURL + "/" + moduleName + "/release" // TODO
	repo, err := name.NewRepository(url, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", r.registryURL, err)
	}

	return remote.List(repo, imageOptions...)
}

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

// WithDisabledAuth don't use authConfig
func WithDisabledAuth() Option {
	return func(options *registryOptions) {
		options.withoutAuth = true
	}
}

// WithAuth use docker config base64 as authConfig
func WithAuth(dockerCfg string) Option {
	return func(options *registryOptions) {
		options.dockerCfg = dockerCfg
	}
}
