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
	"errors"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckProviderMissingConfig(t *testing.T) {
	t.Parallel()

	r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{}, now: time.Now}
	tests := []struct {
		name     string
		provider DexProviderForCheck
		wantStep string
	}{
		{
			name:     "github missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "Github"}},
			wantStep: "githubAPI",
		},
		{
			name: "github empty clientID",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{
				Type:   "Github",
				Github: &DexProviderGithubForCheck{},
			}},
			wantStep: "githubAPI",
		},
		{
			name:     "gitlab missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "Gitlab"}},
			wantStep: "gitlabURL",
		},
		{
			name:     "bitbucket missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "BitbucketCloud"}},
			wantStep: "bitbucketAPI",
		},
		{
			name:     "crowd missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "Crowd"}},
			wantStep: "crowdAPI",
		},
		{
			name: "crowd empty baseURL",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{
				Type:  "Crowd",
				Crowd: &DexProviderCrowdForCheck{ClientID: "id", ClientSecret: "s"},
			}},
			wantStep: "crowdAPI",
		},
		{
			name:     "oidc missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "OIDC"}},
			wantStep: "oidcDiscovery",
		},
		{
			name: "oidc empty issuer",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{
				Type: "OIDC",
				OIDC: &DexProviderOIDCForCheck{},
			}},
			wantStep: "oidcDiscovery",
		},
		{
			name:     "saml missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "SAML"}},
			wantStep: "samlSSOURL",
		},
		{
			name: "saml empty ssoURL",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{
				Type: "SAML",
				SAML: &DexProviderSAMLForCheck{},
			}},
			wantStep: "samlSSOURL",
		},
		{
			name:     "unsupported type",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "Unknown"}},
			wantStep: "providerConfig",
		},
		{
			name:     "ldap missing spec",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{Type: "LDAP"}},
			wantStep: "ldapReachable",
		},
		{
			name: "ldap empty host",
			provider: DexProviderForCheck{Spec: DexProviderForCheckSpec{
				Type: "LDAP",
				LDAP: &DexProviderLDAPForCheck{},
			}},
			wantStep: "ldapReachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &dexProviderCheckResult{now: time.Now()}
			r.checkProviderReachability(t.Context(), result, tt.provider)
			if len(result.checks) == 0 {
				t.Fatal("no checks recorded")
			}
			if result.checks[0].Name != tt.wantStep || result.checks[0].Status != stepFailed {
				t.Fatalf("first check = %+v, want %s/%s", result.checks[0], tt.wantStep, stepFailed)
			}
		})
	}
}

func TestCheckGithubReachableSkipsEmptySecret(t *testing.T) {
	t.Parallel()

	httpClient := newFakeHTTP()
	httpClient.set("https://api.github.com/meta", http.StatusOK, `{}`)
	r := &Reconciler{http: httpClient, ldap: &fakeLDAP{}, now: time.Now}
	result := &dexProviderCheckResult{now: time.Now()}
	r.checkGithub(t.Context(), result, DexProviderForCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "github"},
		Spec: DexProviderForCheckSpec{
			Type:   "Github",
			Github: &DexProviderGithubForCheck{ClientID: "id"},
		},
	})
	assertResultStep(t, result, "githubAPI", stepSucceeded)
	assertResultStep(t, result, "githubCredentials", stepSkipped)
}

func TestCheckCrowdAPI(t *testing.T) {
	t.Parallel()

	httpClient := newFakeHTTP()
	httpClient.set("https://crowd.example.com/rest/usermanagement/1/config/cookie", http.StatusOK, `{}`)
	r := &Reconciler{http: httpClient, ldap: &fakeLDAP{}, now: time.Now}
	result := &dexProviderCheckResult{now: time.Now()}
	r.checkCrowd(t.Context(), result, DexProviderForCheck{
		Spec: DexProviderForCheckSpec{
			Type: "Crowd",
			Crowd: &DexProviderCrowdForCheck{
				BaseURL:      "https://crowd.example.com",
				ClientID:     "app",
				ClientSecret: "secret",
			},
		},
	})
	assertResultStep(t, result, "crowdAPI", stepSucceeded)
}

