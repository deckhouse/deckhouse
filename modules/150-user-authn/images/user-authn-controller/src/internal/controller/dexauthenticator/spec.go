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
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// defaultKeepUsersLoggedInFor is spec.keepUsersLoggedInFor of an object that does not set it.
	defaultKeepUsersLoggedInFor = "168h"

	labelHeritage = "heritage"
	heritageValue = "deckhouse"
)

// ListenerSet is the ListenerSet of the Gateway API the route of an application is attached to.
type ListenerSet struct {
	Name        string
	Namespace   string
	SectionName string
}

// Application is one application of a DexAuthenticator: a host and how requests get to it.
type Application struct {
	Domain                string
	IngressClassName      string
	IngressSecretName     string
	SignOutURL            string
	WhitelistSourceRanges []string
	ListenerSet           ListenerSet
}

// Authenticator is a decoded DexAuthenticator.
type Authenticator struct {
	Name      string
	Namespace string
	UID       types.UID
	Created   time.Time
	// Generation is the generation of the object the fields were decoded from.
	Generation int64

	// Module tells an object of a Deckhouse module from one of a user; only the telemetry reads it.
	Module bool
	// AllowAccessToKubernetes puts the audience of the Kubernetes API into the ID tokens of the
	// object. The kubernetesclient reconciler puts its client into trustedPeers by the same
	// annotation.
	AllowAccessToKubernetes bool

	// Applications are the applications in the order of the object: the name of the route of every
	// application but the first carries a hash of its domain.
	Applications []Application

	SendAuthorizationHeader bool
	KeepUsersLoggedInFor    string
	AllowedEmails           []string
	AllowedGroups           []string
	AllowedProviders        []string
	HighAvailability        *bool
}

// Key names the object in the table of its instance, in the logs and in the messages.
func (a Authenticator) Key() types.NamespacedName {
	return types.NamespacedName{Namespace: a.Namespace, Name: a.Name}
}

// Instance is the shared dex-authenticator instance that serves the object.
func (a Authenticator) Instance() string {
	return naming.AuthenticatorInstance(a.Namespace)
}

// routesByHost reports whether an application of the object on host is published through the
// Gateway API. The request of an HTTPRoute reaches the instance with the host of the application
// and nothing that names the object, so the instance can choose the object by that host only.
func (a Authenticator) routesByHost(host string) bool {
	for _, app := range a.Applications {
		if app.ListenerSet.Name != "" && strings.EqualFold(app.Domain, host) {
			return true
		}
	}
	return false
}

// ingressClasses are the Ingress classes the applications of the object on host are published
// through.
func (a Authenticator) ingressClasses(host string) []string {
	var classes []string
	for _, app := range a.Applications {
		if app.IngressClassName != "" && strings.EqualFold(app.Domain, host) && !slices.Contains(classes, app.IngressClassName) {
			classes = append(classes, app.IngressClassName)
		}
	}
	return classes
}

// ClientID is the id of the Dex client of the object.
func (a Authenticator) ClientID() string {
	return naming.DexAuthenticatorClientID(a.Name, a.Namespace)
}

// listenerSetV1 is the gatewayAPI of an additional application, and of v2alpha1.
type listenerSetV1 struct {
	Name        string `json:"httpRouteListenerSetName"`
	Namespace   string `json:"httpRouteListenerSetNamespace"`
	SectionName string `json:"httpRouteListenerSetSectionName"`
}

