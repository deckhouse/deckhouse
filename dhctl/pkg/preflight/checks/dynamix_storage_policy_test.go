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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dynamix"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const (
	testDynamixAccountName = "acc_user"
	testDynamixAccountID   = uint64(42)
)

// The wire paths the fake platform serves. They are spelled out rather than
// taken from the client, so that a client which drifts off them fails here.
const (
	testDynamixTokenPath             = "/v1/oauth/access_token"
	testDynamixCloudAPIPrefix        = "/restmachine/cloudapi/"
	testDynamixAccountListPath       = "/restmachine/cloudapi/account/list"
	testDynamixStoragePolicyListPath = "/restmachine/cloudapi/storage_policy/list"
)

// dynamixPolicyFixture is a storage policy as the platform holds it: the policy
// itself plus the accounts it is granted to.
type dynamixPolicyFixture struct {
	policy   dynamix.StoragePolicy
	accounts []uint64
}

func enabledDynamixPolicy(name string, accounts ...uint64) dynamixPolicyFixture {
	return dynamixPolicyFixture{
		policy:   dynamix.StoragePolicy{Name: name, Status: dynamix.StoragePolicyStatusEnabled},
		accounts: accounts,
	}
}

// fakeDynamixPlatform answers the two cloudapi endpoints preflight uses, on the
// wire shape the platform SDK produces: a form-encoded body (even for GET) and a
// bearer token taken from the SSO endpoint.
type fakeDynamixPlatform struct {
	t *testing.T

	accounts []dynamix.Account
	policies []dynamixPolicyFixture

	// storagePolicyListStatus, when non-zero, replaces the status of every
	// storage_policy/list answer. 404 is how a pre-4.6 platform behaves.
	storagePolicyListStatus int
	// tokenStatus, when non-zero, replaces the status of the SSO answer.
	tokenStatus int
	// accountListStatus, when non-zero, replaces the status of every
	// account/list answer: 403 is an application without rights to the API.
	accountListStatus int
	// transientFailures makes the first N storage_policy/list answers a 503,
	// the shape of a platform hiccup the check must hand back as retryable.
	transientFailures int

	// mu guards what the handler goroutines record for the test goroutine to
	// read once the check has returned.
	mu             sync.Mutex
	requestedPaths []string
	handlerErrors  []string
}

func newFakeDynamixPlatform(t *testing.T, policies ...dynamixPolicyFixture) *fakeDynamixPlatform {
	return &fakeDynamixPlatform{
		t:        t,
		accounts: []dynamix.Account{{ID: testDynamixAccountID, Name: testDynamixAccountName}},
		policies: policies,
	}
}

func (f *fakeDynamixPlatform) start() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	f.t.Cleanup(func() {
		server.Close()
		f.assertNoHandlerErrors()
		f.assertOnlyCloudAPIWasUsed()
	})
	return server
}

func (f *fakeDynamixPlatform) serve(w http.ResponseWriter, r *http.Request) {
	f.record(r.URL.Path)

	params, err := dynamixFormValues(r)
	if err != nil {
		f.fail(w, "cannot read the request body: %v", err)
		return
	}

	switch r.URL.Path {
	case testDynamixTokenPath:
		if f.tokenStatus != 0 {
			http.Error(w, "invalid client credentials", f.tokenStatus)
			return
		}
		_, _ = io.WriteString(w, "test-access-token")

	case testDynamixAccountListPath:
		if !f.authorized(w, r) {
			return
		}
		if f.accountListStatus != 0 {
			http.Error(w, "access denied", f.accountListStatus)
			return
		}
		f.writeData(w, filterDynamixAccounts(f.accounts, params.Get("name")))

	case testDynamixStoragePolicyListPath:
		if !f.authorized(w, r) {
			return
		}
		if f.storagePolicyListStatus != 0 {
			http.Error(w, "404 Not Found", f.storagePolicyListStatus)
			return
		}
		if f.takeTransientFailure() {
			http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		policies, err := filterDynamixPolicies(f.policies, params)
		if err != nil {
			f.fail(w, "cannot filter storage policies: %v", err)
			return
		}
		f.writeData(w, policies)

	default:
		f.fail(w, "unexpected request to %s", r.URL.Path)
	}
}

// formValues reads the parameters out of the request body: the platform takes
// them form-encoded there even for a GET, so http.Request.ParseForm would not
// see them.
func dynamixFormValues(r *http.Request) (url.Values, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(string(body))
}

func (f *fakeDynamixPlatform) authorized(w http.ResponseWriter, r *http.Request) bool {
	if token := r.Header.Get("Authorization"); token != "bearer test-access-token" {
		f.fail(w, "request to %s carries %q instead of the issued bearer token", r.URL.Path, token)
		return false
	}
	return true
}

// writeData answers with a list the way cloudapi wraps one: under "data".
func (f *fakeDynamixPlatform) writeData(w http.ResponseWriter, items any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": items}); err != nil {
		f.record("encode error: " + err.Error())
	}
}

