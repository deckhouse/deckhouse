// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamix

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Basis Dynamix 4.6 made storage_policy_id a required field of disks/create and
// kvmx86/create, so a cluster cannot be bootstrapped on anything older. The
// platform exposes no version endpoint to lean on — zone/list, the other
// candidate, answers 404 on 4.6 even though the SDK ships the package — so the
// presence of the storage policy API is itself the version probe: the endpoint
// does not exist before 4.6.
const (
	storagePolicyListPath = "/cloudapi/storage_policy/list"
	accountListPath       = "/cloudapi/account/list"
	accessTokenPath       = "/v1/oauth/access_token"

	// restPrefix is the path every cloudapi call is mounted under on the
	// controller.
	restPrefix = "/restmachine"
)

// StoragePolicyStatusEnabled is the only status a policy can be used as a
// placement target in.
const StoragePolicyStatusEnabled = "ENABLED"

// requestTimeout bounds one request, so that a controller that accepts the
// connection and never answers cannot spend the whole of the caller's budget on
// the first of several requests.
const requestTimeout = 10 * time.Second

// ProviderConfig is the provider section of DynamixClusterConfiguration: where
// the cloud API is and which application credentials log in to it.
type ProviderConfig struct {
	ControllerURL string `yaml:"controllerUrl"`
	OAuth2URL     string `yaml:"oAuth2Url"`
	AppID         string `yaml:"appId"`
	AppSecret     string `yaml:"appSecret"`
	Insecure      bool   `yaml:"insecure"`
}

// StoragePolicy is the part of a storage policy a yes/no question needs. The
// platform returns both `id` and `storage_policy_id`; neither is read, because
// nothing here places anything.
type StoragePolicy struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (p StoragePolicy) Enabled() bool {
	return p.Status == StoragePolicyStatusEnabled
}

type Account struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type storagePolicyList struct {
	Data []StoragePolicy `json:"data"`
}

type accountList struct {
	Data []Account `json:"data"`
}

// ErrStoragePolicyAPIMissing reports that the platform does not serve the
// storage policy API at all, which is how a pre-4.6 platform shows up.
var ErrStoragePolicyAPIMissing = errors.New("the platform does not serve the storage policy API")

// ErrCredentialsRefused reports that the SSO endpoint refused the application
// credentials.
var ErrCredentialsRefused = errors.New("the SSO endpoint refused the application credentials")

// ConfigError reports a field of the provider section the client cannot work
// with. Field is the path under DynamixClusterConfiguration, so that the caller
// can name the line of the configuration to fix.
type ConfigError struct {
	Field   string
	Problem string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("malformed provider cluster configuration: .%s %s", e.Field, e.Problem)
}

// statusError is an answer of the platform other than a success. The transport
// knows nothing more about it than the status: what a status means is up to the
// method that asked.
type statusError struct {
	url    string
	status int
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s answered %d: %s", e.url, e.status, e.body)
}

// transientError marks a failure another attempt could get past: a dropped
// connection, or the platform answering 5xx. Everything else the client returns
// is the platform's answer, and repeating the request cannot change it.
type transientError struct{ err error }

func (e transientError) Error() string { return e.err.Error() }
func (e transientError) Unwrap() error { return e.err }

func transient(err error) error { return transientError{err: err} }

// IsTransient reports whether another attempt could get past err.
func IsTransient(err error) bool {
	var t transientError
	return errors.As(err, &t)
}

// Client is a hand-rolled slice of the Dynamix cloud API: dhctl needs a few
// read-only endpoints, which is not worth pulling the platform SDK (and its
// private module registry) into its build for. Everything it calls lives under
// cloudapi, so it works for an account without cloudbroker rights.
type Client struct {
	controllerURL string
	oAuth2URL     string
	appID         string
	appSecret     string
	httpClient    *http.Client
	token         string
}

func NewClient(provider ProviderConfig) (*Client, error) {
	controllerURL, err := validateURL(provider.ControllerURL, "provider.controllerUrl")
	if err != nil {
		return nil, err
	}
	oAuth2URL, err := validateURL(provider.OAuth2URL, "provider.oAuth2Url")
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if provider.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the user asked for it with .provider.insecure
	}

	return &Client{
		controllerURL: controllerURL,
		oAuth2URL:     oAuth2URL,
		appID:         provider.AppID,
		appSecret:     provider.AppSecret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
		},
	}, nil
}

func validateURL(rawURL, field string) (string, error) {
	if rawURL == "" {
		return "", &ConfigError{Field: field, Problem: "is not set"}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", &ConfigError{Field: field, Problem: fmt.Sprintf("is not a valid URL: %v", err)}
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", &ConfigError{Field: field, Problem: fmt.Sprintf("must be an absolute URL, got %q", rawURL)}
	}
	return strings.TrimSuffix(rawURL, "/"), nil
}

// AccountsNamed returns the accounts named exactly name.
func (c *Client) AccountsNamed(ctx context.Context, name string) ([]Account, error) {
	body, err := c.call(ctx, http.MethodPost, accountListPath, url.Values{"name": []string{name}})
	if err != nil {
		return nil, fmt.Errorf("list Dynamix accounts: %w", err)
	}

	var list accountList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse the Dynamix account list response: %w", err)
	}

	return exactMatches(list.Data, name, func(item Account) string { return item.Name }), nil
}