func TestCheckLDAPBindAndKerberos(t *testing.T) {
	t.Parallel()

	t.Run("bind success anonymous skip", func(t *testing.T) {
		t.Parallel()
		r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{hasTLS: true}, now: time.Now}
		result := &dexProviderCheckResult{now: time.Now()}
		r.checkLDAP(t.Context(), result, DexProviderForCheck{
			Spec: DexProviderForCheckSpec{
				Type: "LDAP",
				LDAP: &DexProviderLDAPForCheck{Host: "ldap.example.com:636"},
			},
		})
		assertResultStep(t, result, "ldapReachable", stepSucceeded)
		assertResultStep(t, result, "ldapBind", stepSkipped)
		assertResultStep(t, result, "ldapKerberosKeytab", stepSkipped)
	})

	t.Run("bind failure", func(t *testing.T) {
		t.Parallel()
		r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{bindErr: errors.New("invalid credentials")}, now: time.Now}
		result := &dexProviderCheckResult{now: time.Now()}
		r.checkLDAP(t.Context(), result, DexProviderForCheck{
			Spec: DexProviderForCheckSpec{
				Type: "LDAP",
				LDAP: &DexProviderLDAPForCheck{
					Host:   "ldap.example.com",
					BindDN: "cn=admin",
					BindPW: "pw",
				},
			},
		})
		assertResultStep(t, result, "ldapBind", stepFailed)
	})

	t.Run("kerberos missing secret name", func(t *testing.T) {
		t.Parallel()
		r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{}, now: time.Now}
		result := &dexProviderCheckResult{now: time.Now()}
		r.checkLDAP(t.Context(), result, DexProviderForCheck{
			Spec: DexProviderForCheckSpec{
				Type: "LDAP",
				LDAP: &DexProviderLDAPForCheck{
					Host:     "ldap.example.com",
					Kerberos: &DexProviderLDAPKerberosForCheck{Enabled: true},
				},
			},
		})
		assertResultStep(t, result, "ldapKerberosKeytab", stepFailed)
	})
}

func TestCheckHTTPReachabilityStatusClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{name: "2xx", status: http.StatusOK, want: stepSucceeded},
		{name: "4xx still reachable", status: http.StatusUnauthorized, want: stepSucceeded},
		{name: "5xx", status: http.StatusBadGateway, want: stepFailed},
		{name: "transport error", err: errors.New("dial"), want: stepFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			httpClient := newFakeHTTP()
			if tt.err != nil {
				httpClient.setErr("https://idp.example.com/health", tt.err)
			} else {
				httpClient.set("https://idp.example.com/health", tt.status, `{}`)
			}
			r := &Reconciler{http: httpClient, now: time.Now}
			result := &dexProviderCheckResult{now: time.Now()}
			r.checkHTTPReachability(t.Context(), result, "probe", "https://idp.example.com/health", "")
			assertResultStep(t, result, "probe", tt.want)
		})
	}
}

func TestOAuthErrorCode(t *testing.T) {
	t.Parallel()

	if got := oauthErrorCode([]byte(`{"error":"invalid_client"}`)); got != "invalid_client" {
		t.Errorf("got %q", got)
	}
	if got := oauthErrorCode([]byte(`not-json`)); got != "" {
		t.Errorf("unparseable = %q, want empty", got)
	}
}

func TestReportOAuthClientSecret(t *testing.T) {
	t.Parallel()

	t.Run("empty credentials skipped", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{now: time.Now()}
		reportOAuthClientSecret(t.Context(), newFakeHTTP(), result, oauthClientSecretCheck{
			stepName:     "creds",
			providerName: "GitLab",
			tokenURL:     "https://idp.example.com/token",
		})
		assertResultStep(t, result, "creds", stepSkipped)
	})

	t.Run("accepted token", func(t *testing.T) {
		t.Parallel()
		httpClient := newFakeHTTP()
		httpClient.set("https://idp.example.com/token", http.StatusOK, `{"access_token":"x"}`)
		result := &dexProviderCheckResult{now: time.Now()}
		reportOAuthClientSecret(t.Context(), httpClient, result, oauthClientSecretCheck{
			stepName:     "creds",
			providerName: "GitLab",
			tokenURL:     "https://idp.example.com/token",
			clientID:     "id",
			clientSecret: "secret",
		})
		assertResultStep(t, result, "creds", stepSucceeded)
	})

	t.Run("rejected client", func(t *testing.T) {
		t.Parallel()
		httpClient := newFakeHTTP()
		httpClient.set("https://idp.example.com/token", http.StatusUnauthorized, `{"error":"invalid_client"}`)
		result := &dexProviderCheckResult{now: time.Now()}
		reportOAuthClientSecret(t.Context(), httpClient, result, oauthClientSecretCheck{
			stepName:     "creds",
			providerName: "Bitbucket",
			tokenURL:     "https://idp.example.com/token",
			clientID:     "id",
			clientSecret: "bad",
		})
		assertResultStep(t, result, "creds", stepFailed)
	})
}

func assertResultStep(t *testing.T, result *dexProviderCheckResult, name, want string) {
	t.Helper()
	for _, step := range result.checks {
		if step.Name == name {
			if step.Status != want {
				t.Fatalf("step %q status = %q, want %q (%#v)", name, step.Status, want, result.checks)
			}
			return
		}
	}
	t.Fatalf("missing step %q in %#v", name, result.checks)
}