// takeTransientFailure reports whether this answer is one of the hiccups the
// fixture was told to serve, and spends it.
func (f *fakeDynamixPlatform) takeTransientFailure() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.transientFailures == 0 {
		return false
	}
	f.transientFailures--
	return true
}

func (f *fakeDynamixPlatform) record(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requestedPaths = append(f.requestedPaths, path)
}

// fail records what went wrong instead of calling t.Fatal: the handler runs on
// the server's goroutine, where testify's require cannot fail the test cleanly.
// assertNoHandlerErrors, deferred by start, reports it on the test goroutine.
func (f *fakeDynamixPlatform) fail(w http.ResponseWriter, format string, args ...any) {
	f.mu.Lock()
	f.handlerErrors = append(f.handlerErrors, fmt.Sprintf(format, args...))
	f.mu.Unlock()

	http.Error(w, "test fixture error", http.StatusInternalServerError)
}

// count is how many times path was asked for.
func (f *fakeDynamixPlatform) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, requested := range f.requestedPaths {
		if requested == path {
			n++
		}
	}
	return n
}

func (f *fakeDynamixPlatform) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requestedPaths...)
}

// assertOnlyCloudAPIWasUsed holds the policy-first model's other half: with SEPs
// out of the contract, preflight has no reason to reach past cloudapi, so an
// account without cloudbroker rights must get through. Checked after every
// scenario rather than in one test, so no error path can quietly grow a call.
func (f *fakeDynamixPlatform) assertOnlyCloudAPIWasUsed() {
	for _, path := range f.paths() {
		if path == testDynamixTokenPath {
			continue
		}
		assert.True(f.t, strings.HasPrefix(path, testDynamixCloudAPIPrefix),
			"preflight must not call anything outside cloudapi, called %s", path)
	}
}

func (f *fakeDynamixPlatform) assertNoHandlerErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(f.t, f.handlerErrors, "the fake platform rejected a request")
}

func filterDynamixAccounts(accounts []dynamix.Account, name string) []dynamix.Account {
	result := make([]dynamix.Account, 0, len(accounts))
	for _, account := range accounts {
		if name == "" || strings.Contains(account.Name, name) {
			result = append(result, account)
		}
	}
	return result
}

func filterDynamixPolicies(policies []dynamixPolicyFixture, params url.Values) ([]dynamix.StoragePolicy, error) {
	var accountID uint64
	if raw := params.Get("account_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		accountID = parsed
	}
	name := params.Get("name")

	result := make([]dynamix.StoragePolicy, 0, len(policies))
	for _, fixture := range policies {
		if name != "" && !strings.Contains(fixture.policy.Name, name) {
			continue
		}
		if accountID != 0 && !slices.Contains(fixture.accounts, accountID) {
			continue
		}
		result = append(result, fixture.policy)
	}
	return result, nil
}

// dynamixPCC renders a DynamixClusterConfiguration pointed at the fake platform.
// Master sizing is deliberately above the requirements so that only the storage
// policy checks can fail.
func dynamixPCC(serverURL, clusterPolicy string, nodeGroupPolicies ...string) []byte {
	return dynamixPCCWithMasterPolicy(serverURL, clusterPolicy, "", nodeGroupPolicies...)
}

