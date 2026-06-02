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

package hooks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/go-ldap/ldap/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

const (
	dexProviderCheckQueue           = "/modules/user-authn/dex_provider_check"
	dexProviderCheckRetentionPeriod = 6 * time.Hour
	dexProviderCheckTimeout         = 20 * time.Second
	dexProviderCheckHTTPTimeout     = 5 * time.Second
	dexProviderCheckLDAPTimeout     = 5 * time.Second
	dexProviderCertExpiryWarnWindow = 14 * 24 * time.Hour

	userAuthnNamespace = "d8-user-authn"
	dexDiscoveryURL    = "https://dex.d8-user-authn/.well-known/openid-configuration"
)

type DexProviderCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexProviderCheckSpec   `json:"spec"`
	Status            DexProviderCheckStatus `json:"status"`
}

type DexProviderCheckSpec struct {
	ProviderName  string `json:"providerName"`
	InitiatorType string `json:"initiatorType,omitempty"`
}

type DexProviderCheckStatus struct {
	Phase                         DexProviderCheckPhase        `json:"phase"`
	Message                       string                       `json:"message,omitempty"`
	ObservedDexProviderGeneration int64                        `json:"observedDexProviderGeneration,omitempty"`
	Checks                        []DexProviderCheckStepStatus `json:"checks,omitempty"`
	CompletedAt                   *metav1.Time                 `json:"completedAt,omitempty"`
}

type DexProviderCheckPhase string

const (
	DexProviderCheckPhaseSucceeded = DexProviderCheckPhase("Succeeded")
	DexProviderCheckPhaseFailed    = DexProviderCheckPhase("Failed")
)

type DexProviderCheckStepStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

const (
	dexProviderCheckStepSucceeded = "Succeeded"
	dexProviderCheckStepFailed    = "Failed"
	dexProviderCheckStepSkipped   = "Skipped"
)

type DexProviderForCheck struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexProviderForCheckSpec `json:"spec"`
}

type DexProviderForCheckSpec struct {
	Enabled        *bool                         `json:"enabled,omitempty"`
	Type           string                        `json:"type"`
	Github         *DexProviderGithubForCheck    `json:"github,omitempty"`
	Gitlab         *DexProviderGitlabForCheck    `json:"gitlab,omitempty"`
	BitbucketCloud *DexProviderBitbucketForCheck `json:"bitbucketCloud,omitempty"`
	Crowd          *DexProviderCrowdForCheck     `json:"crowd,omitempty"`
	OIDC           *DexProviderOIDCForCheck      `json:"oidc,omitempty"`
	LDAP           *DexProviderLDAPForCheck      `json:"ldap,omitempty"`
	SAML           *DexProviderSAMLForCheck      `json:"saml,omitempty"`
}

type DexProviderGithubForCheck struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

type DexProviderGitlabForCheck struct {
	ClientID     string `json:"clientID,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	BaseURL      string `json:"baseURL,omitempty"`
	RootCAData   string `json:"rootCAData,omitempty"`
}

type DexProviderBitbucketForCheck struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

type DexProviderCrowdForCheck struct {
	BaseURL      string `json:"baseURL"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

type DexProviderOIDCForCheck struct {
	ClientID           string `json:"clientID,omitempty"`
	ClientSecret       string `json:"clientSecret,omitempty"`
	Issuer             string `json:"issuer"`
	RootCAData         string `json:"rootCAData,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

type DexProviderLDAPForCheck struct {
	Host               string                           `json:"host"`
	InsecureNoSSL      bool                             `json:"insecureNoSSL,omitempty"`
	StartTLS           bool                             `json:"startTLS,omitempty"`
	RootCAData         string                           `json:"rootCAData,omitempty"`
	InsecureSkipVerify bool                             `json:"insecureSkipVerify,omitempty"`
	BindDN             string                           `json:"bindDN,omitempty"`
	BindPW             string                           `json:"bindPW,omitempty"`
	Kerberos           *DexProviderLDAPKerberosForCheck `json:"kerberos,omitempty"`
}

type DexProviderLDAPKerberosForCheck struct {
	Enabled          bool   `json:"enabled,omitempty"`
	KeytabSecretName string `json:"keytabSecretName,omitempty"`
}

type DexProviderSAMLForCheck struct {
	SSOURL     string `json:"ssoURL"`
	RootCAData string `json:"rootCAData,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: dexProviderCheckQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "dexproviderchecks",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "DexProviderCheck",
			FilterFunc:          applyDexProviderCheckFilter,
			ExecuteHookOnEvents: ptr.To(true),
		},
		{
			Name:                "dexproviders_for_check",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "DexProvider",
			FilterFunc:          applyDexProviderForCheckFilter,
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, dependency.WithExternalDependencies(getDexProviderChecks))

func applyDexProviderCheckFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	check := &DexProviderCheck{}
	if err := sdk.FromUnstructured(obj, check); err != nil {
		return nil, fmt.Errorf("cannot convert DexProviderCheck %q: %w", obj.GetName(), err)
	}
	return check, nil
}

func applyDexProviderForCheckFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	provider := &DexProviderForCheck{}
	if err := sdk.FromUnstructured(obj, provider); err != nil {
		return nil, fmt.Errorf("cannot convert DexProvider %q: %w", obj.GetName(), err)
	}
	return provider, nil
}

func getDexProviderChecks(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	providers, err := dexProvidersForCheck(input)
	if err != nil {
		return err
	}

	for check, err := range sdkobjectpatch.SnapshotIter[DexProviderCheck](input.Snapshots.Get("dexproviderchecks")) {
		if err != nil {
			return fmt.Errorf("iterate DexProviderCheck snapshots: %w", err)
		}

		if dexProviderCheckCompleted(check) {
			if time.Since(check.GetCreationTimestamp().Time) >= dexProviderCheckRetentionPeriod {
				input.PatchCollector.Delete("deckhouse.io/v1", "DexProviderCheck", "", check.Name)
			}
			continue
		}

		status := executeDexProviderCheck(ctx, input, dc, check, providers[check.Spec.ProviderName])
		input.PatchCollector.PatchWithMerge(
			map[string]any{"status": status},
			"deckhouse.io/v1",
			"DexProviderCheck",
			"",
			check.Name,
			object_patch.WithSubresource("status"),
		)
	}

	return nil
}

func dexProvidersForCheck(input *go_hook.HookInput) (map[string]DexProviderForCheck, error) {
	providers := map[string]DexProviderForCheck{}
	for provider, err := range sdkobjectpatch.SnapshotIter[DexProviderForCheck](input.Snapshots.Get("dexproviders_for_check")) {
		if err != nil {
			return nil, fmt.Errorf("iterate DexProvider snapshots: %w", err)
		}
		providers[provider.Name] = provider
	}
	return providers, nil
}

func dexProviderCheckCompleted(check DexProviderCheck) bool {
	return check.Status.Phase == DexProviderCheckPhaseSucceeded || check.Status.Phase == DexProviderCheckPhaseFailed
}

func executeDexProviderCheck(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	check DexProviderCheck,
	provider DexProviderForCheck,
) DexProviderCheckStatus {
	checkCtx, cancel := context.WithTimeout(ctx, dexProviderCheckTimeout)
	defer cancel()

	result := &dexProviderCheckResult{checks: make([]DexProviderCheckStepStatus, 0, 4)}
	if provider.Name == "" {
		result.fail("providerExists", "DexProvider %q not found", check.Spec.ProviderName)
		return result.status(0)
	}
	result.succeed("providerExists", "DexProvider %q found", provider.Name)

	if provider.Spec.Enabled != nil && !*provider.Spec.Enabled {
		result.fail("providerEnabled", "DexProvider %q is disabled", provider.Name)
		return result.status(provider.Generation)
	}
	result.succeed("providerEnabled", "DexProvider %q is enabled", provider.Name)

	checkDexReachability(checkCtx, dc, result)
	checkProviderReachability(checkCtx, input, dc, result, provider)

	return result.status(provider.Generation)
}

type dexProviderCheckResult struct {
	checks []DexProviderCheckStepStatus
}

func (r *dexProviderCheckResult) succeed(name, format string, args ...any) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  dexProviderCheckStepSucceeded,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *dexProviderCheckResult) fail(name, format string, args ...any) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  dexProviderCheckStepFailed,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *dexProviderCheckResult) skip(name, message string) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  dexProviderCheckStepSkipped,
		Message: message,
	})
}

func (r *dexProviderCheckResult) status(observedGeneration int64) DexProviderCheckStatus {
	phase := DexProviderCheckPhaseSucceeded
	message := "connectivity check passed"
	for _, check := range r.checks {
		if check.Status == dexProviderCheckStepFailed {
			phase = DexProviderCheckPhaseFailed
			message = check.Message
			break
		}
	}

	return DexProviderCheckStatus{
		Phase:                         phase,
		Message:                       message,
		ObservedDexProviderGeneration: observedGeneration,
		Checks:                        r.checks,
		CompletedAt:                   ptr.To(metav1.Now()),
	}
}

