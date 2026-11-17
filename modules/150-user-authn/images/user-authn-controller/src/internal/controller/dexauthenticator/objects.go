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
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// LabelFor and LabelNamespace name the DexAuthenticator on its objects: the first on all of
	// them, the second on the Dex client, which lives in the namespace of the module. The
	// revocation feed finds the clients of an instance by the second.
	LabelFor       = "deckhouse.io/dex-authenticator-for"
	LabelNamespace = "deckhouse.io/dex-authenticator-namespace"

	// ClientIDAnnotation names the Dex client on the Service and the routes of a DexAuthenticator.
	ClientIDAnnotation = "dexauthenticator.deckhouse.io/client-id"

	labelApp           = "app"
	labelModule        = "module"
	moduleValue        = "user-authn"
	appValue           = "dex-authenticator"
	clientAppValue     = "dex"
	labelNameTruncated = "deckhouse.io/name-truncated"
	labelNameHash      = "deckhouse.io/name-hash"

	proxyPrefix       = "/dex-authenticator"
	signOutPath       = proxyPrefix + "/sign_out"
	servicePort       = 443
	servicePortName   = "https"
	hstsSnippet       = `add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;`
	sessionLifetime   = "session"
	nginxAnnotation   = "nginx.ingress.kubernetes.io/"
	albAnnotation     = "alb.network.deckhouse.io/"
	albBackendTLS     = `{"mode": "SIMPLE", "insecureSkipVerify": true}`
	gatewayGroup      = "gateway.networking.k8s.io"
	listenerSetKind   = "ListenerSet"
	largeHeaderBuffer = "32k"
)

// upstreamVhostAnnotation makes ingress-nginx send its value to the backend as the Host header;
// the host of the application stays in X-Forwarded-Host.
const upstreamVhostAnnotation = nginxAnnotation + "upstream-vhost"

// hstsAnnotations are the annotations an Ingress of an object carries HSTS in, one of them at a
// time. Both are the controller's on an Ingress of an object, whoever wrote them: the chart of an
// earlier release always wrote the snippet, and an Ingress taken over from it must not keep the
// snippet next to the annotation that replaced it.
var hstsAnnotations = []string{nginxAnnotation + "configuration-snippet", nginxAnnotation + "ingress-nginx-hsts"}

// HTTPRouteGVK is the route of an application published through the Gateway API.
var HTTPRouteGVK = schema.GroupVersionKind{Group: gatewayGroup, Version: "v1", Kind: "HTTPRoute"}

// objectName is the name of an object named after a DexAuthenticator, with the labels that mark
// a name that had to be cut.
type objectName struct {
	name      string
	truncated bool
	hash      string
}

func safeName(full string) objectName {
	name, truncated, hash := naming.SafeObjectName(full)
	return objectName{name: name, truncated: truncated, hash: hash}
}

// labels are the labels of an object of the DexAuthenticator name in its namespace.
func (n objectName) labels(name string) map[string]string {
	out := map[string]string{
		labelHeritage: heritageValue,
		labelModule:   moduleValue,
		labelApp:      appValue,
		LabelFor:      name,
	}
	if n.truncated {
		out[labelNameTruncated] = "true"
		out[labelNameHash] = n.hash
	}
	return out
}

// serviceName is the name of the Service of a DexAuthenticator: applications and modules point
// their auth-url at it, so it is the one the module chart gave it.
func serviceName(name string) objectName {
	return safeName(name + "-dex-authenticator")
}

// routeName is the name of the Ingress and of the HTTPRoute of the application number index. The
// route of every application but the first carries a hash of its domain.
func routeName(name string, index int, domain string) objectName {
	return safeName(name + routeSuffix(index, domain) + "-dex-authenticator")
}

// signOutRouteName is the name of the Ingress of the sign-out address of an application.
func signOutRouteName(name string, index int, domain string) objectName {
	return safeName(name + routeSuffix(index, domain) + "-dex-authenticator-sign-out")
}

func routeSuffix(index int, domain string) string {
	if index == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(domain))
	return "-" + hex.EncodeToString(sum[:])[:8]
}

// served reports whether the object is served on the domain: only then routes and the addresses
// of the Dex client are made for it.
func served(record *Record, domain string) bool {
	return record != nil && slices.Contains(record.applications(), strings.ToLower(domain))
}

// clientMetadata is the metadata of the Dex client of an object.
func clientMetadata(a Authenticator) adoption.Metadata {
	return adoption.Metadata{
		Fixed: map[string]string{
			labelHeritage:  heritageValue,
			labelModule:    moduleValue,
			labelApp:       clientAppValue,
			LabelFor:       a.Name,
			LabelNamespace: a.Namespace,
		},
		Track: true,
	}
}

// clientFields are the top-level fields of an OAuth2Client the controller owns.
var clientFields = []string{
	"id", "name", "secret", "allowedEmails", "allowedGroups", "allowedConnectors", "redirectURIs",
	"postLogoutRedirectURIs", "refreshTokenLifetime", "ssoSharedWith",
}

