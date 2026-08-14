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

package providercheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (r *Reconciler) checkGithub(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.Github == nil {
		result.fail("githubAPI", "GitHub provider config is missing")
		return
	}
	if provider.Spec.Github.ClientID == "" {
		result.fail("githubAPI", "GitHub clientID is empty")
		return
	}

	r.checkHTTPReachability(ctx, result, "githubAPI", "https://api.github.com/meta", "")
	r.checkGithubCredentials(ctx, result, provider.Spec.Github)
}

func (r *Reconciler) checkGithubCredentials(ctx context.Context, result *dexProviderCheckResult, cfg *DexProviderGithubForCheck) {
	if cfg.ClientSecret == "" {
		result.skip("githubCredentials", "clientSecret is empty")
		return
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", "deckhouse-dex-provider-check")

	client, err := r.http.New(TLSOptions{})
	if err != nil {
		result.fail("githubCredentials", "cannot build HTTP client: %v", err)
		return
	}
	statusCode, body, err := httpPostForm(ctx, client, "https://github.com/login/oauth/access_token", "", "", form)
	if err != nil {
		result.failUnreachable("githubCredentials", err, "cannot reach the GitHub token endpoint: %v", err)
		return
	}
	if statusCode != http.StatusOK {
		result.fail("githubCredentials", "GitHub token endpoint returned HTTP %d", statusCode)
		return
	}

	if oauthErrorCode(body) == "incorrect_client_credentials" {
		result.fail("githubCredentials", "GitHub rejected the client credentials")
		return
	}
	result.succeed("githubCredentials", "GitHub accepted the client credentials")
}

func (r *Reconciler) checkGitlab(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.Gitlab == nil {
		result.fail("gitlabURL", "GitLab provider config is missing")
		return
	}

	baseURL := strings.TrimSpace(provider.Spec.Gitlab.BaseURL)
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	checkCABundle(result, "gitlabCABundle", provider.Spec.Gitlab.RootCAData)
	checkTLSCertificate(ctx, result, "gitlabCertificate", baseURL, provider.Spec.Gitlab.RootCAData, false)
	r.checkHTTPReachability(ctx, result, "gitlabURL", baseURL, provider.Spec.Gitlab.RootCAData)
	r.checkGitlabCredentials(ctx, result, provider.Spec.Gitlab, baseURL)
}

func (r *Reconciler) checkGitlabCredentials(ctx context.Context, result *dexProviderCheckResult, cfg *DexProviderGitlabForCheck, baseURL string) {
	tokenURL := strings.TrimRight(baseURL, "/") + "/oauth/token"
	client, err := r.http.New(TLSOptions{RootCAData: cfg.RootCAData})
	if err != nil {
		result.fail("gitlabCredentials", "cannot build HTTP client: %v", err)
		return
	}
	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "gitlabCredentials",
		providerName: "GitLab",
		tokenURL:     tokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func (r *Reconciler) checkBitbucket(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	if provider.Spec.BitbucketCloud == nil {
		result.fail("bitbucketAPI", "Bitbucket Cloud provider config is missing")
		return
	}
	if provider.Spec.BitbucketCloud.ClientID == "" {
		result.fail("bitbucketAPI", "Bitbucket Cloud clientID is empty")
		return
	}

	r.checkHTTPReachability(ctx, result, "bitbucketAPI", "https://api.bitbucket.org/2.0/", "")
	r.checkBitbucketCredentials(ctx, result, provider.Spec.BitbucketCloud)
}

func (r *Reconciler) checkBitbucketCredentials(ctx context.Context, result *dexProviderCheckResult, cfg *DexProviderBitbucketForCheck) {
	const tokenURL = "https://bitbucket.org/site/oauth2/access_token"
	client, err := r.http.New(TLSOptions{})
	if err != nil {
		result.fail("bitbucketCredentials", "cannot build HTTP client: %v", err)
		return
	}
	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "bitbucketCredentials",
		providerName: "Bitbucket",
		tokenURL:     tokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func (r *Reconciler) checkCrowd(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
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
	client, err := r.http.New(TLSOptions{})
	if err != nil {
		result.fail("crowdAPI", "cannot build HTTP client: %v", err)
		return
	}
	statusCode, _, err := httpGet(ctx, client, endpoint, provider.Spec.Crowd.ClientID, provider.Spec.Crowd.ClientSecret)
	if err != nil {
		result.failUnreachable("crowdAPI", err, "Crowd API is not reachable: %v", err)
		return
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		result.fail("crowdAPI", "Crowd API returned HTTP %d", statusCode)
		return
	}
	result.succeed("crowdAPI", "Crowd API accepted application credentials")
}

func (r *Reconciler) checkOIDC(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
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
	checkTLSCertificate(ctx, result, "oidcCertificate", issuer, provider.Spec.OIDC.RootCAData, provider.Spec.OIDC.InsecureSkipVerify)

	client, err := r.http.New(TLSOptions{
		RootCAData:         provider.Spec.OIDC.RootCAData,
		InsecureSkipVerify: provider.Spec.OIDC.InsecureSkipVerify,
	})
	if err != nil {
		result.fail("oidcDiscovery", "cannot build HTTP client: %v", err)
		return
	}
	discoveryURL := issuer + "/.well-known/openid-configuration"
	statusCode, body, err := httpGet(ctx, client, discoveryURL, "", "")
	if err != nil {
		result.failUnreachable("oidcDiscovery", err, "OIDC discovery is not reachable: %v", err)
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

	r.checkOIDCCredentials(ctx, client, result, provider.Spec.OIDC, discovery)

	statusCode, body, err = httpGet(ctx, client, discovery.JWKSURI, "", "")
	if err != nil {
		result.failUnreachable("oidcJWKS", err, "OIDC JWKS is not reachable: %v", err)
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

func (r *Reconciler) checkOIDCCredentials(
	ctx context.Context,
	client HTTPDoer,
	result *dexProviderCheckResult,
	cfg *DexProviderOIDCForCheck,
	discovery oidcDiscoveryDocument,
) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		result.skip("oidcCredentials", "clientID or clientSecret is empty")
		return
	}

	if discovery.IntrospectionEndpoint != "" &&
		probeIntrospection(ctx, client, result, discovery.IntrospectionEndpoint, cfg.ClientID, cfg.ClientSecret) {
		return
	}

	if discovery.TokenEndpoint == "" {
		result.skip("oidcCredentials", "OIDC discovery does not advertise a token endpoint")
		return
	}

	reportOAuthClientSecret(ctx, client, result, oauthClientSecretCheck{
		stepName:     "oidcCredentials",
		providerName: "OIDC",
		tokenURL:     discovery.TokenEndpoint,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	})
}

func probeIntrospection(
	ctx context.Context,
	client HTTPDoer,
	result *dexProviderCheckResult,
	introspectionURL, clientID, clientSecret string,
) bool {
	form := url.Values{}
	form.Set("token", "deckhouse-dex-provider-check")
	form.Set("token_type_hint", "access_token")

	statusCode, body, err := httpPostForm(ctx, client, introspectionURL, clientID, clientSecret, form)
	if err != nil {
		return false
	}

	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		result.succeed("oidcCredentials", "OIDC client credentials are valid (verified via token introspection)")
		return true
	}

	errCode := oauthErrorCode(body)
	if statusCode == http.StatusUnauthorized || errCode == "invalid_client" {
		detail := errCode
		if detail == "" {
			detail = fmt.Sprintf("HTTP %d", statusCode)
		}
		result.fail("oidcCredentials", "OIDC rejected the client credentials (%s)", detail)
		return true
	}

	return false
}

func (r *Reconciler) checkSAML(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
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
	checkTLSCertificate(ctx, result, "samlCertificate", provider.Spec.SAML.SSOURL, provider.Spec.SAML.RootCAData, false)
	r.checkHTTPReachability(ctx, result, "samlSSOURL", provider.Spec.SAML.SSOURL, provider.Spec.SAML.RootCAData)
}

type oidcDiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	JWKSURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	IntrospectionEndpoint string `json:"introspection_endpoint"`
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