func checkDexReachability(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult) {
	client := dc.GetHTTPClient(d8http.WithTimeout(dexProviderCheckHTTPTimeout), d8http.WithInsecureSkipVerify())
	statusCode, body, err := httpGet(ctx, client, dexDiscoveryURL, nil)
	if err != nil {
		result.fail("dexReady", "Dex discovery is not reachable: %v", err)
		return
	}
	if statusCode != http.StatusOK {
		result.fail("dexReady", "Dex discovery returned HTTP %d", statusCode)
		return
	}

	var discovery struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(body, &discovery); err != nil {
		result.fail("dexReady", "Dex discovery returned invalid JSON: %v", err)
		return
	}
	if discovery.Issuer == "" {
		result.fail("dexReady", "Dex discovery response has empty issuer")
		return
	}
	result.succeed("dexReady", "Dex discovery is reachable")
}

func checkProviderReachability(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	result *dexProviderCheckResult,
	provider DexProviderForCheck,
) {
	switch provider.Spec.Type {
	case "Github":
		checkGithub(ctx, dc, result, provider)
	case "Gitlab":
		checkGitlab(ctx, dc, result, provider)
	case "BitbucketCloud":
		checkBitbucket(ctx, dc, result, provider)
	case "Crowd":
		checkCrowd(ctx, dc, result, provider)
	case "OIDC":
		checkOIDC(ctx, dc, result, provider)
	case "LDAP":
		checkLDAP(ctx, input, dc, result, provider)
	case "SAML":
		checkSAML(ctx, dc, result, provider)
	default:
		result.fail("providerConfig", "unsupported DexProvider type %q", provider.Spec.Type)
	}
}

func checkGithub(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.Github == nil {
		result.fail("githubAPI", "GitHub provider config is missing")
		return
	}
	if provider.Spec.Github.ClientID == "" {
		result.fail("githubAPI", "GitHub clientID is empty")
		return
	}

	checkHTTPReachability(ctx, dc, result, "githubAPI", "https://api.github.com/meta", "")
	checkGithubCredentials(ctx, dc, result, provider.Spec.Github)
}

// checkGithubCredentials verifies the GitHub OAuth app client_id/client_secret
// without a user flow. GitHub's access_token endpoint reports
// "incorrect_client_credentials" when the secret is wrong and
// "bad_verification_code" when the credentials are valid but the (intentionally
// bogus) authorization code is not, which lets us tell the two apart.
func checkGithubCredentials(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, cfg *DexProviderGithubForCheck) {
	if cfg.ClientSecret == "" {
		result.skip("githubCredentials", "clientSecret is empty")
		return
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", "deckhouse-dex-provider-check")

	client := dc.GetHTTPClient(d8http.WithTimeout(dexProviderCheckHTTPTimeout))
	statusCode, body, err := httpPostForm(ctx, client, "https://github.com/login/oauth/access_token", "", "", form)
	if err != nil {
		result.fail("githubCredentials", "cannot reach the GitHub token endpoint: %v", err)
		return
	}
	if statusCode != http.StatusOK {
		result.fail("githubCredentials", "GitHub token endpoint returned HTTP %d", statusCode)
		return
	}

	var githubErr struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &githubErr)
	if githubErr.Error == "incorrect_client_credentials" {
		result.fail("githubCredentials", "GitHub rejected the client credentials")
		return
	}
	result.succeed("githubCredentials", "GitHub accepted the client credentials")
}

func checkGitlab(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.Gitlab == nil {
		result.fail("gitlabURL", "GitLab provider config is missing")
		return
	}

	baseURL := strings.TrimSpace(provider.Spec.Gitlab.BaseURL)
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	checkCABundle(result, "gitlabCABundle", provider.Spec.Gitlab.RootCAData)
	checkTLSCertificate(result, "gitlabCertificate", baseURL, provider.Spec.Gitlab.RootCAData, false)
	checkHTTPReachability(ctx, dc, result, "gitlabURL", baseURL, provider.Spec.Gitlab.RootCAData)
	checkGitlabCredentials(ctx, dc, result, provider.Spec.Gitlab, baseURL)
}

