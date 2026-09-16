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
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const (
	testDynamixAccountName = "acc_user"
	testDynamixAccountID   = uint64(42)
)

// dynamixPolicyFixture is a storage policy as the platform holds it: the policy
// itself plus the accounts it is granted to.
type dynamixPolicyFixture struct {
	policy   dynamixStoragePolicy
	accounts []uint64
}

func enabledDynamixPolicy(name string, accounts ...uint64) dynamixPolicyFixture {
	return dynamixPolicyFixture{
		policy:   dynamixStoragePolicy{Name: name, Status: dynamixStoragePolicyStatusEnabled},
		accounts: accounts,
	}
}

// fakeDynamixPlatform answers the two cloudapi endpoints preflight uses, on the
// wire shape the platform SDK produces: a form-encoded body (even for GET) and a
// bearer token taken from the SSO endpoint.
type fakeDynamixPlatform struct {
	t *testing.T

	accounts []dynamixAccount
	policies []dynamixPolicyFixture

	// storagePolicyListStatus, when non-zero, replaces the status of every
	// storage_policy/list answer. 404 is how a pre-4.6 platform behaves.
	storagePolicyListStatus int
	// tokenStatus, when non-zero, replaces the status of the SSO answer.
	tokenStatus int
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
		accounts: []dynamixAccount{{ID: testDynamixAccountID, Name: testDynamixAccountName}},
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
	case dynamixAccessTokenPath:
		if f.tokenStatus != 0 {
			http.Error(w, "invalid client credentials", f.tokenStatus)
			return
		}
		_, _ = io.WriteString(w, "test-access-token")

	case dynamixRESTPrefix + dynamixAccountListPath:
		if !f.authorized(w, r) {
			return
		}
		f.writeJSON(w, dynamixAccountList{Data: filterDynamixAccounts(f.accounts, params.Get("name"))})

	case dynamixRESTPrefix + dynamixStoragePolicyListPath:
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
		f.writeJSON(w, dynamixStoragePolicyList{Data: policies})

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

func (f *fakeDynamixPlatform) writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
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
		if path == dynamixAccessTokenPath {
			continue
		}
		assert.True(f.t, strings.HasPrefix(path, dynamixRESTPrefix+"/cloudapi/"),
			"preflight must not call anything outside cloudapi, called %s", path)
	}
}

func (f *fakeDynamixPlatform) assertNoHandlerErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(f.t, f.handlerErrors, "the fake platform rejected a request")
}

func filterDynamixAccounts(accounts []dynamixAccount, name string) []dynamixAccount {
	result := make([]dynamixAccount, 0, len(accounts))
	for _, account := range accounts {
		if name == "" || strings.Contains(account.Name, name) {
			result = append(result, account)
		}
	}
	return result
}

func filterDynamixPolicies(policies []dynamixPolicyFixture, params url.Values) ([]dynamixStoragePolicy, error) {
	var accountID uint64
	if raw := params.Get("account_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		accountID = parsed
	}
	name := params.Get("name")

	result := make([]dynamixStoragePolicy, 0, len(policies))
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
	check := DynamixStoragePolicyCheck{
		InstallConfig: &config.DeckhouseInstaller{ProviderClusterConfig: pcc},
	}
	return check.Run(t.Context())
}

// isPermanent reports whether the preflight framework will stop retrying on this
// error, which is how the check says "this is the platform's verdict".
func isPermanent(err error) bool {
	var permanent *backoff.PermanentError
	return errors.As(err, &permanent)
}

