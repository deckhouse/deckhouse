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

package checks

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

	"gopkg.in/yaml.v3"
)

// Basis Dynamix 4.6 made storage_policy_id a required field of disks/create and
// kvmx86/create, so a cluster cannot be bootstrapped on anything older. The
// platform exposes no version endpoint to lean on — zone/list, the other
// candidate, answers 404 on 4.6 even though the SDK ships the package — so the
// presence of the storage policy API is itself the version probe: the endpoint
// does not exist before 4.6.
const (
	dynamixStoragePolicyListPath = "/cloudapi/storage_policy/list"
	dynamixAccountListPath       = "/cloudapi/account/list"
	dynamixAccessTokenPath       = "/v1/oauth/access_token"

	// dynamixRESTPrefix is the path every cloudapi call is mounted under on the
	// controller.
	dynamixRESTPrefix = "/restmachine"

	// dynamixStoragePolicyStatusEnabled is the only status a policy can be used
	// as a placement target in.
	dynamixStoragePolicyStatusEnabled = "ENABLED"
)

// The check owns a bootstrap's first contact with the platform, so it must not
// hard-fail on a single dropped connection — and must not hang either.
const (
	dynamixAPIRequestTimeout = 20 * time.Second
	dynamixAPIAttempts       = 3
	dynamixAPIRetryInterval  = 2 * time.Second
	dynamixAPITimeout        = 2 * time.Minute
)

// ErrDynamixPlatformTooOld reports that the platform does not know the storage
// policy API at all, which is how a pre-4.6 platform shows up here.
var ErrDynamixPlatformTooOld = errors.New(
	"the Dynamix platform does not provide the storage policy API: Basis Dynamix 4.6 or newer is required. " +
		"Check the platform version and .provider.controllerUrl")

var (
	errDynamixPolicyNotFound  = errors.New("storage policy not found")
	errDynamixPolicyAmbiguous = errors.New("storage policy name is ambiguous")
)

// checkDynamixStoragePolicies verifies, before any infrastructure is created,
// that the platform is 4.6+ and that every storage policy named in the cluster
// configuration can actually be used: it exists, it is ENABLED, and it is
// available to the configured account. Without this the user learns about it
// halfway through master creation, as an opaque `400 storage_policy_id Field
// required` from the platform.
func checkDynamixStoragePolicies(ctx context.Context, providerClusterConfig []byte) error {
	clusterConfig, err := parseDynamixClusterConfiguration(providerClusterConfig)
	if err != nil {
		return err
	}

	client, err := newDynamixAPIClient(clusterConfig.Provider)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, dynamixAPITimeout)
	defer cancel()

	policies := clusterConfig.storagePolicies()

	// The platform is asked about the policies themselves first, before the
	// account is even resolved. On a pre-4.6 platform this is the call that
	// answers 404, and the version is the one thing the user has to fix: any
	// other complaint at this point — a mistyped account, say — would only
	// distract from it.
	for _, policy := range policies {
		if err := client.checkStoragePolicyExists(ctx, policy); err != nil {
			return err
		}
	}

	accountID, err := client.accountIDByName(ctx, clusterConfig.Account)
	if err != nil {
		return err
	}

	for _, policy := range policies {
		if err := client.checkStoragePolicyAvailable(ctx, policy, clusterConfig.Account, accountID); err != nil {
			return err
		}
	}

	return nil
}

// checkStoragePolicyExists answers the first three questions at once, because
// the platform answers them in one call: whether it knows the storage policy API
// at all (it does not before 4.6), whether the policy is there, and whether it is
// in a status a disk can be created in.
func (c *dynamixAPIClient) checkStoragePolicyExists(ctx context.Context, policy dynamixPolicyRef) error {
	items, err := c.listStoragePolicies(ctx, policy.name, 0)
	if err != nil {
		return err
	}

	found, err := matchDynamixStoragePolicy(items, policy)
	if err != nil {
		return err
	}

	return dynamixPolicyStatusError(found, policy)
}