func checkGitlabCredentials(
	ctx context.Context,
	dc dependency.Container,
	result *dexProviderCheckResult,
	cfg *DexProviderGitlabForCheck,
	baseURL string,
) {
	tokenURL := strings.TrimRight(baseURL, "/") + "/oauth/token"
	client := dc.GetHTTPClient(httpOptions(cfg.RootCAData, false)...)
	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "gitlabCredentials",
		providerName: "GitLab",
		tokenURL:     tokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func checkBitbucket(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.BitbucketCloud == nil {
		result.fail("bitbucketAPI", "Bitbucket Cloud provider config is missing")
		return
	}
	if provider.Spec.BitbucketCloud.ClientID == "" {
		result.fail("bitbucketAPI", "Bitbucket Cloud clientID is empty")
		return
	}

	checkHTTPReachability(ctx, dc, result, "bitbucketAPI", "https://api.bitbucket.org/2.0/", "")
	checkBitbucketCredentials(ctx, dc, result, provider.Spec.BitbucketCloud)
}

func checkBitbucketCredentials(
	ctx context.Context,
	dc dependency.Container,
	result *dexProviderCheckResult,
	cfg *DexProviderBitbucketForCheck,
) {
	const tokenURL = "https://bitbucket.org/site/oauth2/access_token"
	client := dc.GetHTTPClient(d8http.WithTimeout(dexProviderCheckHTTPTimeout))
	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "bitbucketCredentials",
		providerName: "Bitbucket",
		tokenURL:     tokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func checkCrowd(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.Crowd == nil {
		result.fail("crowdAPI", "Crowd provider config is missing")
		return
	}
	if provider.Spec.Crowd.BaseURL == "" {
		result.fail("crowdAPI", "Crowd baseURL is empty")
		return
	}
	if provider.Spec.Crowd.ClientID == "" || provider.Spec.Crowd.ClientSecret == "" {
		result.fail("crowdAPI", "Crowd clientID or clientSecret is empty")
		return
	}

	endpoint := strings.TrimRight(provider.Spec.Crowd.BaseURL, "/") + "/rest/usermanagement/1/config/cookie"
	headers := map[string]string{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		result.fail("crowdAPI", "Crowd URL is invalid: %v", err)
		return
	}
	req.SetBasicAuth(provider.Spec.Crowd.ClientID, provider.Spec.Crowd.ClientSecret)
	headers["Authorization"] = req.Header.Get("Authorization")

	client := dc.GetHTTPClient(d8http.WithTimeout(dexProviderCheckHTTPTimeout))
	statusCode, _, err := httpGet(ctx, client, endpoint, headers)
	if err != nil {
		result.fail("crowdAPI", "Crowd API is not reachable: %v", err)
		return
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		result.fail("crowdAPI", "Crowd API returned HTTP %d", statusCode)
		return
	}
	result.succeed("crowdAPI", "Crowd API accepted application credentials")
}

func checkOIDC(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.OIDC == nil {
		result.fail("oidcDiscovery", "OIDC provider config is missing")
		return
	}
	issuer := strings.TrimRight(provider.Spec.OIDC.Issuer, "/")
	if issuer == "" {
		result.fail("oidcDiscovery", "OIDC issuer is empty")
		return
	}

	checkPublicBrowserURL(result, "oidcIssuerPublic", issuer)
	checkCABundle(result, "oidcCABundle", provider.Spec.OIDC.RootCAData)
	checkTLSCertificate(result, "oidcCertificate", issuer, provider.Spec.OIDC.RootCAData, provider.Spec.OIDC.InsecureSkipVerify)

	client := dc.GetHTTPClient(httpOptions(provider.Spec.OIDC.RootCAData, provider.Spec.OIDC.InsecureSkipVerify)...)
	discoveryURL := issuer + "/.well-known/openid-configuration"
	statusCode, body, err := httpGet(ctx, client, discoveryURL, nil)
	if err != nil {
		result.fail("oidcDiscovery", "OIDC discovery is not reachable: %v", err)
		return
	}
	if statusCode != http.StatusOK {
		result.fail("oidcDiscovery", "OIDC discovery returned HTTP %d", statusCode)
		return
	}

	var discovery oidcDiscoveryDocument
	if err := json.Unmarshal(body, &discovery); err != nil {
		result.fail("oidcDiscovery", "OIDC discovery returned invalid JSON: %v", err)
		return
	}
	if discovery.Issuer == "" || discovery.JWKSURI == "" {
		result.fail("oidcDiscovery", "OIDC discovery response is missing issuer or jwks_uri")
		return
	}
	result.succeed("oidcDiscovery", "OIDC discovery is reachable")

	if strings.TrimRight(discovery.Issuer, "/") != issuer {
		result.fail("oidcIssuerMatch", "discovery issuer %q does not match the configured issuer %q", discovery.Issuer, issuer)
	} else {
		result.succeed("oidcIssuerMatch", "discovery issuer matches the configured issuer")
	}

	if missing := discovery.missingEndpoints(); len(missing) > 0 {
		result.fail("oidcEndpoints", "OIDC discovery is missing endpoints: %s", strings.Join(missing, ", "))
	} else {
		result.succeed("oidcEndpoints", "authorization and token endpoints are advertised")
	}

	checkOIDCCredentials(ctx, client, result, provider.Spec.OIDC, discovery.TokenEndpoint)

	statusCode, body, err = httpGet(ctx, client, discovery.JWKSURI, nil)
	if err != nil {
		result.fail("oidcJWKS", "OIDC JWKS is not reachable: %v", err)
		return
	}
	if statusCode != http.StatusOK {
		result.fail("oidcJWKS", "OIDC JWKS returned HTTP %d", statusCode)
		return
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		result.fail("oidcJWKS", "OIDC JWKS returned invalid JSON: %v", err)
		return
	}
	if len(jwks.Keys) == 0 {
		result.fail("oidcJWKS", "OIDC JWKS has no keys")
		return
	}
	result.succeed("oidcJWKS", "OIDC JWKS is reachable")
}

func checkOIDCCredentials(
	ctx context.Context,
	client d8http.Client,
	result *dexProviderCheckResult,
	cfg *DexProviderOIDCForCheck,
	tokenEndpoint string,
) {
	if tokenEndpoint == "" {
		result.skip("oidcCredentials", "OIDC discovery does not advertise a token endpoint")
		return
	}

	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "oidcCredentials",
		providerName: "OIDC",
		tokenURL:     tokenEndpoint,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func checkLDAP(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	result *dexProviderCheckResult,
	provider DexProviderForCheck,
) {
	if provider.Spec.LDAP == nil {
		result.fail("ldapReachable", "LDAP provider config is missing")
		return
	}
	if provider.Spec.LDAP.Host == "" {
		result.fail("ldapReachable", "LDAP host is empty")
		return
	}

	checkCABundle(result, "ldapCABundle", provider.Spec.LDAP.RootCAData)

	conn, err := ldapDial(provider.Spec.LDAP)
	if err != nil {
		result.fail("ldapReachable", "LDAP endpoint is not reachable: %v", err)
	} else {
		defer conn.Close()
		result.succeed("ldapReachable", "LDAP endpoint is reachable")
	}

	switch {
	case provider.Spec.LDAP.InsecureNoSSL:
		result.skip("ldapCertificate", "TLS is disabled (insecureNoSSL)")
	case err != nil:
		result.skip("ldapCertificate", "skipped because the LDAP endpoint is not reachable")
	default:
		if state, ok := conn.TLSConnectionState(); ok {
			reportLeafExpiry(result, "ldapCertificate", state.PeerCertificates)
		} else {
			result.fail("ldapCertificate", "TLS connection state is not available")
		}
	}

	checkLDAPBind(result, provider.Spec.LDAP, conn, err)
	checkLDAPKerberosKeytab(ctx, input, dc, result, provider)
}

// checkLDAPBind performs a real LDAP simple bind with the configured service
// account so that bindDN/bindPW (the credentials Dex uses to search the
// directory) are validated, not just the network reachability.
func checkLDAPBind(result *dexProviderCheckResult, cfg *DexProviderLDAPForCheck, conn *ldap.Conn, dialErr error) {
	switch {
	case dialErr != nil:
		result.skip("ldapBind", "skipped because the LDAP endpoint is not reachable")
	case cfg.BindDN == "":
		result.skip("ldapBind", "no bindDN configured (anonymous access)")
	default:
		if err := conn.Bind(cfg.BindDN, cfg.BindPW); err != nil {
			result.fail("ldapBind", "LDAP service account bind failed: %v", err)
		} else {
			result.succeed("ldapBind", "LDAP service account bind succeeded")
		}
	}
}

func checkLDAPKerberosKeytab(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	result *dexProviderCheckResult,
	provider DexProviderForCheck,
) {
	if provider.Spec.LDAP.Kerberos == nil || !provider.Spec.LDAP.Kerberos.Enabled {
		result.skip("ldapKerberosKeytab", "LDAP Kerberos is disabled")
		return
	}
	if provider.Spec.LDAP.Kerberos.KeytabSecretName == "" {
		result.fail("ldapKerberosKeytab", "LDAP Kerberos is enabled but keytabSecretName is empty")
		return
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		result.fail("ldapKerberosKeytab", "cannot create Kubernetes client: %v", err)
		return
	}
	_, err = kubeClient.CoreV1().Secrets(userAuthnNamespace).Get(
		ctx,
		provider.Spec.LDAP.Kerberos.KeytabSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		input.Logger.Warn("cannot find LDAP Kerberos keytab Secret",
			"provider", provider.Name,
			"secret", provider.Spec.LDAP.Kerberos.KeytabSecretName,
		)
		result.fail("ldapKerberosKeytab", "keytab Secret %q is not available", provider.Spec.LDAP.Kerberos.KeytabSecretName)
		return
	}
	result.succeed("ldapKerberosKeytab", "keytab Secret %q is available", provider.Spec.LDAP.Kerberos.KeytabSecretName)
}

func checkSAML(ctx context.Context, dc dependency.Container, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.SAML == nil {
		result.fail("samlSSOURL", "SAML provider config is missing")
		return
	}
	if provider.Spec.SAML.SSOURL == "" {
		result.fail("samlSSOURL", "SAML ssoURL is empty")
		return
	}

	checkPublicBrowserURL(result, "samlSSOURLPublic", provider.Spec.SAML.SSOURL)
	checkCABundle(result, "samlCABundle", provider.Spec.SAML.RootCAData)
	checkTLSCertificate(result, "samlCertificate", provider.Spec.SAML.SSOURL, provider.Spec.SAML.RootCAData, false)
	checkHTTPReachability(ctx, dc, result, "samlSSOURL", provider.Spec.SAML.SSOURL, provider.Spec.SAML.RootCAData)
}

func checkHTTPReachability(
	ctx context.Context,
	dc dependency.Container,
	result *dexProviderCheckResult,
	stepName string,
	rawURL string,
	rootCAData string,
) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		result.fail(stepName, "URL %q is invalid: %v", rawURL, err)
		return
	}

	client := dc.GetHTTPClient(httpOptions(rootCAData, false)...)
	statusCode, _, err := httpGet(ctx, client, rawURL, nil)
	if err != nil {
		result.fail(stepName, "URL %q is not reachable: %v", rawURL, err)
		return
	}
	if statusCode >= http.StatusInternalServerError {
		result.fail(stepName, "URL %q returned HTTP %d", rawURL, statusCode)
		return
	}
	result.succeed(stepName, "URL %q is reachable with HTTP %d", rawURL, statusCode)
}

// checkPublicBrowserURL verifies that a browser-facing endpoint is not a
// cluster-internal address. For OIDC the user's browser is redirected to the
// issuer's authorization endpoint, and for SAML it is redirected to ssoURL, so
// these URLs must resolve from outside the cluster. Backend connectivity checks
// can pass against an in-cluster Service (e.g. *.svc) while real logins still
// break in the browser; this step catches that misconfiguration.
func checkPublicBrowserURL(result *dexProviderCheckResult, stepName, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		result.fail(stepName, "URL %q is invalid: %v", rawURL, err)
		return
	}
	if reason := clusterInternalHostReason(parsed.Hostname()); reason != "" {
		result.fail(stepName,
			"URL %q is not browser-reachable (%s); the user's browser is redirected here during login, so it must be a publicly resolvable domain",
			rawURL, reason)
		return
	}
	result.succeed(stepName, "URL %q uses a publicly resolvable host", rawURL)
}

