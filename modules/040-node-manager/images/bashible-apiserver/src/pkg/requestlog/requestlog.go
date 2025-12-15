/*
Copyright 2025 Flant JSC

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

package requestlog

import (
	"context"
	"net"
	"net/http"
	"strings"

	"bashible-apiserver/pkg/apis/bashible"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"
)

type contextKey string

const requestIDKey contextKey = "bashible-request-id"
const checksumAnnotation = "bashible.deckhouse.io/configuration-checksum"
const bashibles_uri = "/apis/bashible.deckhouse.io"

func WithRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.RequestURI, bashibles_uri) {
			next.ServeHTTP(w, r)
			return
		}

		reqID := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		r = r.WithContext(ctx)

		info, _ := apirequest.RequestInfoFrom(ctx)
		resource := resourceName(info)
		if len(resource) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		klog.Infof(
			"bashible-request id=%s remote=%s method=%s uri=%s resource=%s name=%s verb=%s query=%s ua=%s",
			reqID,
			remoteIP(r.RemoteAddr),
			r.Method,
			r.RequestURI,
			resourceName(info),
			infoName(info),
			infoVerb(info),
			r.URL.RawQuery,
			r.UserAgent(),
		)

		next.ServeHTTP(w, r)
	})
}

func RequestIDFrom(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func LogRenderResult(ctx context.Context, obj runtime.Object, fromCache bool, renderErr error) {
	reqID := RequestIDFrom(ctx)
	info, _ := apirequest.RequestInfoFrom(ctx)

	if renderErr != nil {
		klog.Errorf(
			"bashible-response id=%s resource=%s name=%s from_cache=%t error=%v",
			reqID,
			resourceName(info),
			infoName(info),
			fromCache,
			renderErr,
		)
		return
	}

	checksum, ok := bashibleChecksum(obj)
	if !ok {
		// Not our custom type — skip noisy logging.
		return
	}

	klog.Infof(
		"bashible-response id=%s resource=%s name=%s from_cache=%t checksum=%s",
		reqID,
		resourceName(info),
		infoName(info),
		fromCache,
		checksum,
	)
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	return remoteAddr
}

func bashibleChecksum(obj runtime.Object) (string, bool) {
	switch obj.(type) {
	case *bashible.NodeGroupBundle, *bashible.Bashible, *bashible.Bootstrap:
	default:
		return "", false
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", false
	}

	ann := accessor.GetAnnotations()
	if ann == nil {
		return "", false
	}

	val := ann[checksumAnnotation]
	if val == "" {
		return "", false
	}

	return val, true
}

func resourceName(info *apirequest.RequestInfo) string {
	if info == nil {
		return ""
	}
	return info.Resource
}

func infoName(info *apirequest.RequestInfo) string {
	if info == nil {
		return ""
	}
	return info.Name
}

func infoVerb(info *apirequest.RequestInfo) string {
	if info == nil {
		return ""
	}
	return info.Verb
}