// clientContent is the content of the Dex client of an object. peers are the clients the object
// shares the sign-in with.
func clientContent(a Authenticator, record Record, settings Settings, peers []string) map[string]any {
	out := map[string]any{
		"id":     record.ClientID,
		"name":   record.ClientID,
		"secret": record.ClientSecret,
	}

	var redirects, postLogout []string
	for _, app := range a.Applications {
		if !served(&record, app.Domain) {
			continue
		}
		redirect := "https://" + app.Domain + proxyPrefix + "/callback"
		if slices.Contains(redirects, redirect) {
			// Two applications of the object publish one domain, each through its Ingress class.
			continue
		}
		redirects = append(redirects, redirect)
		// Sign-out through Dex returns to the root of the application the user signed out of.
		postLogout = append(postLogout, "https://"+app.Domain+"/")
	}

	lists := map[string][]string{
		"allowedEmails": a.AllowedEmails,
		"allowedGroups": a.AllowedGroups,
		// The providers of the sign-in: Dex calls them connectors, and a connector ID is the name of
		// its DexProvider.
		"allowedConnectors":      a.AllowedProviders,
		"redirectURIs":           redirects,
		"postLogoutRedirectURIs": postLogout,
	}
	if settings.DexSessions {
		// The session of the application ends with the session of Dex it signed in through.
		out["refreshTokenLifetime"] = sessionLifetime
		lists["ssoSharedWith"] = peers
	}
	for key, values := range lists {
		if len(values) == 0 {
			continue
		}
		items := make([]any, 0, len(values))
		for _, v := range values {
			items = append(items, v)
		}
		out[key] = items
	}
	return out
}

// sharesSSO reports whether the objects of namespace share one sign-in through the sessions of
// Dex with the other web interfaces of the platform.
func sharesSSO(settings Settings, namespace string) bool {
	return settings.DexSessions && settings.PlatformSSO && naming.IsSystemNamespace(namespace)
}

// desiredClient is the Dex client of an object.
func desiredClient(a Authenticator, record Record, settings Settings, peers []string) *unstructured.Unstructured {
	obj := controller.Object(controller.OAuth2ClientGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(naming.ToFnvLikeDex(record.ClientID))
	clientMetadata(a).New(obj)
	for key, value := range clientContent(a, record, settings, peers) {
		obj.Object[key] = value
	}
	return obj
}

func objectMetadata(labels, annotations map[string]string) adoption.Metadata {
	return adoption.Metadata{Fixed: labels, Annotations: annotations, Track: true}
}

// desiredService is the Service of an object: a name in the namespace of the object for the
// Service of its instance, which the cluster keeps pointed at the pods of the instance.
func desiredService(a Authenticator, settings Settings) (*corev1.Service, adoption.Metadata) {
	name := serviceName(a.Name)
	meta := objectMetadata(name.labels(a.Name), map[string]string{ClientIDAnnotation: a.ClientID()})

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name.name, Namespace: a.Namespace},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: settings.instanceServiceHost(a.Instance()),
			Ports: []corev1.ServicePort{{
				Name:       servicePortName,
				Port:       servicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(servicePort),
			}},
		},
	}
	meta.New(svc)
	return svc, meta
}

// route is an Ingress of an object with its metadata.
type route struct {
	ingress *networkingv1.Ingress
	meta    adoption.Metadata
}

// desiredIngresses are the Ingress objects of an object: for every application that names an
// Ingress class, the one that leads the proxy prefix to the instance, and the one of the sign-out
// address of the application, if it has one. The Ingress objects of an application whose host is
// one of throughService send the name of the Service of the object as the Host header.
func desiredIngresses(a Authenticator, record *Record, throughService []string, settings Settings) []route {
	if !settings.IngressEnabled {
		return nil
	}

	var routes []route
	for index, app := range a.Applications {
		if app.IngressClassName == "" || !served(record, app.Domain) {
			continue
		}

		viaService := slices.Contains(throughService, strings.ToLower(app.Domain))
		annotations := ingressAnnotations(a, settings, viaService)
		if a.SendAuthorizationHeader {
			// The ID token in the Authorization header does not fit the default buffer.
			annotations[nginxAnnotation+"proxy-buffer-size"] = largeHeaderBuffer
		}
		if len(app.WhitelistSourceRanges) > 0 {
			annotations[nginxAnnotation+"whitelist-source-range"] = strings.Join(app.WhitelistSourceRanges, ",")
		}
		routes = append(routes, newIngress(a, app, routeName(a.Name, index, app.Domain), annotations, proxyPrefix, settings))

		if app.SignOutURL != "" {
			annotations := ingressAnnotations(a, settings, viaService)
			annotations[nginxAnnotation+"rewrite-target"] = signOutPath
			routes = append(routes, newIngress(a, app, signOutRouteName(a.Name, index, app.Domain), annotations, app.SignOutURL, settings))
		}
	}
	return routes
}