// clusterInternalHostReason reports why a host is only reachable inside the
// cluster, or an empty string if it looks publicly resolvable. It is a
// best-effort heuristic: it flags loopback/private/link-local IPs, localhost,
// Kubernetes service domains (*.svc, *.cluster.local), the .local mDNS suffix
// and bare single-label hostnames.
func clusterInternalHostReason(host string) string {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if ip := net.ParseIP(h); ip != nil {
		switch {
		case ip.IsLoopback():
			return "loopback IP address"
		case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast():
			return "link-local IP address"
		case ip.IsPrivate():
			return "private IP address"
		case ip.IsUnspecified():
			return "unspecified IP address"
		}
		return ""
	}
	if h == "localhost" {
		return "localhost"
	}
	for _, suffix := range []string{".cluster.local", ".svc", ".local"} {
		if strings.HasSuffix(h, suffix) {
			return "cluster-internal domain (" + suffix + ")"
		}
	}
	if !strings.Contains(h, ".") {
		return "single-label hostname resolvable only inside the cluster"
	}
	return ""
}

func httpOptions(rootCAData string, insecureSkipVerify bool) []d8http.Option {
	options := []d8http.Option{d8http.WithTimeout(dexProviderCheckHTTPTimeout)}
	if insecureSkipVerify {
		options = append(options, d8http.WithInsecureSkipVerify())
	}
	if rootCAData != "" {
		options = append(options, d8http.WithAdditionalCACerts([][]byte{[]byte(rootCAData)}))
	}
	return options
}

