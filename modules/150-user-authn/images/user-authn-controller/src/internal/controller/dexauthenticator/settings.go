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

package dexauthenticator

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"user-authn-controller/internal/naming"
)

const (
	// SettingsConfigMapName is the ConfigMap of the module chart with the settings the objects of
	// a DexAuthenticator depend on, under SettingsKey as JSON. Without it the reconcilers do
	// nothing: the chart does not render the shared instances.
	SettingsConfigMapName = "shared-dex-authenticator"
	SettingsKey           = "settings.json"

	httpsModeCertManager       = "CertManager"
	httpsModeCustomCertificate = "CustomCertificate"

	defaultIDTokenTTL = 10 * time.Minute
)

// Settings are the settings of the module and of the cluster that the table records and the
// objects of every DexAuthenticator are built from. The chart used to read them from its values.
type Settings struct {
	// ClusterDomain is the DNS domain of the cluster, such as cluster.local.
	ClusterDomain string `json:"clusterDomain"`
	// HTTPSMode is the HTTPS mode of the module: CertManager, CustomCertificate, OnlyInURI or Disabled.
	HTTPSMode string `json:"httpsMode"`
	// IngressEnabled tells that the module publishes through Ingress objects.
	IngressEnabled bool `json:"ingressEnabled"`
	// IngressNginxHSTSAnnotation tells that the ingress-nginx controllers know the HSTS annotation
	// (annotations of version 1.3.0 and later); older ones take a configuration snippet.
	IngressNginxHSTSAnnotation bool `json:"ingressNginxHSTSAnnotation"`
	// GatewayAPI tells that the module publishes through the Gateway API: the applications that
	// name a ListenerSet get an HTTPRoute.
	GatewayAPI bool `json:"gatewayAPI"`
	// DexSessions tells that the sessions of Dex are on, and PlatformSSO that the objects of the
	// d8-* namespaces share one sign-in through them.
	DexSessions bool `json:"dexSessions"`
	PlatformSSO bool `json:"platformSSO"`
	// IDTokenTTL is the lifetime of the ID tokens of Dex, the shortest a session can live.
	IDTokenTTL string `json:"idTokenTTL"`
}

// ParseSettings reads the settings ConfigMap.
func ParseSettings(cm *corev1.ConfigMap) (Settings, error) {
	var settings Settings
	raw, ok := cm.Data[SettingsKey]
	if !ok {
		return Settings{}, fmt.Errorf("ConfigMap %s has no %s", cm.Name, SettingsKey)
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return Settings{}, fmt.Errorf("ConfigMap %s: %s: %w", cm.Name, SettingsKey, err)
	}
	if settings.ClusterDomain == "" {
		return Settings{}, fmt.Errorf("ConfigMap %s: %s: clusterDomain is empty", cm.Name, SettingsKey)
	}
	if settings.IDTokenTTL != "" {
		if ttl, err := time.ParseDuration(settings.IDTokenTTL); err != nil || ttl <= 0 {
			return Settings{}, fmt.Errorf("ConfigMap %s: %s: idTokenTTL %q is not a positive duration", cm.Name, SettingsKey, settings.IDTokenTTL)
		}
	}
	return settings, nil
}

// ingressTLS reports whether the Ingress objects of the module terminate TLS with a certificate.
func (s Settings) ingressTLS() bool {
	return s.HTTPSMode == httpsModeCertManager || s.HTTPSMode == httpsModeCustomCertificate
}

func (s Settings) idTokenTTL() time.Duration {
	if ttl, err := time.ParseDuration(s.IDTokenTTL); err == nil && ttl > 0 {
		return ttl
	}
	return defaultIDTokenTTL
}

// cookieExpire is how long the session cookie of an object lives: keepUsersLoggedInFor, but
// longer than the ID token by a second at least, since a session is refreshed once its ID token
// has expired.
func (s Settings) cookieExpire(keepUsersLoggedInFor string) string {
	ttl := s.idTokenTTL()
	if keep, err := time.ParseDuration(keepUsersLoggedInFor); err == nil && keep > ttl {
		return keepUsersLoggedInFor
	}
	return (ttl + time.Second).String()
}

// instanceServiceHost is the DNS name of the Service of an instance: the Service of every
// DexAuthenticator points at it.
func (s Settings) instanceServiceHost(instance string) string {
	return instance + "." + naming.DexNamespace + ".svc." + s.ClusterDomain
}

// serviceHost is the DNS name of the Service of an object, as the applications write it in
// auth-url: the Host header of a request the ingress controller sends through that Service.
func (s Settings) serviceHost(a Authenticator) string {
	return strings.ToLower(serviceName(a.Name).name + "." + a.Namespace + ".svc." + s.ClusterDomain)
}

// isInstanceHost reports whether host is a name of the Service of an instance. A check that
// comes straight to an instance carries such a name as its host, and it must not be given to the
// client of an application that took the name as its domain.
//
// Every DNS name of the Service is such a name: the Service alone, with its namespace, and then
// with "svc" and any domain, since the cluster may have aliases of its domain and a check may
// come by any of them.
func (s Settings) isInstanceHost(host string) bool {
	labels := strings.SplitN(strings.TrimSuffix(strings.ToLower(host), "."), ".", 4)
	if !slices.Contains(naming.AuthenticatorInstances(), labels[0]) {
		return false
	}
	switch len(labels) {
	case 1:
		return true
	case 2:
		return labels[1] == naming.DexNamespace
	default:
		return labels[1] == naming.DexNamespace && labels[2] == "svc"
	}
}
