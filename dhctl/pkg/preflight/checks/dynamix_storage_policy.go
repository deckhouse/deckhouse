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
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dynamix"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// dynamixClusterConfigurationKind is the only provider cluster configuration
// this check has anything to say about.
const dynamixClusterConfigurationKind = "DynamixClusterConfiguration"

const DynamixStoragePolicyCheckName preflight.CheckName = "dynamix-storage-policy"

// DynamixStoragePolicyCheck asks the Dynamix platform, before any infrastructure
// exists, whether it can finish the bootstrap at all: Basis Dynamix 4.6 is the
// first version with storage policies, and every disk and machine of the cluster
// is placed in one.
type DynamixStoragePolicyCheck struct {
	InstallConfig *config.DeckhouseInstaller
}

func (DynamixStoragePolicyCheck) Description() string {
	return "the Dynamix platform supports storage policies and the configured policy is usable"
}

func (DynamixStoragePolicyCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

// RetryPolicy is NetworkRetry because every failure this check reports is
// either a verdict or a blip, and it says which: a verdict comes back wrapped in
// preflight.Permanent, so only the blips are ever tried again.
func (DynamixStoragePolicyCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c DynamixStoragePolicyCheck) Run(ctx context.Context) (string, error) {
	observed, err := c.run(ctx)
	if err == nil || dynamix.IsTransient(err) {
		return observed, err
	}
	return "", preflight.Permanent(err)
}

func (c DynamixStoragePolicyCheck) run(ctx context.Context) (string, error) {
	// Nothing to ask about without a provider cluster configuration: in the
	// ModuleConfig-only flow the storage policy arrives as a DynamixInstanceClass
	// resource, which the provider's own validator binary checks.
	if c.InstallConfig == nil || len(c.InstallConfig.ProviderClusterConfig) == 0 {
		return "", preflight.NotApplicable("the input file has no DynamixClusterConfiguration to read the storage policy from")
	}

	clusterConfig, err := parseDynamixClusterConfiguration(c.InstallConfig.ProviderClusterConfig)
	if err != nil {
		return "", err
	}
	if clusterConfig.Kind != dynamixClusterConfigurationKind {
		return "", preflight.NotApplicable("the cluster is not on Dynamix, the provider cluster configuration is %s", clusterConfig.Kind)
	}
	if err := clusterConfig.validate(); err != nil {
		return "", err
	}

	client, err := dynamix.NewClient(clusterConfig.Provider)
	if err != nil {
		return "", dynamixConfigFailure(err)
	}

	policies := clusterConfig.storagePolicies()
	checker := newDynamixPolicyChecker(client, clusterConfig)

	// The version question is asked first and on its own, before the account is
	// even resolved: on a pre-4.6 platform this is the call that answers 404, and
	// the version is the one thing the user has to fix. Any other complaint at
	// this point — a mistyped account, say — would only distract from it. The
	// answer is not thrown away: it is what explains the first policy should the
	// account turn out not to see it.
	if _, err := checker.visiblePolicies(ctx, policies[0].name); err != nil {
		return "", dynamixAPIFailure(err, clusterConfig.Provider)
	}

	checker.accountID, err = resolveDynamixAccount(ctx, client, clusterConfig)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(policies))
	for _, policy := range policies {
		if err := checker.check(ctx, policy); err != nil {
			return "", err
		}
		names = append(names, fmt.Sprintf("%q", policy.name))
	}

	if len(names) == 1 {
		return fmt.Sprintf("storage policy %s is ENABLED and available to account %q", names[0], clusterConfig.Account), nil
	}
	return fmt.Sprintf("storage policies %s are ENABLED and available to account %q",
		strings.Join(names, ", "), clusterConfig.Account), nil
}

