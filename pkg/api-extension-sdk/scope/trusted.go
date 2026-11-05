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

package scope

import (
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/klog/v2"
)

// trustFunc decides whether r was sent by kube-apiserver's front proxy. user
// is the username the proxy asserted; reason explains a negative answer and
// goes to the log only.
type trustFunc func(r *http.Request) (user string, trusted bool, reason string)

// WithTrusted wraps the handler chain of a k8s.io/apiserver based server. It
// must sit outside WithAuthentication, which deletes the identity headers:
//
//	cfg.BuildHandlerChainFunc = func(h http.Handler, c *genericapiserver.Config) http.Handler {
//		return scope.WithTrusted(genericapiserver.DefaultBuildHandlerChain(h, c), c.Authentication.RequestHeaderConfig)
//	}
//
// A request is trusted when the requestheader authenticator built from rh
// would accept it: a client certificate that chains to the requestheader
// client CA (reloaded with rh.CAContentProvider), a common name from
// rh.AllowedClientNames, and a non-empty username header. Unlike the upstream
// authenticator, an empty list of allowed names trusts nothing. A nil rh, or a
// provider without a CA, trusts nothing.
func WithTrusted(h http.Handler, rh *authenticatorfactory.RequestHeaderConfig) http.Handler {
	return newTrustedHandler(h, requestHeaderTrust(rh))
}

func requestHeaderTrust(rh *authenticatorfactory.RequestHeaderConfig) trustFunc {
	if rh == nil || rh.CAContentProvider == nil {
		return func(*http.Request) (string, bool, string) {
			return "", false, "no requestheader configuration"
		}
	}
	allowed := orEmpty(rh.AllowedClientNames)
	verifier := headerrequest.NewDynamicVerifyOptionsSecure(
		rh.CAContentProvider.VerifyOptions,
		allowed,
		orEmpty(rh.UsernameHeaders),
		orEmpty(rh.UIDHeaders),
		orEmpty(rh.GroupHeaders),
		orEmpty(rh.ExtraHeaderPrefixes),
	)
	return func(r *http.Request) (string, bool, string) {
		if !anyNonEmpty(allowed.Value()) {
			return "", false, "requestheader allowed names are empty"
		}
		if _, ok := rh.CAContentProvider.VerifyOptions(); !ok {
			return "", false, "no requestheader client CA"
		}
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			return "", false, "no client certificate"
		}
		// The header authenticator deletes the identity headers of the
		// request it is given; WithAuthentication further down needs them.
		resp, ok, err := verifier.AuthenticateRequest(r.Clone(r.Context()))
		if err != nil {
			return "", false, err.Error()
		}
		if !ok || resp == nil || resp.User == nil || resp.User.GetName() == "" {
			return "", false, "not authenticated by the requestheader authenticator"
		}
		return resp.User.GetName(), true, ""
	}
}

func orEmpty(p headerrequest.StringSliceProvider) headerrequest.StringSliceProvider {
	if p == nil {
		return headerrequest.StaticStringSlice(nil)
	}
	return p
}

func anyNonEmpty(values []string) bool {
	for _, v := range values {
		if v != "" {
			return true
		}
	}
	return false
}

// newTrustedHandler is the common part of WithTrusted and NewTrustedHandler.
func newTrustedHandler(h http.Handler, trust trustFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasScopeHeaders(r.Header) {
			h.ServeHTTP(w, r)
			return
		}
		user, trusted, reason := trust(r)
		if !trusted {
			klog.V(2).InfoS("scope: ignoring scope headers of an untrusted request", "reason", reason, "method", r.Method, "path", r.URL.Path)
			stripScopeHeaders(r.Header)
			h.ServeHTTP(w, r)
			return
		}
		d, ok, err := parseHeaders(r.Header)
		stripScopeHeaders(r.Header)
		if err != nil {
			klog.InfoS("scope: refusing a request with malformed scope headers", "user", user, "err", err, "method", r.Method, "path", r.URL.Path)
			writeBadRequest(w, "malformed scope headers: "+err.Error())
			return
		}
		if !ok {
			h.ServeHTTP(w, r)
			return
		}
		klog.V(3).InfoS("scope: trusted decision", "user", user, "mode", d.Mode, "namespaces", d.Namespaces.Len(), "method", r.Method, "path", r.URL.Path)
		h.ServeHTTP(w, r.WithContext(WithDecision(r.Context(), d)))
	})
}

// writeBadRequest answers before content negotiation exists in the chain, so
// it writes a raw metav1.Status.
func writeBadRequest(w http.ResponseWriter, msg string) {
	status := apierrors.NewBadRequest(msg).Status()
	status.Kind, status.APIVersion = "Status", "v1"
	responsewriters.WriteRawJSON(int(status.Code), status, w)
}
