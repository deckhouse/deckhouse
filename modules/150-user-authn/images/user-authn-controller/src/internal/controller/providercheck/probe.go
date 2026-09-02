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
	"net/http"
)

func (r *Reconciler) execute(ctx context.Context, check *DexProviderCheck, provider DexProviderForCheck) DexProviderCheckStatus {
	result := &dexProviderCheckResult{
		checks: make([]DexProviderCheckStepStatus, 0, 4),
		now:    r.now(),
	}
	if provider.Name == "" {
		result.fail("providerExists", "DexProvider %q not found", check.Spec.ProviderName)
		return result.status(0)
	}
	result.succeed("providerExists", "DexProvider %q found", provider.Name)
	result.acknowledgeAllWarnings, result.acknowledgedWarnings = parseAcknowledgedWarnings(provider.Annotations)

	if provider.Spec.Enabled != nil && !*provider.Spec.Enabled {
		result.fail("providerEnabled", "DexProvider %q is disabled", provider.Name)
		return result.status(provider.Generation)
	}
	result.succeed("providerEnabled", "DexProvider %q is enabled", provider.Name)

	r.checkDexReachability(ctx, result)
	r.checkProviderReachability(ctx, result, provider)

	return result.status(provider.Generation)
}

func (r *Reconciler) checkDexReachability(ctx context.Context, result *dexProviderCheckResult) {
	client, err := r.http.New(TLSOptions{InsecureSkipVerify: true})
	if err != nil {
		result.fail("dexReady", "cannot build HTTP client: %v", err)
		return
	}
	statusCode, body, err := httpGet(ctx, client, dexDiscoveryURL, "", "")
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
	if err := unmarshalJSON(body, &discovery); err != nil {
		result.fail("dexReady", "Dex discovery returned invalid JSON: %v", err)
		return
	}
	if discovery.Issuer == "" {
		result.fail("dexReady", "Dex discovery response has empty issuer")
		return
	}
	result.succeed("dexReady", "Dex discovery is reachable")
}

func (r *Reconciler) checkProviderReachability(ctx context.Context, result *dexProviderCheckResult, provider DexProviderForCheck) {
	switch provider.Spec.Type {
	case "Github":
		r.checkGithub(ctx, result, provider)
	case "Gitlab":
		r.checkGitlab(ctx, result, provider)
	case "BitbucketCloud":
		r.checkBitbucket(ctx, result, provider)
	case "Crowd":
		r.checkCrowd(ctx, result, provider)
	case "OIDC":
		r.checkOIDC(ctx, result, provider)
	case "LDAP":
		r.checkLDAP(ctx, result, provider)
	case "SAML":
		r.checkSAML(ctx, result, provider)
	default:
		result.fail("providerConfig", "unsupported DexProvider type %q", provider.Spec.Type)
	}
}
