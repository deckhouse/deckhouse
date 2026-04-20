// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import "net/http"

// TransportMiddleware is a function that wraps an http.RoundTripper to add
// cross-cutting behaviour such as metrics, tracing, logging or rate-limiting.
//
// Example:
//
//	func loggingMiddleware(next http.RoundTripper) http.RoundTripper {
//	    return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
//	        log.Printf("-> %s %s", req.Method, req.URL)
//	        resp, err := next.RoundTrip(req)
//	        if err == nil {
//	            log.Printf("<- %d %s", resp.StatusCode, req.URL)
//	        }
//	        return resp, err
//	    })
//	}
type TransportMiddleware func(http.RoundTripper) http.RoundTripper

// RoundTripperFunc is an adapter to allow the use of ordinary functions as
// http.RoundTripper. If f is a function with the appropriate signature,
// RoundTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// WithMiddleware returns an Option that applies the given transport middlewares.
// Middlewares are applied in order: the first middleware wraps the outermost layer.
//
//	client.New("registry.example.com",
//	    client.WithMiddleware(metricsMiddleware, tracingMiddleware),
//	)
func WithMiddleware(middlewares ...TransportMiddleware) Option {
	return func(o *Options) {
		o.Middlewares = append(o.Middlewares, middlewares...)
	}
}
