package registryclient

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
)

func readAuthConfig(repo, dockerCfg string) (authn.AuthConfig, error) {
	r, err := parse(repo)
	if err != nil {
		return authn.AuthConfig{}, err
	}

	var auths dockercfgAuths

	err = json.Unmarshal([]byte(dockerCfg), &auths)
	if err != nil {
		return authn.AuthConfig{}, err
	}

	// The config should have at least one .auths.* entry
	for repoName, repoAuth := range auths.Auths {
		if repoName == r.Host {
			return repoAuth, nil
		}
	}

	return authn.AuthConfig{}, fmt.Errorf("no auth data")
}

type dockercfgAuths struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

// parse parses url without scheme://
// if we pass url without scheme ve've got url back with two leading slashes
func parse(rawURL string) (*url.URL, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return url.ParseRequestURI(rawURL)
	}
	return url.Parse("//" + rawURL)
}

func getHTTPTransport(ca string) (transport http.RoundTripper) {
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
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{RootCAs: caPool},
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}
