package registry

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/docker/distribution/configuration"
	dcontext "github.com/docker/distribution/context"
	"gopkg.in/yaml.v2"
)

type authProxyOptions struct {
	Url string `yaml:"url"`
	CA  string `yaml:"ca"`
}

func getAuthProxyOptions(data any) (params authProxyOptions, err error) {
	buf, err := yaml.Marshal(data)
	if err != nil {
		err = fmt.Errorf("cannot marshal data: %w", err)
		return
	}

	err = yaml.Unmarshal(buf, &params)
	if err != nil {
		err = fmt.Errorf("cannot unmarshal data: %w", err)
	}

	return
}

func authProxyHandler(ctx context.Context, config *configuration.Configuration, h http.Handler) http.Handler {
	l := dcontext.GetLogger(ctx)
	if config.Auth.Type() != "token" {
		l.Info("Auth type is not token, auth proxy disabled")
		return h
	}

	params, ok := config.Auth.Parameters()["proxy"]
	if !ok || params == nil {
		l.Info("Auth proxy disabled: config not found")
		return h
	}

	opts, err := getAuthProxyOptions(params)
	if err != nil {
		l.
			WithError(err).
			Fatalln("Cannot get auth proxy options")

		return h
	}

	if opts.Url == "" {
		l.Info("Auth proxy disabled: url not set")
		return h
	}

	remote, err := url.Parse(opts.Url)
	if err != nil {
		l.
			WithError(err).
			Fatalln("Cannot parse auth proxy URL")

		return h
	}

	proxyOpts := []string{
		fmt.Sprintf("upstream: \"%v\"", remote),
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	if opts.CA != "" {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			l.
				WithError(err).
				Fatalln("Cannot load auth proxy system CAs pool")

			return h
		}

		pem, err := os.ReadFile(opts.CA)
		if err != nil {
			l.
				WithError(err).
				Fatalf("Cannot load auth proxy CA file %v", opts.CA)

			return h
		}

		certPool.AppendCertsFromPEM(pem)

		transport.TLSClientConfig.RootCAs = certPool

		proxyOpts = append(proxyOpts, fmt.Sprintf("CA: \"%v\"", opts.CA))
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()

			if r.In.URL.Scheme == "https" || r.In.URL.Scheme == "http" {
				r.Out.Header.Set("X-Forwarded-Proto", r.In.URL.Scheme)
			}

			r.SetURL(remote)
			r.Out.URL.Path = remote.Path

			r.Out.Host = r.In.Host // keep Host header
		},
		Transport: transport,
	}

	ret := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/token" {
			proxy.ServeHTTP(w, r)
			return
		}

		h.ServeHTTP(w, r)
	})

	l.Infof("Auth proxy enabled (%v)", strings.Join(proxyOpts, ", "))

	return ret
}
