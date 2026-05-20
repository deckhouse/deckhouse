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

package registry

import (
	"context"
	"net/http"
	"regexp"
)

type contextKey int

const paramsKey contextKey = iota

// RegexpHandler is an HTTP router that matches requests by method and regexp pattern.
// Use Add to register routes and ServeHTTP to dispatch them.
type RegexpHandler struct {
	routes []regexpRoute
	defaul http.HandlerFunc
}

type regexpRoute struct {
	pattern *regexp.Regexp
	handler http.HandlerFunc
}

// NewRegexpHandler creates a new Handler.
func NewRegexpHandler() *RegexpHandler {
	return &RegexpHandler{}
}

// SetDefault overrides the default 404 handler.
func (h *RegexpHandler) SetDefault(fn http.HandlerFunc) {
	h.defaul = fn
}

// Add registers a new route for the given HTTP method and regexp pattern.
func (h *RegexpHandler) Add(pattern *regexp.Regexp, fn http.HandlerFunc) {
	h.routes = append(h.routes, regexpRoute{pattern, fn})
}

func (h *RegexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rt := range h.routes {
		if m := rt.pattern.FindStringSubmatch(r.URL.Path); m != nil {
			params := make(map[string]string, len(m)-1)
			for i, name := range rt.pattern.SubexpNames() {
				if name != "" {
					params[name] = m[i]
				}
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, paramsKey, params)
			rt.handler(w, r.WithContext(ctx))
			return
		}
	}
	if h.defaul != nil {
		h.defaul(w, r)
		return
	}
	http.NotFound(w, r)
}

func RegexpParam(r *http.Request, name string) string {
	params, _ := r.Context().Value(paramsKey).(map[string]string)
	return params[name]
}