func httpGet(ctx context.Context, client d8http.Client, rawURL string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, body, nil
}

func httpPostForm(
	ctx context.Context,
	client d8http.Client,
	rawURL, basicUser, basicPass string,
	form url.Values,
) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, body, nil
}

// oauthClientSecretCheck describes a single OAuth 2.0 / OIDC client-secret
// verification against a provider's token endpoint.
type oauthClientSecretCheck struct {
	stepName     string
	providerName string
	tokenURL     string
	clientID     string
	clientSecret string
}

// reportOAuthClientSecret verifies a confidential client secret and records the
// result. It unifies the OIDC, GitLab and Bitbucket credential steps so they
// share the same probing logic and messages.
func reportOAuthClientSecret(
	ctx context.Context,
	client d8http.Client,
	result *dexProviderCheckResult,
	check oauthClientSecretCheck,
) {
	if check.clientID == "" || check.clientSecret == "" {
		result.skip(check.stepName, "clientID or clientSecret is empty")
		return
	}

	accepted, detail, err := probeClientSecret(ctx, client, check.tokenURL, check.clientID, check.clientSecret)
	switch {
	case err != nil:
		result.fail(check.stepName, "cannot reach the %s token endpoint: %v", check.providerName, err)
	case !accepted:
		result.fail(check.stepName, "%s rejected the client credentials: %s", check.providerName, detail)
	default:
		result.succeed(check.stepName, "%s client credentials are valid (%s)", check.providerName, detail)
	}
}