func DynamixStoragePolicy(installConfig *config.DeckhouseInstaller) preflight.Check {
	check := DynamixStoragePolicyCheck{InstallConfig: installConfig}
	return preflight.Check{
		Name:        DynamixStoragePolicyCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		// One attempt is not one round trip but a handful — a token, the version
		// question, the account, then each policy of the configuration, once more
		// for a policy the account cannot see — and the client gives each of them
		// up to 10 seconds. That is the budget of a check walking a list, as
		// CloudAPICheck does, not DefaultPreflightCheckTimeout's single question.
		Timeout: preflight.LongCheckTimeout,
		Run:     check.Run,
	}
}

var (
	errDynamixPolicyNotFound     = errors.New("storage policy not found")
	errDynamixPolicyAmbiguous    = errors.New("storage policy name is ambiguous")
	errDynamixPolicyNotEnabled   = errors.New("storage policy is not enabled")
	errDynamixPolicyNotAvailable = errors.New("storage policy is not available to the account")
)

// dynamixConfigFailure names the field of the provider section the client
// refused to work with.
func dynamixConfigFailure(err error) error {
	var configErr *dynamix.ConfigError
	if !errors.As(err, &configErr) {
		return err
	}
	checked := dynamixClusterConfigurationKind + "." + configErr.Field
	return &preflight.Failure{
		Checked:  checked,
		Observed: fmt.Sprintf("the field %s", configErr.Problem),
		Expected: "an absolute URL, such as https://controller.example.com",
		Fix:      fmt.Sprintf("set %s to the URL the Dynamix platform documentation gives for it", checked),
		Err:      err,
	}
}

// dynamixAPIFailure names what the API said when it did not answer the
// question at all. The client's error stays in Err: it is what keeps
// dynamix.IsTransient and errors.Is working through the Failure.
func dynamixAPIFailure(err error, provider dynamix.ProviderConfig) error {
	switch {
	case errors.Is(err, dynamix.ErrStoragePolicyAPIMissing):
		return &preflight.Failure{
			Checked:  fmt.Sprintf("the storage policy API at %s", provider.ControllerURL),
			Observed: "the platform does not provide the storage policy API",
			Expected: "Basis Dynamix 4.6 or newer, the first version with storage policies",
			Fix: "upgrade the platform to Basis Dynamix 4.6 or newer, or correct " +
				dynamixClusterConfigurationKind + ".provider.controllerUrl if it points at another controller",
			Err: err,
		}
	case errors.Is(err, dynamix.ErrCredentialsRefused):
		return &preflight.Failure{
			Checked:  fmt.Sprintf("a Dynamix API token from %s", provider.OAuth2URL),
			Observed: "the SSO endpoint refused the application credentials",
			Expected: "an access token for the configured application",
			Fix: fmt.Sprintf("correct %[1]s.provider.appId and %[1]s.provider.appSecret, or %[1]s.provider.oAuth2Url",
				dynamixClusterConfigurationKind),
			Err: err,
		}
	case dynamix.IsTransient(err):
		return &preflight.Failure{
			Checked:  fmt.Sprintf("the Dynamix cloud API at %s", provider.ControllerURL),
			Observed: "the platform did not answer",
			Expected: "an answer from the cloud API",
			Fix: fmt.Sprintf("check that the installer reaches %[1]s.provider.controllerUrl and %[1]s.provider.oAuth2Url",
				dynamixClusterConfigurationKind),
			Err: err,
		}
	default:
		return &preflight.Failure{
			Checked:  fmt.Sprintf("the Dynamix cloud API at %s", provider.ControllerURL),
			Observed: "the platform refused the request",
			Expected: "an answer from the cloud API",
			Fix:      "see the details for what the platform answered",
			Err:      err,
		}
	}
}