// specV1 is the spec of the stored version of a DexAuthenticator, v1: the first application is
// in the spec itself, the others in additionalApplications. The reconcilers read the stored
// version, so that listing the objects does not depend on the conversion webhook.
type specV1 struct {
	ApplicationDomain                       string   `json:"applicationDomain"`
	ApplicationIngressClassName             string   `json:"applicationIngressClassName"`
	ApplicationIngressCertificateSecretName string   `json:"applicationIngressCertificateSecretName"`
	SignOutURL                              string   `json:"signOutURL"`
	WhitelistSourceRanges                   []string `json:"whitelistSourceRanges"`
	GatewayAPI                              struct {
		Name        string `json:"applicationHTTPRouteListenerSetName"`
		Namespace   string `json:"applicationHTTPRouteListenerSetNamespace"`
		SectionName string `json:"applicationHTTPRouteListenerSetSectionName"`
	} `json:"gatewayAPI"`
	AdditionalApplications []struct {
		Domain                string        `json:"domain"`
		IngressClassName      string        `json:"ingressClassName"`
		IngressSecretName     string        `json:"ingressSecretName"`
		SignOutURL            string        `json:"signOutURL"`
		WhitelistSourceRanges []string      `json:"whitelistSourceRanges"`
		GatewayAPI            listenerSetV1 `json:"gatewayAPI"`
	} `json:"additionalApplications"`

	SendAuthorizationHeader bool     `json:"sendAuthorizationHeader"`
	KeepUsersLoggedInFor    string   `json:"keepUsersLoggedInFor"`
	AllowedEmails           []string `json:"allowedEmails"`
	AllowedGroups           []string `json:"allowedGroups"`
	AllowedProviders        []string `json:"allowedProviders"`
	HighAvailability        *bool    `json:"highAvailability"`
}

// Decode reads a DexAuthenticator of the stored version.
func Decode(obj *unstructured.Unstructured) (Authenticator, error) {
	var decoded struct {
		Spec specV1 `json:"spec"`
	}
	if err := controller.DecodeInto(obj, &decoded); err != nil {
		return Authenticator{}, fmt.Errorf("decode DexAuthenticator %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	spec := decoded.Spec

	a := Authenticator{
		Name:                    obj.GetName(),
		Namespace:               obj.GetNamespace(),
		UID:                     obj.GetUID(),
		Created:                 obj.GetCreationTimestamp().Time,
		Generation:              obj.GetGeneration(),
		Module:                  obj.GetLabels()[labelHeritage] == heritageValue,
		SendAuthorizationHeader: spec.SendAuthorizationHeader,
		KeepUsersLoggedInFor:    spec.KeepUsersLoggedInFor,
		AllowedEmails:           spec.AllowedEmails,
		AllowedGroups:           spec.AllowedGroups,
		AllowedProviders:        spec.AllowedProviders,
		HighAvailability:        spec.HighAvailability,
	}
	// A value that is not a boolean gives no access: it must not hand out tokens kube-apiserver accepts.
	a.AllowAccessToKubernetes, _ = controller.AllowsAccessToKubernetes(obj.GetAnnotations(), controller.DexAuthenticatorAllowAccessToKubernetesAnnotation)
	if a.KeepUsersLoggedInFor == "" {
		a.KeepUsersLoggedInFor = defaultKeepUsersLoggedInFor
	}

	a.Applications = append(a.Applications, Application{
		Domain:                spec.ApplicationDomain,
		IngressClassName:      spec.ApplicationIngressClassName,
		IngressSecretName:     spec.ApplicationIngressCertificateSecretName,
		SignOutURL:            spec.SignOutURL,
		WhitelistSourceRanges: spec.WhitelistSourceRanges,
		ListenerSet: ListenerSet{
			Name:        spec.GatewayAPI.Name,
			Namespace:   spec.GatewayAPI.Namespace,
			SectionName: spec.GatewayAPI.SectionName,
		},
	})
	for _, app := range spec.AdditionalApplications {
		a.Applications = append(a.Applications, Application{
			Domain:                app.Domain,
			IngressClassName:      app.IngressClassName,
			IngressSecretName:     app.IngressSecretName,
			SignOutURL:            app.SignOutURL,
			WhitelistSourceRanges: app.WhitelistSourceRanges,
			ListenerSet:           ListenerSet(app.GatewayAPI),
		})
	}

	for i, app := range a.Applications {
		if app.Domain == "" {
			return Authenticator{}, fmt.Errorf("DexAuthenticator %s/%s: application %d has no domain", a.Namespace, a.Name, i)
		}
		if strings.ContainsAny(app.Domain, "/:@ ") {
			return Authenticator{}, fmt.Errorf("DexAuthenticator %s/%s: application %d: %q is not a DNS name", a.Namespace, a.Name, i, app.Domain)
		}
	}
	if _, err := time.ParseDuration(a.KeepUsersLoggedInFor); err != nil {
		return Authenticator{}, errors.Join(fmt.Errorf("DexAuthenticator %s/%s: keepUsersLoggedInFor", a.Namespace, a.Name), err)
	}

	return a, nil
}