// probeClientSecret verifies a confidential OAuth 2.0 / OIDC client secret
// without performing an interactive login. It sends an authorization_code token
// request with a deliberately invalid code: the authorization server
// authenticates the client *before* validating the code, so the response
// reveals whether the secret is correct.
//
//   - "invalid_client"/"unauthorized_client" or HTTP 401 → the secret is wrong.
//     RFC 6749 mandates invalid_client; some providers (e.g. Bitbucket) answer
//     unauthorized_client with "Invalid OAuth client credentials" instead.
//   - any other error (invalid_grant, invalid_request, …) or a 2xx response →
//     the client authenticated and only the bogus code was rejected.
//
// The authorization_code grant is used rather than client_credentials because it
// is always enabled for interactive login clients, whereas client_credentials is
// frequently disabled and then answered with unauthorized_client regardless of
// whether the secret is correct — which would yield false positives.
//
// The secret is sent via both HTTP Basic auth and the form body to satisfy
// providers expecting either client authentication style.
func probeClientSecret(
	ctx context.Context,
	client d8http.Client,
	tokenURL, clientID, clientSecret string,
) (bool, string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "deckhouse-dex-provider-check")
	form.Set("redirect_uri", "https://deckhouse.local/dex-provider-check")

	statusCode, body, err := httpPostForm(ctx, client, tokenURL, clientID, clientSecret, form)
	if err != nil {
		return false, "", err
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return true, "token endpoint accepted the client", nil
	}

	var oauthErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &oauthErr)

	clientAuthFailed := statusCode == http.StatusUnauthorized ||
		oauthErr.Error == "invalid_client" ||
		oauthErr.Error == "unauthorized_client"
	if clientAuthFailed {
		if oauthErr.Error != "" {
			return false, fmt.Sprintf("client authentication failed (%s)", oauthErr.Error), nil
		}
		return false, "client authentication failed (HTTP 401)", nil
	}

	if oauthErr.Error != "" {
		return true, fmt.Sprintf("client authenticated; test code rejected (%s)", oauthErr.Error), nil
	}
	return true, fmt.Sprintf("token endpoint returned HTTP %d", statusCode), nil
}

type oidcDiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	JWKSURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

func (d oidcDiscoveryDocument) missingEndpoints() []string {
	var missing []string
	if d.AuthorizationEndpoint == "" {
		missing = append(missing, "authorization_endpoint")
	}
	if d.TokenEndpoint == "" {
		missing = append(missing, "token_endpoint")
	}
	return missing
}

// checkCABundle validates a user-supplied PEM CA bundle: that it parses and
// that its certificates are not expired. It needs no provider credentials.
func checkCABundle(result *dexProviderCheckResult, stepName, rootCAData string) {
	if rootCAData == "" {
		result.skip(stepName, "No custom CA provided; using the system trust store")
		return
	}

	notAfter, err := earliestCertExpiry([]byte(rootCAData))
	if err != nil {
		result.fail(stepName, "rootCAData is invalid: %v", err)
		return
	}
	reportExpiry(result, stepName, "rootCAData certificate", notAfter)
}

// earliestCertExpiry returns the soonest NotAfter among the certificates in a
// PEM bundle. It is the limiting factor for the bundle's validity.
func earliestCertExpiry(pemData []byte) (time.Time, error) {
	rest := pemData
	var earliest time.Time
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse certificate: %w", err)
		}
		if !found || cert.NotAfter.Before(earliest) {
			earliest = cert.NotAfter
			found = true
		}
	}
	if !found {
		return time.Time{}, errors.New("no certificates found")
	}
	return earliest, nil
}