// resolveDynamixAccount resolves the account the cluster is created in. An
// ambiguous name is refused rather than resolved to the first match: that
// would silently bootstrap into somebody else's account.
func resolveDynamixAccount(ctx context.Context, client *dynamix.Client, clusterConfig dynamixClusterConfiguration) (uint64, error) {
	accounts, err := client.AccountsNamed(ctx, clusterConfig.Account)
	if err != nil {
		return 0, dynamixAPIFailure(err, clusterConfig.Provider)
	}

	checked := dynamixClusterConfigurationKind + ".account"
	switch len(accounts) {
	case 1:
		return accounts[0].ID, nil
	case 0:
		return 0, &preflight.Failure{
			Checked:  checked,
			Observed: fmt.Sprintf("account %q does not exist or is not available to the configured credentials", clusterConfig.Account),
			Expected: "exactly one account with that name",
			Fix:      fmt.Sprintf("set %s to the exact name of an account the application credentials have access to", checked),
		}
	default:
		return 0, &preflight.Failure{
			Checked:  checked,
			Observed: fmt.Sprintf("account name %q matches %d accounts, it does not identify one", clusterConfig.Account, len(accounts)),
			Expected: "exactly one account with that name",
			Fix:      fmt.Sprintf("set %s to a name that identifies one account", checked),
		}
	}
}

// dynamixPolicyChecker holds what checking one storage policy takes: the
// client, the configuration the errors name, the resolved account and the
// policies the credentials can see across accounts, asked for at most once per
// name — the version question already asked about the first policy.
type dynamixPolicyChecker struct {
	client        *dynamix.Client
	clusterConfig dynamixClusterConfiguration
	accountID     uint64

	visible map[string][]dynamix.StoragePolicy
}

func newDynamixPolicyChecker(client *dynamix.Client, clusterConfig dynamixClusterConfiguration) *dynamixPolicyChecker {
	return &dynamixPolicyChecker{
		client:        client,
		clusterConfig: clusterConfig,
		visible:       make(map[string][]dynamix.StoragePolicy),
	}
}

// check resolves the policy exactly the way the components
// that will place disks in it resolve it — narrowed to the account, as
// dynamix-common's GetStoragePolicyByName does — so that preflight is neither
// stricter nor laxer than the runtime it is standing in for. In particular a
// name is ambiguous only when it is ambiguous *within the account*: another
// account's policy of the same name is none of this cluster's business.
func (c *dynamixPolicyChecker) check(ctx context.Context, policy dynamixPolicyRef) error {
	matches, err := c.client.AccountStoragePoliciesNamed(ctx, c.accountID, policy.name)
	if err != nil {
		return dynamixAPIFailure(err, c.clusterConfig.Provider)
	}

	switch len(matches) {
	case 0:
		return c.explainInvisible(ctx, policy)
	case 1:
		return checkDynamixPolicyEnabled(matches[0], policy)
	default:
		return &preflight.Failure{
			Checked: policy.checked(),
			Observed: fmt.Sprintf("storage policy name %q matches %d policies of account %q, it does not identify one",
				policy.name, len(matches), c.clusterConfig.Account),
			Expected: "exactly one storage policy with that name in the account",
			Fix:      fmt.Sprintf("set %s to a name that identifies one policy, or have the platform administrator rename the others", policy.checked()),
			Err:      errDynamixPolicyAmbiguous,
		}
	}
}

// explainInvisible says why the account cannot see the
// policy. The same question is asked again without the account, because "there
// is no such policy" and "it is not yours" are different things to the user and
// only this second answer separates them.
func (c *dynamixPolicyChecker) explainInvisible(ctx context.Context, policy dynamixPolicyRef) error {
	matches, err := c.visiblePolicies(ctx, policy.name)
	if err != nil {
		return dynamixAPIFailure(err, c.clusterConfig.Provider)
	}

	if len(matches) == 0 {
		return &preflight.Failure{
			Checked:  policy.checked(),
			Observed: fmt.Sprintf("storage policy %q does not exist", policy.name),
			Expected: fmt.Sprintf("an ENABLED storage policy available to account %q", c.clusterConfig.Account),
			Fix:      fmt.Sprintf("set %s to the exact name of an ENABLED storage policy available to the account", policy.checked()),
			Err:      errDynamixPolicyNotFound,
		}
	}

	// A policy that could not be used even if it were granted is worth reporting
	// as such: the status has to be fixed either way, and fixing the grant first
	// would only buy the user a second refusal.
	if !slices.ContainsFunc(matches, dynamix.StoragePolicy.Enabled) {
		return checkDynamixPolicyEnabled(matches[0], policy)
	}

	return &preflight.Failure{
		Checked:  policy.checked(),
		Observed: fmt.Sprintf("storage policy %q is not available to account %q", policy.name, c.clusterConfig.Account),
		Expected: fmt.Sprintf("an ENABLED storage policy available to account %q", c.clusterConfig.Account),
		Fix:      fmt.Sprintf("grant account %q access to the policy, or set %s to another one", c.clusterConfig.Account, policy.checked()),
		Err:      errDynamixPolicyNotAvailable,
	}
}