// checkStoragePolicyAvailable asks for the policy the way the components that
// will place disks in it ask — narrowed to the account, as dynamix-common's
// GetStoragePolicyByName does — so that a policy accepted here is a policy CSI,
// CAPD and terraform will also resolve. The policy is already known to exist by
// this point, so its absence under the account filter means one thing.
func (c *dynamixAPIClient) checkStoragePolicyAvailable(ctx context.Context, policy dynamixPolicyRef, accountName string, accountID uint64) error {
	items, err := c.listStoragePolicies(ctx, policy.name, accountID)
	if err != nil {
		return err
	}

	if _, err := matchDynamixStoragePolicy(items, policy); err != nil {
		if errors.Is(err, errDynamixPolicyNotFound) {
			return fmt.Errorf("%s is not available to account %q: grant the account access to the policy or name another one",
				policy, accountName)
		}
		return err
	}

	return nil
}

func dynamixPolicyStatusError(found dynamixStoragePolicy, policy dynamixPolicyRef) error {
	if found.Status == dynamixStoragePolicyStatusEnabled {
		return nil
	}
	return fmt.Errorf("%s is in status %q, expected %q: disks cannot be created in a policy that is not enabled",
		policy, found.Status, dynamixStoragePolicyStatusEnabled)
}

func matchDynamixStoragePolicy(items []dynamixStoragePolicy, policy dynamixPolicyRef) (dynamixStoragePolicy, error) {
	matches := dynamixExactMatches(items, policy.name, func(item dynamixStoragePolicy) string { return item.Name })

	switch len(matches) {
	case 0:
		return dynamixStoragePolicy{}, fmt.Errorf("%s does not exist: %w", policy, errDynamixPolicyNotFound)
	case 1:
		return matches[0], nil
	default:
		return dynamixStoragePolicy{}, fmt.Errorf("%s matches %d policies: %w", policy, len(matches), errDynamixPolicyAmbiguous)
	}
}

// dynamixExactMatches keeps the items named exactly want. Both list endpoints
// take a name as a filter, not as a key: the platform is free to answer with
// more than the exact match, so the name is always re-checked here. Answering
// with the first item instead would silently bootstrap into the wrong object,
// which is why an ambiguous name is refused rather than resolved.
func dynamixExactMatches[T any](items []T, want string, nameOf func(T) string) []T {
	var matches []T
	for _, item := range items {
		if nameOf(item) == want {
			matches = append(matches, item)
		}
	}
	return matches
}

// dynamixPolicyRef is a storage policy name together with the field it was read
// from, so that an error tells the user which line of their configuration to fix.
type dynamixPolicyRef struct {
	name  string
	field string
}

func (p dynamixPolicyRef) String() string {
	return fmt.Sprintf("storage policy %q (.%s)", p.name, p.field)
}

type dynamixInstanceClass struct {
	StoragePolicy string `yaml:"storagePolicy"`
}

type dynamixProvider struct {
	ControllerURL string `yaml:"controllerUrl"`
	OAuth2URL     string `yaml:"oAuth2Url"`
	AppID         string `yaml:"appId"`
	AppSecret     string `yaml:"appSecret"`
	Insecure      bool   `yaml:"insecure"`
}

type dynamixClusterConfiguration struct {
	Account         string          `yaml:"account"`
	StoragePolicy   string          `yaml:"storagePolicy"`
	Provider        dynamixProvider `yaml:"provider"`
	MasterNodeGroup struct {
		InstanceClass dynamixInstanceClass `yaml:"instanceClass"`
	} `yaml:"masterNodeGroup"`
	NodeGroups []struct {
		Name          string               `yaml:"name"`
		InstanceClass dynamixInstanceClass `yaml:"instanceClass"`
	} `yaml:"nodeGroups"`
}

// storagePolicies returns every distinct policy the cluster configuration names:
// the cluster-wide one plus the per-instance-class overrides, which fail a
// bootstrap exactly the same way. Each is kept with the first field that named
// it, and the order is the order of the configuration.
func (c dynamixClusterConfiguration) storagePolicies() []dynamixPolicyRef {
	var policies []dynamixPolicyRef
	seen := make(map[string]struct{})

	add := func(name, field string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		policies = append(policies, dynamixPolicyRef{name: name, field: field})
	}

	add(c.StoragePolicy, "storagePolicy")
	add(c.MasterNodeGroup.InstanceClass.StoragePolicy, "masterNodeGroup.instanceClass.storagePolicy")
	for i, nodeGroup := range c.NodeGroups {
		add(nodeGroup.InstanceClass.StoragePolicy, fmt.Sprintf("nodeGroups[%d].instanceClass.storagePolicy", i))
	}

	return policies
}