// ingressAnnotations are the annotations every Ingress of an object carries. With viaService the
// ingress controller sends the name of the Service of the object as the Host header.
func ingressAnnotations(a Authenticator, settings Settings, viaService bool) map[string]string {
	annotations := map[string]string{
		ClientIDAnnotation:                   a.ClientID(),
		nginxAnnotation + "backend-protocol": "HTTPS",
	}
	if viaService {
		annotations[upstreamVhostAnnotation] = settings.serviceHost(a)
	}
	switch {
	case !settings.IngressNginxHSTSAnnotation:
		snippet := ""
		if settings.ingressTLS() {
			snippet = hstsSnippet
		}
		annotations[nginxAnnotation+"configuration-snippet"] = snippet
	case settings.ingressTLS():
		annotations[nginxAnnotation+"ingress-nginx-hsts"] = "true"
	}
	return annotations
}

func newIngress(a Authenticator, app Application, name objectName, annotations map[string]string, path string, settings Settings) route {
	meta := objectMetadata(name.labels(a.Name), annotations)
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name.name, Namespace: a.Namespace},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(app.IngressClassName),
			Rules: []networkingv1.IngressRule{{
				Host: app.Domain,
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path:     path,
						PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
							Name: serviceName(a.Name).name,
							Port: networkingv1.ServiceBackendPort{Number: servicePort},
						}},
					}},
				}},
			}},
		},
	}
	if settings.ingressTLS() && app.IngressSecretName != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{Hosts: []string{app.Domain}, SecretName: app.IngressSecretName}}
	}
	meta.New(ingress)
	return route{ingress: ingress, meta: meta}
}

// httpRoute is an HTTPRoute of an object with its metadata.
type httpRoute struct {
	object *unstructured.Unstructured
	meta   adoption.Metadata
}

// desiredHTTPRoutes are the HTTPRoute objects of an object: one for every application that names
// a ListenerSet, with the proxy prefix and the sign-out address of the application.
func desiredHTTPRoutes(a Authenticator, record *Record, settings Settings) []httpRoute {
	if !settings.GatewayAPI {
		return nil
	}

	var routes []httpRoute
	for index, app := range a.Applications {
		if app.ListenerSet.Name == "" || !served(record, app.Domain) {
			continue
		}

		name := routeName(a.Name, index, app.Domain)
		annotations := map[string]string{
			ClientIDAnnotation:                     a.ClientID(),
			albAnnotation + "backend-tls-settings": albBackendTLS,
		}
		if len(app.WhitelistSourceRanges) > 0 {
			annotations[albAnnotation+"whitelist-source-range"] = strings.Join(app.WhitelistSourceRanges, ",")
		}
		meta := objectMetadata(name.labels(a.Name), annotations)

		obj := controller.Object(HTTPRouteGVK)
		obj.SetNamespace(a.Namespace)
		obj.SetName(name.name)
		meta.New(obj)
		obj.Object["spec"] = httpRouteSpec(a, app)
		routes = append(routes, httpRoute{object: obj, meta: meta})
	}
	return routes
}

func httpRouteSpec(a Authenticator, app Application) map[string]any {
	parent := map[string]any{
		"group":     gatewayGroup,
		"kind":      listenerSetKind,
		"name":      app.ListenerSet.Name,
		"namespace": a.Namespace,
	}
	if app.ListenerSet.Namespace != "" {
		parent["namespace"] = app.ListenerSet.Namespace
	}
	if app.ListenerSet.SectionName != "" {
		parent["sectionName"] = app.ListenerSet.SectionName
	}

	backend := func() []any {
		return []any{map[string]any{
			"group":  "",
			"kind":   "Service",
			"name":   serviceName(a.Name).name,
			"port":   int64(servicePort),
			"weight": int64(1),
		}}
	}
	match := func(path string) []any {
		return []any{map[string]any{"path": map[string]any{"type": "PathPrefix", "value": path}}}
	}

	rules := []any{map[string]any{"matches": match(proxyPrefix), "backendRefs": backend()}}
	if app.SignOutURL != "" {
		rules = append(rules, map[string]any{
			"matches":     match(app.SignOutURL),
			"backendRefs": backend(),
			"filters": []any{map[string]any{
				"type": "URLRewrite",
				"urlRewrite": map[string]any{
					"path": map[string]any{"type": "ReplaceFullPath", "replaceFullPath": signOutPath},
				},
			}},
		})
	}

	return map[string]any{
		"hostnames":  []any{app.Domain},
		"parentRefs": []any{parent},
		"rules":      rules,
	}
}

// ssoPeers are the clients the object a shares the sign-in with: with the sessions of Dex on
// and the platform sharing one sign-in, those of the other objects of the d8-* namespaces. keys
// are the objects of the cluster.
func ssoPeers(a Authenticator, keys []types.NamespacedName, settings Settings) []string {
	if !sharesSSO(settings, a.Namespace) {
		return nil
	}

	peers := make([]types.NamespacedName, 0, len(keys))
	for _, key := range keys {
		if key != a.Key() && sharesSSO(settings, key.Namespace) {
			peers = append(peers, key)
		}
	}
	slices.SortFunc(peers, compareKeys)

	ids := make([]string, 0, len(peers))
	for _, peer := range peers {
		ids = append(ids, naming.DexAuthenticatorClientID(peer.Name, peer.Namespace))
	}
	return ids
}