// dynamixPCCWithMasterPolicy additionally overrides the policy on the master
// instance class, which the cluster-wide field does not cover.
func dynamixPCCWithMasterPolicy(serverURL, clusterPolicy, masterPolicy string, nodeGroupPolicies ...string) []byte {
	var builder strings.Builder
	fmt.Fprintf(&builder, `
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: %s
storagePolicy: %s
nodeNetworkCIDR: "10.241.32.0/24"
provider:
  controllerUrl: %q
  oAuth2Url: %q
  appId: app-id
  appSecret: app-secret
  insecure: false
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: image
    externalNetwork: extnet
`, testDynamixAccountName, clusterPolicy, serverURL, serverURL)

	if masterPolicy != "" {
		fmt.Fprintf(&builder, "    storagePolicy: %s\n", masterPolicy)
	}

	if len(nodeGroupPolicies) > 0 {
		builder.WriteString("nodeGroups:\n")
		for i, policy := range nodeGroupPolicies {
			fmt.Fprintf(&builder, `- name: worker%d
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: image
    externalNetwork: extnet
    storagePolicy: %s
`, i, policy)
		}
	}

	return []byte(builder.String())
}

func runDynamixCheck(t *testing.T, pcc []byte) error {
	t.Helper()
	_, err := runDynamixCheckObserved(t, pcc)
	return err
}

func runDynamixCheckObserved(t *testing.T, pcc []byte) (string, error) {
	t.Helper()
	check := DynamixStoragePolicyCheck{
		InstallConfig: &config.DeckhouseInstaller{ProviderClusterConfig: pcc},
	}
	return check.Run(t.Context())
}

// requireDynamixFailure asserts the check named what it looked at, rather than
// handing the runner a bare error to print under `reason:`.
func requireDynamixFailure(t *testing.T, err error) *preflight.Failure {
	t.Helper()
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	return failure
}

// dynamixIsPermanent reports whether the preflight framework will stop retrying
// on this error, which is how the check says "this is the platform's verdict".
func dynamixIsPermanent(err error) bool {
	var permanent *backoff.PermanentError
	return errors.As(err, &permanent)
}