// parseDynamixClusterConfiguration re-states the presence of the fields the
// check reads. The OpenAPI schema already lists them in `required`, so a
// configuration that came through `dhctl config parse` cannot miss them — the
// guards are here so that one which reached preflight some other way fails
// loudly instead of skipping the check on an empty policy name.
func parseDynamixClusterConfiguration(providerClusterConfig []byte) (dynamixClusterConfiguration, error) {
	var clusterConfig dynamixClusterConfiguration
	if err := yaml.Unmarshal(providerClusterConfig, &clusterConfig); err != nil {
		return dynamixClusterConfiguration{}, fmt.Errorf("malformed provider cluster configuration: %w", err)
	}

	if clusterConfig.Account == "" {
		return dynamixClusterConfiguration{}, errors.New("malformed provider cluster configuration: reading .account: no such property")
	}
	if clusterConfig.StoragePolicy == "" {
		return dynamixClusterConfiguration{}, errors.New("malformed provider cluster configuration: reading .storagePolicy: no such property")
	}

	return clusterConfig, nil
}

// dynamixStoragePolicy is the part of a storage policy preflight needs. The
// platform returns both `id` and `storage_policy_id`; neither is used here,
// because the check answers a yes/no question and never places anything.
type dynamixStoragePolicy struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type dynamixStoragePolicyList struct {
	Data []dynamixStoragePolicy `json:"data"`
}

type dynamixAccount struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type dynamixAccountList struct {
	Data []dynamixAccount `json:"data"`
}

// dynamixAPIClient is a hand-rolled slice of the Dynamix cloud API: preflight
// needs two read-only endpoints, which is not worth pulling the platform SDK
// (and its private module registry) into dhctl's build for. Both endpoints live
// under cloudapi, so the check passes for an account without cloudbroker rights.
type dynamixAPIClient struct {
	controllerURL string
	oAuth2URL     string
	appID         string
	appSecret     string
	httpClient    *http.Client
	token         string
}

func newDynamixAPIClient(provider dynamixProvider) (*dynamixAPIClient, error) {
	controllerURL, err := validateDynamixURL(provider.ControllerURL, "provider.controllerUrl")
	if err != nil {
		return nil, err
	}
	oAuth2URL, err := validateDynamixURL(provider.OAuth2URL, "provider.oAuth2Url")
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if provider.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the user asked for it with .provider.insecure
	}

	return &dynamixAPIClient{
		controllerURL: controllerURL,
		oAuth2URL:     oAuth2URL,
		appID:         provider.AppID,
		appSecret:     provider.AppSecret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   dynamixAPIRequestTimeout,
		},
	}, nil
}

func validateDynamixURL(rawURL, field string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("malformed provider cluster configuration: reading .%s: no such property", field)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("malformed provider cluster configuration: .%s is not a valid URL: %w", field, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("malformed provider cluster configuration: .%s must be an absolute URL, got %q", field, rawURL)
	}
	return strings.TrimSuffix(rawURL, "/"), nil
}

// accountIDByName resolves the account the cluster is created in.
func (c *dynamixAPIClient) accountIDByName(ctx context.Context, name string) (uint64, error) {
	body, err := c.call(ctx, http.MethodPost, dynamixAccountListPath, url.Values{"name": []string{name}})
	if err != nil {
		return 0, fmt.Errorf("cannot list Dynamix accounts: %w", err)
	}

	var list dynamixAccountList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, fmt.Errorf("cannot parse the Dynamix account list response: %w", err)
	}

	matches := dynamixExactMatches(list.Data, name, func(item dynamixAccount) string { return item.Name })

	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("account %q from .account does not exist or is not available to the configured credentials", name)
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("account name %q from .account matches %d accounts, it does not identify one", name, len(matches))
	}
}

