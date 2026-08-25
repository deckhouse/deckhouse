package proxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
)

// authChallenger encapsulates a request to the upstream to establish credential challenges
type authChallenger interface {
	tryEstablishChallenges(context.Context) error
	challengeManager() challenge.Manager

	basicCredentials() auth.CredentialStore
	tokenCredentials() auth.CredentialStore
}

func newAuthChallenger(remoteURL url.URL, client *http.Client, username, password string) authChallenger {
	authCreds := userpass{
		username: username,
		password: password,
	}

	return &remoteAuthChallenger{
		remoteURL:  remoteURL,
		httpClient: client,
		cm:         challenge.NewSimpleManager(),
		basicCreds: basicCredentials{
			creds: authCreds,
		},
		tokenCreds: tokenCredentials{
			creds: authCreds,
		},
	}
}

type remoteAuthChallenger struct {
	remoteURL  url.URL
	httpClient *http.Client
	mu         sync.Mutex
	cm         challenge.Manager

	basicCreds basicCredentials
	tokenCreds tokenCredentials
}

func (r *remoteAuthChallenger) basicCredentials() auth.CredentialStore {
	return &r.basicCreds
}

func (r *remoteAuthChallenger) tokenCredentials() auth.CredentialStore {
	return &r.tokenCreds
}

func (r *remoteAuthChallenger) challengeManager() challenge.Manager {
	return r.cm
}

// tryEstablishChallenges will attempt to get a challenge type for the upstream if none currently exist
func (r *remoteAuthChallenger) tryEstablishChallenges(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	remoteURL := r.remoteURL
	remoteURL.Path = "/v2/"

	resp, err := r.httpClient.Get(remoteURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = r.cm.AddResponse(resp)
	if err != nil {
		return err
	}

	challenges, err := r.cm.GetChallenges(remoteURL)
	if err != nil {
		return err
	}

	tokenAuth := r.tokenCreds.updateAuthUrls(challenges)

	if tokenAuth {
		r.basicCreds.SetUrlPrefix("")
	} else {
		r.basicCreds.SetUrlPrefix(r.remoteURL.String())
	}

	dcontext.GetLogger(ctx).
		Infof(
			"Challenge established with upstream (URL: %s, challenges: %+v, token_auth: %v)\n",
			remoteURL.String(), challenges, tokenAuth,
		)
	return nil
}

type userpass struct {
	username string
	password string
}

type tokenCredentials struct {
	mu       sync.RWMutex
	creds    userpass
	authUrls map[string]struct{}
}

func (c *tokenCredentials) Basic(u *url.URL) (string, string) {
	c.mu.RLock()
	_, ok := c.authUrls[u.String()]
	c.mu.RUnlock()

	if ok {
		return c.creds.username, c.creds.password
	}
	return "", ""
}

func (c *tokenCredentials) updateAuthUrls(challenges []challenge.Challenge) bool {
	authURLs := make(map[string]struct{})
	for _, c := range challenges {
		if strings.EqualFold(c.Scheme, "bearer") {
			authURLs[c.Parameters["realm"]] = struct{}{}
		}
	}

	c.mu.Lock()
	c.authUrls = authURLs
	c.mu.Unlock()

	return len(authURLs) > 0
}

func (c *tokenCredentials) RefreshToken(u *url.URL, service string) string {
	return ""
}

func (c *tokenCredentials) SetRefreshToken(u *url.URL, service, token string) {
}

type basicCredentials struct {
	mu        sync.RWMutex
	creds     userpass
	urlPrefix string
}

func (c *basicCredentials) Basic(u *url.URL) (string, string) {
	c.mu.RLock()
	urlPrefix := c.urlPrefix
	c.mu.RUnlock()

	if urlPrefix == "" || !strings.HasPrefix(u.String(), urlPrefix) {
		return "", ""
	}
	return c.creds.username, c.creds.password
}

func (c *basicCredentials) SetUrlPrefix(urlPrefix string) {
	c.mu.Lock()
	c.urlPrefix = urlPrefix
	c.mu.Unlock()
}

func (c *basicCredentials) RefreshToken(u *url.URL, service string) string {
	return ""
}

func (c *basicCredentials) SetRefreshToken(u *url.URL, service, token string) {
}