func TestDynamixStoragePolicyCheck(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		observed, err := runDynamixCheckObserved(t, dynamixPCC(server.URL, "storage_policy01"))

		require.NoError(t, err)
		assert.Equal(t, `storage policy "storage_policy01" is ENABLED and available to account "acc_user"`, observed)
	})

	t.Run("every policy checked is named in the pass", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t,
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
			enabledDynamixPolicy("worker_policy", testDynamixAccountID),
		)
		server := platform.start()

		observed, err := runDynamixCheckObserved(t, dynamixPCC(server.URL, "storage_policy01", "worker_policy"))

		require.NoError(t, err)
		assert.Equal(t, `storage policies "storage_policy01", "worker_policy" are ENABLED and available to account "acc_user"`, observed)
	})

	t.Run("platform older than 4.6 has no storage policy API", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.storagePolicyListStatus = http.StatusNotFound
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, dynamix.ErrStoragePolicyAPIMissing)
		failure := requireDynamixFailure(t, err)
		assert.Contains(t, failure.Expected, "Basis Dynamix 4.6 or newer")
		assert.Contains(t, failure.Fix, "DynamixClusterConfiguration.provider.controllerUrl")
	})

	t.Run("policy does not exist", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "missing_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		failure := requireDynamixFailure(t, err)
		assert.Equal(t, "DynamixClusterConfiguration.storagePolicy", failure.Checked)
		assert.Equal(t, `storage policy "missing_policy" does not exist`, failure.Observed)
		assert.Contains(t, failure.Fix, "DynamixClusterConfiguration.storagePolicy")
	})

	t.Run("policy is not enabled", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, dynamixPolicyFixture{
			policy:   dynamix.StoragePolicy{Name: "storage_policy01", Status: "DISABLED"},
			accounts: []uint64{testDynamixAccountID},
		})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyNotEnabled)
		failure := requireDynamixFailure(t, err)
		assert.Equal(t, "DynamixClusterConfiguration.storagePolicy", failure.Checked)
		assert.Equal(t, `storage policy "storage_policy01" is in status "DISABLED"`, failure.Observed)
		assert.Contains(t, failure.Expected, `status "ENABLED"`)
	})

	t.Run("policy is not available to the account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID+1))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyNotAvailable)
		failure := requireDynamixFailure(t, err)
		assert.Equal(t, `storage policy "storage_policy01" is not available to account "acc_user"`, failure.Observed)
		assert.Contains(t, failure.Fix, `grant account "acc_user" access to the policy`)
	})

	// The list request's name filter narrows the answer on the platform side but
	// is not documented to match exactly, so the decision is made client-side.
	t.Run("policy lookup is exact, a prefix match is not the policy", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01_backup", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
	})

	// A policy the account cannot see may also be disabled. The status is the
	// more actionable half of that, so it must not be masked by the access error.
	t.Run("a policy that is both disabled and not granted reports its status", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, dynamixPolicyFixture{
			policy:   dynamix.StoragePolicy{Name: "storage_policy01", Status: "DISABLED"},
			accounts: []uint64{testDynamixAccountID + 1},
		})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyNotEnabled)
		assert.NotErrorIs(t, err, errDynamixPolicyNotAvailable)
	})

	// Retrying is the preflight framework's job; the check's job is to say which
	// failures are worth retrying at all.
	t.Run("a platform hiccup asks to be retried", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.transientFailures = 1
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		failure := requireDynamixFailure(t, err)
		assert.Equal(t, "the platform did not answer", failure.Observed)
		assert.ErrorContains(t, failure.Err, "answered 503")
		assert.False(t, dynamixIsPermanent(err), "a 5xx must not stop the framework from retrying")
	})

	t.Run("an unreachable platform asks to be retried", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()
		serverURL := server.URL
		server.Close()

		err := runDynamixCheck(t, dynamixPCC(serverURL, "storage_policy01"))

		assert.False(t, dynamixIsPermanent(err), "a dropped connection must not stop the framework from retrying")
	})

	// Everything that is the platform's answer rather than a blip must stop the
	// retrying: repeating the request cannot change a missing policy into a
	// present one, and the backed-off attempts would only delay the report.
	t.Run("a verdict stops the retrying", func(t *testing.T) {
		tests := []struct {
			name    string
			arrange func(*fakeDynamixPlatform)
			policy  string
		}{
			{
				name:    "the platform is older than 4.6",
				arrange: func(f *fakeDynamixPlatform) { f.storagePolicyListStatus = http.StatusNotFound },
				policy:  "storage_policy01",
			},
			{
				name:   "the policy does not exist",
				policy: "missing_policy",
			},
			{
				name:    "the credentials are rejected",
				arrange: func(f *fakeDynamixPlatform) { f.tokenStatus = http.StatusUnauthorized },
				policy:  "storage_policy01",
			},
			{
				name:    "the account does not exist",
				arrange: func(f *fakeDynamixPlatform) { f.accounts = nil },
				policy:  "storage_policy01",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
				if tt.arrange != nil {
					tt.arrange(platform)
				}
				server := platform.start()

				err := runDynamixCheck(t, dynamixPCC(server.URL, tt.policy))

				require.Error(t, err)
				assert.True(t, dynamixIsPermanent(err), "the framework must not retry a verdict")
			})
		}
	})

	// dynamix-common resolves the name inside the account, so a namesake in
	// somebody else's account resolves fine at runtime. Preflight refusing it
	// would block a bootstrap the platform would have completed.
	t.Run("a namesake in another account is not ambiguity", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t,
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID+1),
		)
		server := platform.start()

		require.NoError(t, runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01")))
	})

	t.Run("policy name matches more than one policy in the account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t,
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
		)
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyAmbiguous)
		failure := requireDynamixFailure(t, err)
		assert.Contains(t, failure.Observed, `matches 2 policies of account "acc_user"`)
	})

	t.Run("node group instance class override is checked too", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01", "worker_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		assert.Equal(t, "DynamixClusterConfiguration.nodeGroups[0].instanceClass.storagePolicy", requireDynamixFailure(t, err).Checked)
	})

	t.Run("account does not exist", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = nil
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		failure := requireDynamixFailure(t, err)
		assert.Equal(t, "DynamixClusterConfiguration.account", failure.Checked)
		assert.Equal(t, `account "acc_user" does not exist or is not available to the configured credentials`, failure.Observed)
	})

	t.Run("account name matches more than one account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = append(platform.accounts, dynamix.Account{ID: 43, Name: testDynamixAccountName})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.Contains(t, requireDynamixFailure(t, err).Observed, `matches 2 accounts`)
	})

	t.Run("account lookup is exact, a prefix match is not the account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = []dynamix.Account{{ID: 43, Name: testDynamixAccountName + "_staging"}}
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.Contains(t, requireDynamixFailure(t, err).Observed, `account "acc_user" does not exist`)
	})

	t.Run("a request the platform refuses is a verdict with its answer in the details", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accountListStatus = http.StatusForbidden
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.True(t, dynamixIsPermanent(err), "the framework must not retry a refusal")
		failure := requireDynamixFailure(t, err)
		assert.Equal(t, "the platform refused the request", failure.Observed)
		assert.ErrorContains(t, failure.Err, "answered 403: access denied")
	})

	t.Run("credentials are rejected by SSO", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.tokenStatus = http.StatusUnauthorized
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, dynamix.ErrCredentialsRefused)
		failure := requireDynamixFailure(t, err)
		assert.Contains(t, failure.Fix, "DynamixClusterConfiguration.provider.appSecret")
		assert.ErrorContains(t, failure.Err, "get a Dynamix API token")
	})

	t.Run("platform is unreachable", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()
		serverURL := server.URL
		server.Close()

		err := runDynamixCheck(t, dynamixPCC(serverURL, "storage_policy01"))

		failure := requireDynamixFailure(t, err)
		assert.ErrorContains(t, failure.Err, "reach")
		assert.ErrorContains(t, failure.Err, testDynamixTokenPath)
	})

	// Every reader of the field has to treat an empty override the same way, or a
	// configuration passes preflight and then fails where it is read differently: the
	// CAPI template falls back to the cluster-wide value, and so do the terraform
	// modules since they stopped reading the field with a nested lookup().
	t.Run("an empty master instance class override reads as the cluster-wide policy", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		observed, err := runDynamixCheckObserved(t, dynamixPCCWithMasterPolicy(server.URL, "storage_policy01", `""`))

		require.NoError(t, err)
		assert.Equal(t, `storage policy "storage_policy01" is ENABLED and available to account "acc_user"`, observed)
	})

	t.Run("master instance class override is checked too", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCCWithMasterPolicy(server.URL, "storage_policy01", "master_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		assert.Equal(t, "DynamixClusterConfiguration.masterNodeGroup.instanceClass.storagePolicy", requireDynamixFailure(t, err).Checked)
	})

	t.Run("the platform is asked something", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		require.NoError(t, runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01")))

		// Guards the cloudapi-only invariant asserted for every scenario in
		// start's cleanup: it would pass vacuously on a check that called nothing.
		require.NotEmpty(t, platform.paths())
	})

	// The version question and the "is it somebody else's" question about the
	// first policy are the same request; it is sent once.
	t.Run("the version answer explains a policy the account cannot see", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", 7))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyNotAvailable)
		// One without the account (the version question, reused), one with it.
		assert.Equal(t, 2, platform.count(testDynamixStoragePolicyListPath))
	})

	// A pre-4.6 platform must be named as such even when something else about
	// the configuration is wrong too — the version is what has to be fixed first.
	t.Run("the version verdict is not masked by a bad account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.storagePolicyListStatus = http.StatusNotFound
		platform.accounts = nil
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, dynamix.ErrStoragePolicyAPIMissing)
	})

	// Whatever a gateway in front of the platform turns a missing route into,
	// the verdict is the same: the endpoint is not served here.
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented} {
		t.Run(fmt.Sprintf("a missing storage policy route answering %d reads as pre-4.6", status), func(t *testing.T) {
			platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
			platform.storagePolicyListStatus = status
			server := platform.start()

			err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

			require.ErrorIs(t, err, dynamix.ErrStoragePolicyAPIMissing)
		})
	}
}

func TestDynamixStoragePolicyCheckMalformedConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		mutate          func(pcc string) string
		expectedChecked string
		expectedError   string
	}{
		{
			name:            "no storagePolicy",
			mutate:          func(pcc string) string { return strings.ReplaceAll(pcc, "storagePolicy: p", "stub: p") },
			expectedChecked: "DynamixClusterConfiguration.storagePolicy",
			expectedError:   "reading .storagePolicy: no such property",
		},
		{
			// An empty value is not a missing key to every reader of the field, so it is
			// refused here as well: the OpenAPI schema keeps it out of a configuration
			// that came through `dhctl config parse`, this keeps it out of the rest.
			name:            "empty storagePolicy",
			mutate:          func(pcc string) string { return strings.ReplaceAll(pcc, "storagePolicy: p", `storagePolicy: ""`) },
			expectedChecked: "DynamixClusterConfiguration.storagePolicy",
			expectedError:   "reading .storagePolicy: no such property",
		},
		{
			name:            "no account",
			mutate:          func(pcc string) string { return strings.ReplaceAll(pcc, "account: acc_user", "stub: acc_user") },
			expectedChecked: "DynamixClusterConfiguration.account",
			expectedError:   "reading .account: no such property",
		},
		{
			name: "controllerUrl is not absolute",
			mutate: func(pcc string) string {
				return strings.ReplaceAll(pcc, `controllerUrl: "http`, `controllerUrl: "controller//http`)
			},
			expectedChecked: "DynamixClusterConfiguration.provider.controllerUrl",
			expectedError:   ".provider.controllerUrl must be an absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("p", testDynamixAccountID))
			server := platform.start()

			err := runDynamixCheck(t, []byte(tt.mutate(string(dynamixPCC(server.URL, "p")))))

			assert.True(t, dynamixIsPermanent(err), "a configuration error is a verdict")
			failure := requireDynamixFailure(t, err)
			assert.Equal(t, tt.expectedChecked, failure.Checked)
			assert.ErrorContains(t, failure.Err, tt.expectedError)
			assert.Empty(t, platform.paths(), "a malformed configuration is refused before the platform is asked")
		})
	}
}

