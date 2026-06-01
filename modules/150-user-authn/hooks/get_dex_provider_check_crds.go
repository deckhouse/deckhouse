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
	"bytes"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

const (
	dexProviderCheckQueue           = "/modules/user-authn/dex_provider_check"
	dexProviderCheckRetentionPeriod = 24 * time.Hour
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
	ClientID string `json:"clientID"`
}

type DexProviderGitlabForCheck struct {
	BaseURL    string `json:"baseURL,omitempty"`
	RootCAData string `json:"rootCAData,omitempty"`
}

type DexProviderBitbucketForCheck struct {
	ClientID string `json:"clientID"`
}

type DexProviderCrowdForCheck struct {
	BaseURL      string `json:"baseURL"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

type DexProviderOIDCForCheck struct {
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

func (r *dexProviderCheckResult) skip(name, format string, args ...any) {
	r.checks = append(r.checks, DexProviderCheckStepStatus{
		Name:    name,
		Status:  dexProviderCheckStepSkipped,
		Message: fmt.Sprintf(format, args...),
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

	certs, err := ldapReachable(ctx, provider.Spec.LDAP)
	if err != nil {
		result.fail("ldapReachable", "LDAP endpoint is not reachable: %v", err)
	} else {
		result.succeed("ldapReachable", "LDAP endpoint is reachable")
	}

	switch {
	case provider.Spec.LDAP.InsecureNoSSL:
		result.skip("ldapCertificate", "TLS is disabled (insecureNoSSL)")
	case err != nil:
		result.skip("ldapCertificate", "skipped because the LDAP endpoint is not reachable")
	default:
		reportLeafExpiry(result, "ldapCertificate", certs)
	}

	checkLDAPKerberosKeytab(ctx, input, dc, result, provider)
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

func ldapReachable(ctx context.Context, cfg *DexProviderLDAPForCheck) ([]*x509.Certificate, error) {
	host, serverName, err := ldapAddress(cfg)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: dexProviderCheckLDAPTimeout}
	if cfg.InsecureNoSSL {
		conn, err := dialer.DialContext(ctx, "tcp", host)
		if err != nil {
			return nil, fmt.Errorf("dial tcp: %w", err)
		}
		return nil, conn.Close()
	}

	tlsConfig, err := buildTLSConfig(cfg.RootCAData, serverName, cfg.InsecureSkipVerify)
	if err != nil {
		return nil, err
	}
	if cfg.StartTLS {
		return ldapStartTLS(ctx, dialer, host, tlsConfig)
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", host, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("dial tls: %w", err)
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates, nil
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

func ldapStartTLS(ctx context.Context, dialer *net.Dialer, addr string, tlsConfig *tls.Config) ([]*x509.Certificate, error) {
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(dexProviderCheckLDAPTimeout)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set LDAP connection deadline: %w", err)
	}

	if _, err := conn.Write(ldapStartTLSRequest); err != nil {
		return nil, fmt.Errorf("send LDAP StartTLS request: %w", err)
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read LDAP StartTLS response: %w", err)
	}
	if !bytes.Contains(buf[:n], ldapSuccessResultCode) {
		return nil, fmt.Errorf("LDAP StartTLS request was not accepted")
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("LDAP StartTLS handshake: %w", err)
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	return certs, tlsConn.Close()
}

var (
	ldapStartTLSRequest = []byte{
		0x30, 0x1d, // LDAPMessage sequence, 29 bytes
		0x02, 0x01, 0x01, // messageID = 1
		0x77, 0x18, // extendedReq, 24 bytes
		0x80, 0x16, // requestName, 22 bytes
		'1', '.', '3', '.', '6', '.', '1', '.', '4', '.', '1', '.', '1', '4', '6', '6', '.', '2', '0', '0', '3', '7',
	}
	ldapSuccessResultCode = []byte{0x0a, 0x01, 0x00}
)
