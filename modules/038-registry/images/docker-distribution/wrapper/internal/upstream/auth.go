/*
Copyright 2026 Flant JSC

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

package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// tokenRequestTimeout bounds one exchange with the upstream's token service. It is on the path of a
// cache miss, and a token service that hangs must fail the miss rather than hold it.
const tokenRequestTimeout = 30 * time.Second

// defaultTokenLifetime is what the token specification says to assume when the service names none.
const defaultTokenLifetime = 60 * time.Second

// authenticator is the upstream's authentication, done here and not by the cache.
//
// The cache cannot do it, for two reasons that are each enough on their own. It asks for a token
// scoped to the repository under the CLUSTER's name (`system/deckhouse/...`), which the upstream knows
// nothing about and grants nothing for; and it only sends its credentials to a token service on the
// host it believes is the upstream — the loopback — so the real one would be asked anonymously. So the
// cache is told there is nothing to authenticate (see Handler), whatever it sends is dropped, and this
// answers the upstream's challenges with the upstream's own scope.
//
// Basic credentials go with every request that has no token yet, as they always have: an upstream
// that accepts them answers at once, and one that wants a token challenges, which is where the token
// comes from. The token is kept per repository until it expires, so a pull costs one exchange and not
// one per layer.
type authenticator struct {
	next     http.RoundTripper
	target   url.URL
	username string
	password string
	log      *slog.Logger

	mu     sync.Mutex
	tokens map[string]token
}

type token struct {
	value   string
	expires time.Time
}

func newAuthenticator(next http.RoundTripper, target url.URL, username, password string, log *slog.Logger) *authenticator {
	return &authenticator{
		next:     next,
		target:   target,
		username: username,
		password: password,
		log:      log,
		tokens:   make(map[string]token),
	}
}

func (a *authenticator) hasCredentials() bool {
	return a.username != "" || a.password != ""
}

func (a *authenticator) RoundTrip(request *http.Request) (*http.Response, error) {
	repository := repositoryOf(request.URL.Path)

	cached, haveToken := a.token(repository)
	response, err := a.next.RoundTrip(a.authorize(request, cached, haveToken))
	if err != nil {
		return nil, err
	}

	// Only a request without a body can be sent twice, and the cache sends nothing else: a
	// pull-through cache reads.
	if response.StatusCode != http.StatusUnauthorized || !replayable(request) {
		return response, nil
	}

	bearer, ok := bearerChallenge(response)
	if !ok {
		return a.refused(response), nil
	}

	issued, err := a.fetchToken(request, bearer, repository)
	if err != nil {
		// The refusal is still the answer the cache gets: what failed is authenticating with the
		// upstream, and a gateway failure would read as the upstream being down.
		if a.log != nil {
			a.log.Warn("the upstream's token service did not issue a token",
				"upstream", a.target.Host, "repository", repository, "error", err.Error())
		}
		return a.refused(response), nil
	}
	drain(response)

	a.store(repository, issued)

	response, err = a.next.RoundTrip(a.authorize(request, issued.value, true))
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusUnauthorized {
		// A token the service just issued and the registry still refuses is not worth keeping.
		a.forget(repository)
		if a.log != nil {
			a.log.Warn("the upstream refused the token its own service issued",
				"upstream", a.target.Host, "repository", repository)
		}
		return a.refused(response), nil
	}
	return response, nil
}

// authorize clones the request with this process's authorization and nothing the cache set.
func (a *authenticator) authorize(request *http.Request, bearer string, haveToken bool) *http.Request {
	outgoing := request.Clone(request.Context())
	outgoing.Header.Del("Authorization")

	switch {
	case haveToken:
		outgoing.Header.Set("Authorization", "Bearer "+bearer)
	case a.hasCredentials():
		outgoing.SetBasicAuth(a.username, a.password)
	}
	return outgoing
}

// refused hands a refusal to the cache without the challenge in it.
//
// A challenge that reaches the cache names the upstream's token service, and the cache would take it
// up with the cluster's repository name — exactly the exchange this type exists to replace.
func (a *authenticator) refused(response *http.Response) *http.Response {
	response.Header.Del("Www-Authenticate")
	return response
}

// fetchToken exchanges the credentials for a token the way the token specification describes it.
func (a *authenticator) fetchToken(request *http.Request, challenge map[string]string, repository string) (token, error) {
	realm, err := url.Parse(challenge["realm"])
	if err != nil || realm.Host == "" {
		return token{}, fmt.Errorf("the upstream names no usable token service (%q)", challenge["realm"])
	}

	query := realm.Query()
	if service := challenge["service"]; service != "" {
		query.Set("service", service)
	}
	scopes := strings.Fields(challenge["scope"])
	if len(scopes) == 0 && repository != "" {
		scopes = []string{"repository:" + repository + ":pull"}
	}
	for _, scope := range scopes {
		query.Add("scope", scope)
	}
	realm.RawQuery = query.Encode()

	ctx := request.Context()
	exchange, err := http.NewRequestWithContext(ctx, http.MethodGet, realm.String(), nil)
	if err != nil {
		return token{}, err
	}
	if a.hasCredentials() && a.mayReceiveCredentials(realm) {
		exchange.SetBasicAuth(a.username, a.password)
	}

	client := &http.Client{Transport: a.next, Timeout: tokenRequestTimeout}
	response, err := client.Do(exchange)
	if err != nil {
		return token{}, err
	}
	defer drain(response)

	if response.StatusCode != http.StatusOK {
		return token{}, fmt.Errorf("%s answered %s", realm.Host, response.Status)
	}

	var body struct {
		Token       string    `json:"token"`
		AccessToken string    `json:"access_token"`
		ExpiresIn   int       `json:"expires_in"`
		IssuedAt    time.Time `json:"issued_at"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return token{}, fmt.Errorf("decoding the answer of %s: %w", realm.Host, err)
	}

	value := body.Token
	if value == "" {
		value = body.AccessToken
	}
	if value == "" {
		return token{}, fmt.Errorf("%s issued no token", realm.Host)
	}

	lifetime := defaultTokenLifetime
	if body.ExpiresIn > 0 {
		lifetime = time.Duration(body.ExpiresIn) * time.Second
	}
	// Renewed early, so that a token does not expire between being picked and being checked.
	lifetime -= min(lifetime/2, 10*time.Second)

	return token{value: value, expires: time.Now().Add(lifetime)}, nil
}

// mayReceiveCredentials refuses to put the upstream's credentials on the wire in the clear to a host
// that is not the upstream itself.
func (a *authenticator) mayReceiveCredentials(realm *url.URL) bool {
	if strings.EqualFold(realm.Scheme, "https") {
		return true
	}
	return strings.EqualFold(realm.Host, a.target.Host)
}

func (a *authenticator) token(repository string) (string, bool) {
	if repository == "" {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	cached, ok := a.tokens[repository]
	if !ok || time.Now().After(cached.expires) {
		return "", false
	}
	return cached.value, true
}

func (a *authenticator) store(repository string, issued token) {
	if repository == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[repository] = issued
}

func (a *authenticator) forget(repository string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tokens, repository)
}

// bearerChallenge returns the parameters of the upstream's Bearer challenge, if it made one.
func bearerChallenge(response *http.Response) (map[string]string, bool) {
	for _, header := range response.Header.Values("Www-Authenticate") {
		scheme, rest, _ := strings.Cut(strings.TrimSpace(header), " ")
		if !strings.EqualFold(scheme, "bearer") {
			continue
		}
		parameters := parseChallengeParameters(rest)
		if parameters["realm"] == "" {
			continue
		}
		return parameters, true
	}
	return nil, false
}

// parseChallengeParameters reads `key="value", key=value` pairs, where a quoted value may hold commas
// and spaces — a scope listing several repositories does.
func parseChallengeParameters(input string) map[string]string {
	parameters := make(map[string]string)
	for {
		input = strings.TrimLeft(input, " \t,")
		key, rest, found := strings.Cut(input, "=")
		if !found {
			return parameters
		}
		key = strings.ToLower(strings.TrimSpace(key))

		var value strings.Builder
		if quoted, ok := strings.CutPrefix(rest, `"`); ok {
			i := 0
			for ; i < len(quoted) && quoted[i] != '"'; i++ {
				if quoted[i] == '\\' && i+1 < len(quoted) {
					i++
				}
				value.WriteByte(quoted[i])
			}
			input = quoted[min(i+1, len(quoted)):]
		} else {
			plain, remainder, _ := strings.Cut(rest, ",")
			value.WriteString(strings.TrimSpace(plain))
			input = remainder
		}
		parameters[key] = value.String()
	}
}

// repositoryOf names the repository a registry API path is about, or "" for a path about none.
//
// The name may itself contain any of the words that follow it, so each is looked for from the right:
// what follows the name — a tag, a digest — never contains a slash.
func repositoryOf(path string) string {
	rest, found := strings.CutPrefix(path, "/v2/")
	if !found {
		return ""
	}

	if name, found := strings.CutSuffix(rest, "/tags/list"); found {
		return name
	}
	for _, separator := range []string{"/manifests/", "/blobs/", "/referrers/"} {
		if i := strings.LastIndex(rest, separator); i > 0 {
			if !strings.Contains(rest[i+len(separator):], "/") {
				return rest[:i]
			}
		}
	}
	return ""
}

func replayable(request *http.Request) bool {
	return (request.Method == http.MethodGet || request.Method == http.MethodHead) &&
		(request.Body == nil || request.Body == http.NoBody)
}

// drain reads what is left of a body so that the connection can be reused, and closes it.
func drain(response *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	_ = response.Body.Close()
}