// The check now stands on its own in the cloud suite, so it has to recognise
// what it is looking at and stay out of the way of everything else.
func TestDynamixStoragePolicyCheckSkipsWhatIsNotItsOwn(t *testing.T) {
	tests := []struct {
		name          string
		installConfig *config.DeckhouseInstaller
	}{
		{
			name:          "no install config",
			installConfig: nil,
		},
		{
			// The ModuleConfig-only flow carries no provider cluster
			// configuration at all.
			name:          "no provider cluster configuration",
			installConfig: &config.DeckhouseInstaller{ProviderClusterConfig: nil},
		},
		{
			name:          "another provider",
			installConfig: &config.DeckhouseInstaller{ProviderClusterConfig: validPCC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := DynamixStoragePolicyCheck{InstallConfig: tt.installConfig}

			// Not a pass: the check asserts nothing about a cluster that is not on Dynamix.
			_, err := check.Run(t.Context())
			assert.ErrorIs(t, err, preflight.ErrNotApplicable)
		})
	}
}

func TestDynamixStoragePolicyCheckIsRegistered(t *testing.T) {
	check := DynamixStoragePolicy(nil)

	assert.Equal(t, DynamixStoragePolicyCheckName, check.Name)
	require.NoError(t, check.Name.Validate())
	assert.Equal(t, preflight.PhasePreInfra, check.Phase)
	assert.Equal(t, preflight.NetworkRetry, check.Retry)
	assert.Equal(t, preflight.LongCheckTimeout, check.Timeout)
	assert.NotEmpty(t, check.Description)
}