// visiblePolicies answers which policies of a name the credentials can see,
// whatever account they belong to, asking the platform at most once per name.
func (c *dynamixPolicyChecker) visiblePolicies(ctx context.Context, name string) ([]dynamix.StoragePolicy, error) {
	if matches, ok := c.visible[name]; ok {
		return matches, nil
	}
	matches, err := c.client.StoragePoliciesNamed(ctx, name)
	if err != nil {
		return nil, err
	}
	c.visible[name] = matches
	return matches, nil
}

// checkDynamixPolicyEnabled refuses a policy that disks cannot be created in.
func checkDynamixPolicyEnabled(found dynamix.StoragePolicy, policy dynamixPolicyRef) error {
	if found.Enabled() {
		return nil
	}
	return &preflight.Failure{
		Checked:  policy.checked(),
		Observed: fmt.Sprintf("storage policy %q is in status %q", policy.name, found.Status),
		Expected: fmt.Sprintf("status %q: disks cannot be created in a policy that is not enabled", dynamix.StoragePolicyStatusEnabled),
		Fix:      fmt.Sprintf("have the platform administrator enable the policy, or set %s to an ENABLED one", policy.checked()),
		Err:      errDynamixPolicyNotEnabled,
	}
}

// dynamixPolicyRef is a storage policy name together with the field it was read
// from, so that a failure tells the user which line of their configuration to fix.
type dynamixPolicyRef struct {
	name  string
	field string
}

// checked is the field the policy was read from, in the reader's vocabulary.
func (p dynamixPolicyRef) checked() string {
	return dynamixClusterConfigurationKind + "." + p.field
}

type dynamixInstanceClass struct {
	StoragePolicy string `yaml:"storagePolicy"`
}

type dynamixClusterConfiguration struct {
	Kind            string                 `yaml:"kind"`
	Account         string                 `yaml:"account"`
	StoragePolicy   string                 `yaml:"storagePolicy"`
	Provider        dynamix.ProviderConfig `yaml:"provider"`
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

func parseDynamixClusterConfiguration(providerClusterConfig []byte) (dynamixClusterConfiguration, error) {
	var clusterConfig dynamixClusterConfiguration
	if err := yaml.Unmarshal(providerClusterConfig, &clusterConfig); err != nil {
		return dynamixClusterConfiguration{}, fmt.Errorf("malformed provider cluster configuration: %w", err)
	}
	return clusterConfig, nil
}

// validate re-states the presence of the fields the check reads. The OpenAPI
// schema already lists them in `required`, so a configuration that came through
// `dhctl config parse` cannot miss them — the guards are here so that one which
// reached preflight some other way fails loudly instead of quietly checking
// nothing on an empty policy name.
func (c dynamixClusterConfiguration) validate() error {
	if c.Account == "" {
		return missingDynamixField("account", "the name of the account the cluster runs under")
	}
	if c.StoragePolicy == "" {
		return missingDynamixField("storagePolicy", "the name of an ENABLED storage policy of the account")
	}
	return nil
}

func missingDynamixField(field, expected string) error {
	checked := dynamixClusterConfigurationKind + "." + field
	return &preflight.Failure{
		Checked:  checked,
		Observed: "the field is not set",
		Expected: expected,
		Fix:      fmt.Sprintf("set %s", checked),
		Err:      fmt.Errorf("malformed provider cluster configuration: reading .%s: no such property", field),
	}
}