// AccountStoragePoliciesNamed returns the storage policies named exactly name
// that the account can use: the way the components that place disks resolve a
// policy name. Like StoragePoliciesNamed, it answers ErrStoragePolicyAPIMissing
// on a platform older than 4.6.
func (c *Client) AccountStoragePoliciesNamed(ctx context.Context, accountID uint64, name string) ([]StoragePolicy, error) {
	items, err := c.listStoragePolicies(ctx, url.Values{
		"name":       []string{name},
		"account_id": []string{strconv.FormatUint(accountID, 10)},
	})
	if err != nil {
		return nil, err
	}
	return exactMatches(items, name, func(item StoragePolicy) string { return item.Name }), nil
}

// StoragePoliciesNamed returns the storage policies named exactly name among
// everything the credentials can see, whichever account they belong to.
//
// The storage policy API does not exist before Basis Dynamix 4.6, and the
// platform has no version endpoint to lean on, so this call is the version
// question as well: on an older platform it answers ErrStoragePolicyAPIMissing.
func (c *Client) StoragePoliciesNamed(ctx context.Context, name string) ([]StoragePolicy, error) {
	items, err := c.listStoragePolicies(ctx, url.Values{"name": []string{name}})
	if err != nil {
		return nil, err
	}
	return exactMatches(items, name, func(item StoragePolicy) string { return item.Name }), nil
}

func (c *Client) listStoragePolicies(ctx context.Context, params url.Values) ([]StoragePolicy, error) {
	body, err := c.call(ctx, http.MethodGet, storagePolicyListPath, params)
	var answered *statusError
	if errors.As(err, &answered) && routeIsMissing(answered.status) {
		return nil, fmt.Errorf("%s answered %d: %w", answered.url, answered.status, ErrStoragePolicyAPIMissing)
	}
	if err != nil {
		return nil, err
	}

	var list storagePolicyList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse the Dynamix storage policy list response: %w", err)
	}

	return list.Data, nil
}

// exactMatches keeps the items named exactly want. Both list endpoints take a
// name as a filter, not as a key: the platform answers with every item whose
// name contains it, so the name is always re-checked here.
func exactMatches[T any](items []T, want string, nameOf func(T) string) []T {
	var matches []T
	for _, item := range items {
		if nameOf(item) == want {
			matches = append(matches, item)
		}
	}
	return matches
}

// call performs one cloudapi request. The platform takes its parameters
// form-encoded in the request body even for GET, which is what the platform SDK
// and the terraform provider both send, so that is the shape used here.
func (c *Client) call(ctx context.Context, method, path string, params url.Values) ([]byte, error) {
	if err := c.authenticate(ctx); err != nil {
		return nil, err
	}

	requestURL := c.controllerURL + restPrefix + path
	header := http.Header{}
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	header.Set("Accept", "application/json")
	header.Set("Authorization", "bearer "+c.token)

	status, body, err := c.send(ctx, method, requestURL, params.Encode(), header)
	if err != nil {
		return nil, err
	}

	switch {
	case status == http.StatusOK || status == http.StatusNoContent:
		return body, nil
	case status >= http.StatusInternalServerError:
		return nil, transient(newStatusError(requestURL, status, body))
	default:
		return nil, newStatusError(requestURL, status, body)
	}
}

// routeIsMissing reports whether the answer means "this endpoint is not served
// here", which is how a platform older than 4.6 answers the storage policy API.
// A platform that does serve it never answers a documented GET this way, so
// widening past 404 costs nothing and covers a gateway that rewrites it. 501 is
// a 5xx, so call marks it transient; listStoragePolicies reads it as the verdict
// it is before that matters.
func routeIsMissing(status int) bool {
	return status == http.StatusNotFound ||
		status == http.StatusMethodNotAllowed ||
		status == http.StatusNotImplemented
}

func newStatusError(requestURL string, status int, body []byte) *statusError {
	return &statusError{url: requestURL, status: status, body: strings.TrimSpace(string(body))}
}

// authenticate exchanges the application credentials for an access token. The
// token is fetched once per client: a client lives for one check run, far
// shorter than the token's lifetime, so there is nothing to refresh.
func (c *Client) authenticate(ctx context.Context) error {
	if c.token != "" {
		return nil
	}

	form := url.Values{
		"grant_type":    []string{"client_credentials"},
		"client_id":     []string{c.appID},
		"client_secret": []string{c.appSecret},
		"response_type": []string{"id_token"},
	}
	header := http.Header{}
	header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenURL := c.oAuth2URL + accessTokenPath
	status, body, err := c.send(ctx, http.MethodPost, tokenURL, form.Encode(), header)
	if err != nil {
		return err
	}
	switch {
	case status >= http.StatusInternalServerError:
		return transient(fmt.Errorf("get a Dynamix API token: %w", newStatusError(tokenURL, status, body)))
	case status != http.StatusOK:
		return fmt.Errorf("get a Dynamix API token: %w: %w", ErrCredentialsRefused, newStatusError(tokenURL, status, body))
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		return fmt.Errorf("get a Dynamix API token from %s: the response is empty", tokenURL)
	}

	c.token = token
	return nil
}

func (c *Client) send(ctx context.Context, method, requestURL, body string, header http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, strings.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("create a request to %s: %w", requestURL, err)
	}
	req.Header = header.Clone()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, transient(fmt.Errorf("reach %s: %w", requestURL, err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, transient(fmt.Errorf("read the response from %s: %w", requestURL, err))
	}

	return resp.StatusCode, respBody, nil
}