// listStoragePolicies returns the policies matching name. A zero accountID
// leaves the account filter out, asking for everything the credentials can see.
func (c *dynamixAPIClient) listStoragePolicies(ctx context.Context, name string, accountID uint64) ([]dynamixStoragePolicy, error) {
	params := url.Values{"name": []string{name}}
	if accountID != 0 {
		params.Set("account_id", strconv.FormatUint(accountID, 10))
	}

	body, err := c.call(ctx, http.MethodGet, dynamixStoragePolicyListPath, params)
	if err != nil {
		return nil, err
	}

	var list dynamixStoragePolicyList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("cannot parse the Dynamix storage policy list response: %w", err)
	}

	return list.Data, nil
}

// call performs one cloudapi request. The platform takes its parameters
// form-encoded in the request body even for GET, which is what the platform SDK
// and the terraform provider both send, so that is the shape used here.
func (c *dynamixAPIClient) call(ctx context.Context, method, path string, params url.Values) ([]byte, error) {
	if err := c.authenticate(ctx); err != nil {
		return nil, err
	}

	requestURL := c.controllerURL + dynamixRESTPrefix + path
	header := http.Header{}
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	header.Set("Accept", "application/json")
	header.Set("Authorization", "bearer "+c.token)

	status, body, err := c.do(ctx, method, requestURL, params.Encode(), header)
	if err != nil {
		return nil, err
	}

	switch {
	case status == http.StatusOK || status == http.StatusNoContent:
		return body, nil
	case path == dynamixStoragePolicyListPath && dynamixRouteIsMissing(status):
		return nil, ErrDynamixPlatformTooOld
	default:
		return nil, dynamixHTTPError(requestURL, status, body)
	}
}

// dynamixRouteIsMissing reports whether the answer means "this endpoint is not
// served here", which is how a platform older than 4.6 reports the storage
// policy API. A platform that does serve it never answers a documented GET this
// way, so widening past 404 costs nothing and covers a gateway that rewrites it.
func dynamixRouteIsMissing(status int) bool {
	return status == http.StatusNotFound ||
		status == http.StatusMethodNotAllowed ||
		status == http.StatusNotImplemented
}

func dynamixHTTPError(requestURL string, status int, body []byte) error {
	return fmt.Errorf("%s answered %d: %s", requestURL, status, strings.TrimSpace(string(body)))
}

// authenticate exchanges the application credentials for an access token. The
// token is fetched once per check run: a preflight run is far shorter than the
// token's lifetime, so there is nothing to refresh.
func (c *dynamixAPIClient) authenticate(ctx context.Context) error {
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

	tokenURL := c.oAuth2URL + dynamixAccessTokenPath
	status, body, err := c.do(ctx, http.MethodPost, tokenURL, form.Encode(), header)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("cannot get a Dynamix API token from %s: %d: %s", tokenURL, status, strings.TrimSpace(string(body)))
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		return fmt.Errorf("cannot get a Dynamix API token from %s: the response is empty", tokenURL)
	}

	c.token = token
	return nil
}

// do sends a request, repeating it while the failure still looks transient: a
// dropped connection or a 5xx. Anything the platform answers below 500 is its
// verdict, not a blip, and is handed back to the caller as is.
func (c *dynamixAPIClient) do(ctx context.Context, method, requestURL, body string, header http.Header) (int, []byte, error) {
	var lastErr error

	for attempt := 1; attempt <= dynamixAPIAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return 0, nil, fmt.Errorf("cannot reach %s: %w", requestURL, ctx.Err())
			case <-time.After(dynamixAPIRetryInterval):
			}
		}

		status, respBody, err := c.send(ctx, method, requestURL, body, header)
		switch {
		case err != nil:
			lastErr = err
		// A missing route is a verdict, not a blip, even when it arrives as a
		// 5xx: repeating the request cannot make the endpoint exist.
		case status >= http.StatusInternalServerError && !dynamixRouteIsMissing(status):
			lastErr = dynamixHTTPError(requestURL, status, respBody)
		default:
			return status, respBody, nil
		}
	}

	return 0, nil, lastErr
}

func (c *dynamixAPIClient) send(ctx context.Context, method, requestURL, body string, header http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, strings.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("cannot create a request to %s: %w", requestURL, err)
	}
	req.Header = header.Clone()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot reach %s: %w", requestURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot read the response from %s: %w", requestURL, err)
	}

	return resp.StatusCode, respBody, nil
}