func TestDynamixStoragePolicyCheck(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		require.NoError(t, runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01")))
	})

	t.Run("platform older than 4.6 has no storage policy API", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.storagePolicyListStatus = http.StatusNotFound
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, ErrDynamixPlatformTooOld)
		assert.ErrorContains(t, err, "Basis Dynamix 4.6 or newer is required")
	})

	t.Run("policy does not exist", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "missing_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		assert.ErrorContains(t, err, `storage policy "missing_policy" (.storagePolicy) does not exist`)
	})

	t.Run("policy is not enabled", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, dynamixPolicyFixture{
			policy:   dynamixStoragePolicy{Name: "storage_policy01", Status: "DISABLED"},
			accounts: []uint64{testDynamixAccountID},
		})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `storage policy "storage_policy01" (.storagePolicy) is in status "DISABLED", expected "ENABLED"`)
	})

	t.Run("policy is not available to the account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID+1))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `storage policy "storage_policy01" (.storagePolicy) is not available to account "acc_user"`)
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
			policy:   dynamixStoragePolicy{Name: "storage_policy01", Status: "DISABLED"},
			accounts: []uint64{testDynamixAccountID + 1},
		})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `is in status "DISABLED"`)
		assert.NotContains(t, err.Error(), "is not available to account")
	})

	// Retrying is the preflight framework's job; the check's job is to say which
	// failures are worth retrying at all.
	t.Run("a platform hiccup asks to be retried", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.transientFailures = 1
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, "answered 503")
		assert.False(t, isPermanent(err), "a 5xx must not stop the framework from retrying")
	})

	t.Run("an unreachable platform asks to be retried", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()
		serverURL := server.URL
		server.Close()

		err := runDynamixCheck(t, dynamixPCC(serverURL, "storage_policy01"))

		assert.False(t, isPermanent(err), "a dropped connection must not stop the framework from retrying")
	})

	// Everything that is the platform's answer rather than a blip must stop the
	// retrying: repeating the request cannot change a missing policy into a
	// present one, and five backed-off attempts would only delay the report.
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
				assert.True(t, isPermanent(err), "the framework must not retry a verdict")
			})
		}
	})

	t.Run("policy name matches more than one policy", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t,
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
			enabledDynamixPolicy("storage_policy01", testDynamixAccountID),
		)
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, errDynamixPolicyAmbiguous)
	})

	t.Run("node group instance class override is checked too", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01", "worker_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		assert.ErrorContains(t, err, `storage policy "worker_policy" (.nodeGroups[0].instanceClass.storagePolicy)`)
	})

	t.Run("account does not exist", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = nil
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `account "acc_user" from .account does not exist`)
	})

	t.Run("account name matches more than one account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = append(platform.accounts, dynamixAccount{ID: 43, Name: testDynamixAccountName})
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `matches 2 accounts`)
	})

	t.Run("account lookup is exact, a prefix match is not the account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.accounts = []dynamixAccount{{ID: 43, Name: testDynamixAccountName + "_staging"}}
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, `account "acc_user" from .account does not exist`)
	})

	t.Run("credentials are rejected by SSO", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.tokenStatus = http.StatusUnauthorized
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		assert.ErrorContains(t, err, "cannot get a Dynamix API token")
	})

	t.Run("platform is unreachable", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()
		serverURL := server.URL
		server.Close()

		err := runDynamixCheck(t, dynamixPCC(serverURL, "storage_policy01"))

		assert.ErrorContains(t, err, "cannot reach")
		assert.ErrorContains(t, err, dynamixAccessTokenPath)
	})

	t.Run("master instance class override is checked too", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCCWithMasterPolicy(server.URL, "storage_policy01", "master_policy"))

		require.ErrorIs(t, err, errDynamixPolicyNotFound)
		assert.ErrorContains(t, err, `storage policy "master_policy" (.masterNodeGroup.instanceClass.storagePolicy)`)
	})

	t.Run("the platform is asked something", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		server := platform.start()

		require.NoError(t, runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01")))

		// Guards the cloudapi-only invariant asserted for every scenario in
		// start's cleanup: it would pass vacuously on a check that called nothing.
		require.NotEmpty(t, platform.paths())
	})

	// A pre-4.6 platform must be named as such even when something else about
	// the configuration is wrong too — the version is what has to be fixed first.
	t.Run("the version verdict is not masked by a bad account", func(t *testing.T) {
		platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
		platform.storagePolicyListStatus = http.StatusNotFound
		platform.accounts = nil
		server := platform.start()

		err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

		require.ErrorIs(t, err, ErrDynamixPlatformTooOld)
	})

	// Whatever a gateway in front of the platform turns a missing route into,
	// the verdict is the same: the endpoint is not served here.
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented} {
		t.Run(fmt.Sprintf("a missing storage policy route answering %d reads as pre-4.6", status), func(t *testing.T) {
			platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("storage_policy01", testDynamixAccountID))
			platform.storagePolicyListStatus = status
			server := platform.start()

			err := runDynamixCheck(t, dynamixPCC(server.URL, "storage_policy01"))

			require.ErrorIs(t, err, ErrDynamixPlatformTooOld)
		})
	}
}

func TestDynamixStoragePolicyCheckMalformedConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(pcc string) string
		expectedError string
	}{
		{
			name:          "no storagePolicy",
			mutate:        func(pcc string) string { return strings.ReplaceAll(pcc, "storagePolicy: p", "stub: p") },
			expectedError: "reading .storagePolicy: no such property",
		},
		{
			name:          "no account",
			mutate:        func(pcc string) string { return strings.ReplaceAll(pcc, "account: acc_user", "stub: acc_user") },
			expectedError: "reading .account: no such property",
		},
		{
			name: "controllerUrl is not absolute",
			mutate: func(pcc string) string {
				return strings.ReplaceAll(pcc, `controllerUrl: "http`, `controllerUrl: "controller//http`)
			},
			expectedError: ".provider.controllerUrl must be an absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := newFakeDynamixPlatform(t, enabledDynamixPolicy("p", testDynamixAccountID))
			server := platform.start()

			err := runDynamixCheck(t, []byte(tt.mutate(string(dynamixPCC(server.URL, "p")))))

			assert.ErrorContains(t, err, tt.expectedError)
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

			assert.NoError(t, check.Run(t.Context()))
		})
	}
}

func TestDynamixStoragePolicyCheckIsRegistered(t *testing.T) {
	check := DynamixStoragePolicy(nil)

	assert.Equal(t, DynamixStoragePolicyCheckName, check.Name)
	require.NoError(t, check.Name.Validate())
	assert.Equal(t, preflightnew.PhasePreInfra, check.Phase)
	assert.Equal(t, preflightnew.DefaultRetryPolicy, check.Retry)
	assert.NotEmpty(t, check.Description)
}