// checkTLSCertificate performs a TLS handshake against an HTTPS endpoint and
// reports the validity window of the server certificate. It is a transport-only
// probe and requires no provider credentials.
func checkTLSCertificate(result *dexProviderCheckResult, stepName, rawURL, rootCAData string, insecureSkipVerify bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		result.skip(stepName, "endpoint is not HTTPS; certificate check skipped")
		return
	}

	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	tlsConfig, err := buildTLSConfig(rootCAData, parsed.Hostname(), insecureSkipVerify)
	if err != nil {
		result.fail(stepName, "cannot build TLS config: %v", err)
		return
	}

	dialer := &net.Dialer{Timeout: dexProviderCheckHTTPTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(parsed.Hostname(), port), tlsConfig)
	if err != nil {
		result.fail(stepName, "cannot establish TLS connection: %v", err)
		return
	}
	defer conn.Close()
	reportLeafExpiry(result, stepName, conn.ConnectionState().PeerCertificates)
}

func reportLeafExpiry(result *dexProviderCheckResult, stepName string, certs []*x509.Certificate) {
	if len(certs) == 0 {
		result.fail(stepName, "server did not present a TLS certificate")
		return
	}
	reportExpiry(result, stepName, "server TLS certificate", certs[0].NotAfter)
}

func reportExpiry(result *dexProviderCheckResult, stepName, subject string, notAfter time.Time) {
	remaining := time.Until(notAfter)
	expiry := notAfter.UTC().Format(time.RFC3339)
	switch {
	case remaining <= 0:
		result.fail(stepName, "%s expired on %s", subject, expiry)
	case remaining <= dexProviderCertExpiryWarnWindow:
		result.succeed(stepName, "%s expires soon, on %s", subject, expiry)
	default:
		result.succeed(stepName, "%s is valid until %s", subject, expiry)
	}
}

// ldapDial opens an LDAP connection honouring the provider's TLS settings
// (ldaps, StartTLS or plain ldap). The caller owns the returned connection and
// must close it. It uses the go-ldap library so that the StartTLS handshake and
// later bind follow the protocol correctly.
func ldapDial(cfg *DexProviderLDAPForCheck) (*ldap.Conn, error) {
	host, serverName, err := ldapAddress(cfg)
	if err != nil {
		return nil, err
	}

	dialOpts := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: dexProviderCheckLDAPTimeout})}

	if cfg.InsecureNoSSL {
		conn, err := ldap.DialURL("ldap://"+host, dialOpts...)
		if err != nil {
			return nil, fmt.Errorf("dial ldap: %w", err)
		}
		conn.SetTimeout(dexProviderCheckLDAPTimeout)
		return conn, nil
	}

	tlsConfig, err := buildTLSConfig(cfg.RootCAData, serverName, cfg.InsecureSkipVerify)
	if err != nil {
		return nil, err
	}

	if cfg.StartTLS {
		conn, err := ldap.DialURL("ldap://"+host, dialOpts...)
		if err != nil {
			return nil, fmt.Errorf("dial ldap: %w", err)
		}
		conn.SetTimeout(dexProviderCheckLDAPTimeout)
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS: %w", err)
		}
		return conn, nil
	}

	conn, err := ldap.DialURL("ldaps://"+host, append(dialOpts, ldap.DialWithTLSConfig(tlsConfig))...)
	if err != nil {
		return nil, fmt.Errorf("dial ldaps: %w", err)
	}
	conn.SetTimeout(dexProviderCheckLDAPTimeout)
	return conn, nil
}

func ldapAddress(cfg *DexProviderLDAPForCheck) (string, string, error) {
	host, port, err := net.SplitHostPort(cfg.Host)
	if err == nil {
		return net.JoinHostPort(host, port), host, nil
	}

	if strings.Contains(err.Error(), "missing port in address") {
		port = "636"
		if cfg.InsecureNoSSL || cfg.StartTLS {
			port = "389"
		}
		return net.JoinHostPort(cfg.Host, port), cfg.Host, nil
	}
	return "", "", fmt.Errorf("parse LDAP host %q: %w", cfg.Host, err)
}

// buildTLSConfig builds a tls.Config that trusts the system roots plus an
// optional PEM-encoded CA bundle. It is shared by the LDAP and HTTPS
// certificate checks so that custom CAs are honoured consistently.
func buildTLSConfig(rootCAData, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         serverName,
	}
	if insecureSkipVerify {
		return tlsConfig, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system CA pool: %w", err)
	}
	if rootCAData != "" && !pool.AppendCertsFromPEM([]byte(rootCAData)) {
		return nil, errors.New("append rootCAData: no certificates found")
	}
	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}
