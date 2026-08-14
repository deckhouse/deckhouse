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
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"user-authn-controller/internal/controller"
)

const (
	recheckInterval      = time.Hour
	checkTimeout         = time.Minute
	httpTimeout          = 15 * time.Second
	ldapTimeout          = 15 * time.Second
	certExpiryWarnWindow = 14 * 24 * time.Hour

	dexDiscoveryURL = "https://dex.d8-user-authn/.well-known/openid-configuration"

	dexProviderAPIVersion = "deckhouse.io/v1"
	dexProviderKind       = "DexProvider"
	dexProviderCheckKind  = "DexProviderCheck"

	// DexProviderCheckPhasePending is set when the controller starts a check,
	// before connectivity probes run.
	DexProviderCheckPhasePending   DexProviderCheckPhase = "Pending"
	DexProviderCheckPhaseSucceeded DexProviderCheckPhase = "Succeeded"
	DexProviderCheckPhaseFailed    DexProviderCheckPhase = "Failed"

	pendingMessage = "connectivity check is running"

	stepSucceeded = "Succeeded"
	stepFailed    = "Failed"
	stepSkipped   = "Skipped"
	stepWarning   = "Warning"

	// acknowledgedWarningsAnnotation is a comma-separated list of warning step
	// names, or "*", recorded as Succeeded instead of Warning.
	acknowledgedWarningsAnnotation = "dexprovider.deckhouse.io/acknowledged-warnings"

	maxResponseBody = 1 << 20
)

var (
	dexProviderCheckGV = schema.GroupVersion{
		Group:   "deckhouse.io",
		Version: "v1",
	}
)

// HTTPDoer is the subset of *http.Client used by connectivity probes.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// TLSOptions configures the HTTP client for a single probe.
type TLSOptions struct {
	RootCAData         string
	InsecureSkipVerify bool
}

// HTTPFactory builds an HTTP client for a probe's TLS settings.
type HTTPFactory interface {
	New(opts TLSOptions) (HTTPDoer, error)
}

// LDAPConn is an opened LDAP connection used for bind and certificate checks.
type LDAPConn interface {
	Close() error
	Bind(bindDN, bindPW string) error
	TLSConnectionState() (tls.ConnectionState, bool)
}

// LDAPDialer opens an LDAP connection honouring the provider's TLS settings.
type LDAPDialer interface {
	Dial(ctx context.Context, cfg *DexProviderLDAPForCheck) (LDAPConn, error)
}

// DexProviderCheck is the cluster-scoped connectivity check for a DexProvider.
type DexProviderCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexProviderCheckSpec   `json:"spec"`
	Status            DexProviderCheckStatus `json:"status"`
}

// DexProviderCheckSpec identifies the DexProvider to probe.
type DexProviderCheckSpec struct {
	ProviderName  string `json:"providerName"`
	InitiatorType string `json:"initiatorType,omitempty"`
}

// DexProviderCheckStatus is the observed probe result.
type DexProviderCheckStatus struct {
	Phase                         DexProviderCheckPhase        `json:"phase"`
	Message                       string                       `json:"message,omitempty"`
	ObservedDexProviderGeneration int64                        `json:"observedDexProviderGeneration,omitempty"`
	Checks                        []DexProviderCheckStepStatus `json:"checks,omitempty"`
	CompletedAt                   *metav1.Time                 `json:"completedAt,omitempty"`
}

// DexProviderCheckPhase is the overall check phase.
type DexProviderCheckPhase string

// DexProviderCheckStepStatus is one probe step.
type DexProviderCheckStepStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// DexProviderCheckList contains a list of DexProviderCheck.
type DexProviderCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DexProviderCheck `json:"items"`
}

// DexProviderForCheck is the DexProvider snapshot used by connectivity probes.
type DexProviderForCheck struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexProviderForCheckSpec `json:"spec"`
}

// DexProviderForCheckSpec is the subset of DexProvider spec needed for probes.
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

// DexProviderGithubForCheck is the GitHub connector snapshot.
type DexProviderGithubForCheck struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// DexProviderGitlabForCheck is the GitLab connector snapshot.
type DexProviderGitlabForCheck struct {
	ClientID     string `json:"clientID,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	BaseURL      string `json:"baseURL,omitempty"`
	RootCAData   string `json:"rootCAData,omitempty"`
}

// DexProviderBitbucketForCheck is the Bitbucket Cloud connector snapshot.
type DexProviderBitbucketForCheck struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// DexProviderCrowdForCheck is the Crowd connector snapshot.
type DexProviderCrowdForCheck struct {
	BaseURL      string `json:"baseURL"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

// DexProviderOIDCForCheck is the OIDC connector snapshot.
type DexProviderOIDCForCheck struct {
	ClientID           string `json:"clientID,omitempty"`
	ClientSecret       string `json:"clientSecret,omitempty"`
	Issuer             string `json:"issuer"`
	RootCAData         string `json:"rootCAData,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

// DexProviderLDAPForCheck is the LDAP connector snapshot.
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

// DexProviderLDAPKerberosForCheck is the LDAP Kerberos snapshot.
type DexProviderLDAPKerberosForCheck struct {
	Enabled          bool   `json:"enabled,omitempty"`
	KeytabSecretName string `json:"keytabSecretName,omitempty"`
}

// DexProviderSAMLForCheck is the SAML connector snapshot.
type DexProviderSAMLForCheck struct {
	SSOURL     string `json:"ssoURL"`
	RootCAData string `json:"rootCAData,omitempty"`
}

// AddToScheme registers DexProviderCheck types on s so the manager can watch them.
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(dexProviderCheckGV, &DexProviderCheck{}, &DexProviderCheckList{})
	metav1.AddToGroupVersion(s, dexProviderCheckGV)
	return nil
}

func decodeProvider(obj *unstructured.Unstructured) (DexProviderForCheck, error) {
	var provider DexProviderForCheck
	if err := controller.DecodeInto(obj, &provider); err != nil {
		return DexProviderForCheck{}, fmt.Errorf("decode dexprovider: %w", err)
	}
	return provider, nil
}
