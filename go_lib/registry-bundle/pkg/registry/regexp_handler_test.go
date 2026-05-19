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
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestHandler_Routing(t *testing.T) {
	type testRoute struct {
		pattern string
	}

	// /api/users
	route1 := &testRoute{
		pattern: `^/api/users$`,
	}

	// /api/users/:id
	route2 := &testRoute{
		pattern: `^/api/users/(?P<id>[^/]+)$`,
	}

	// /api/users/:id/posts
	route6 := &testRoute{
		pattern: `^/api/users/(?P<id>[^/]+)/posts$`,
	}

	// /health
	route7 := &testRoute{
		pattern: `^/health$`,
	}

	// /files/:path — path may contain multiple slashes
	route8 := &testRoute{
		pattern: `^/files/(?P<path>.+)$`,
	}

	// /storage/:bucket/:key — key may contain slashes
	route9 := &testRoute{
		pattern: `^/storage/(?P<bucket>[^/]+)/(?P<key>.+)$`,
	}

	routes := []*testRoute{
		route1,
		route2,
		route6,
		route7,
		route8,
		route9,
	}

	tests := []struct {
		path string
		want *testRoute
	}{
		// list
		{
			path: "/api/users",
			want: route1,
		},
		// by id
		{
			path: "/api/users/42",
			want: route2,
		},
		{
			path: "/api/users/abc-123",
			want: route2,
		},
		// nested resource
		{
			path: "/api/users/42/posts",
			want: route6,
		},
		// health
		{
			path: "/health",
			want: route7,
		},
		// no match: unknown path
		{
			path: "/api/unknown",
			want: nil,
		},
		// partial path should not match anchored pattern
		{
			path: "/api/users/42/extra",
			want: nil,
		},
		// /api/users/ with trailing slash should not match /api/users
		{
			path: "/api/users/",
			want: nil,
		},
		// health path with extra segment
		{
			path: "/health/check",
			want: nil,
		},
		// root path
		{
			path: "/",
			want: nil,
		},
		// file: multi-segment path
		{
			path: "/files/docs/reports/2024/summary.pdf",
			want: route8,
		},
		// file: single-segment path
		{
			path: "/files/readme.txt",
			want: route8,
		},
		// file: deeply nested path
		{
			path: "/files/a/b/c/d/e",
			want: route8,
		},
		// /files/ trailing slash: httptest.NewRequest normalises "/files/" → "/files",
		// which does not match ^/files/(?P<path>.+)$ — no route expected.
		{
			path: "/files/",
			want: nil,
		},
		// storage: key with multiple slashes
		{
			path: "/storage/mybucket/2024/01/report.csv",
			want: route9,
		},
		// storage: key with single segment
		{
			path: "/storage/mybucket/file.txt",
			want: route9,
		},
		// storage: key with multiple slashes (different path)
		{
			path: "/storage/mybucket/logs/app/error.log",
			want: route9,
		},
	}

	h := NewRegexpHandler()
	var result *testRoute
	for _, rt := range routes {
		h.Add(regexp.MustCompile(rt.pattern), func(_ http.ResponseWriter, _ *http.Request) {
			result = rt
		})
	}
	h.SetDefault(func(_ http.ResponseWriter, _ *http.Request) {
		result = nil
	})

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result = nil
			req, err := http.NewRequest(http.MethodGet, tt.path, nil)
			if err != nil {
				if tt.want != nil {
					t.Errorf("failed to create request: %v", err)
				}
				return
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if result != tt.want {
				t.Errorf("got route %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHandler_PathParams(t *testing.T) {
	type requestResponse struct {
		requestPath string
		wantParam   map[string]string
	}

	tests := []struct {
		name     string
		pattern  string
		subtests []requestResponse
	}{
		{
			name:    "user id param",
			pattern: `^/api/users/(?P<id>[^/]+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/api/users/42",
					wantParam: map[string]string{
						"id": "42",
					},
				},
			},
		},
		{
			name:    "user and post id params",
			pattern: `^/api/users/(?P<user_id>[^/]+)/posts/(?P<post_id>[^/]+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/api/users/7/posts/99",
					wantParam: map[string]string{
						"user_id": "7",
						"post_id": "99",
					},
				},
			},
		},
		{
			name:    "slug with dashes and dots",
			pattern: `^/api/articles/(?P<slug>[^/]+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/api/articles/hello-world-v1.0",
					wantParam: map[string]string{
						"slug": "hello-world-v1.0",
					},
				},
			},
		},
		{
			name:    "missing param returns empty string",
			pattern: `^/api/users/(?P<id>[^/]+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/api/users/42",
					wantParam: map[string]string{
						"nonexistent": "",
					},
				},
			},
		},
		{
			name:    "multiple subtests for same pattern",
			pattern: `^/api/users/(?P<id>[^/]+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/api/users/1",
					wantParam: map[string]string{
						"id": "1",
					},
				},
				{
					requestPath: "/api/users/abc-def",
					wantParam: map[string]string{
						"id": "abc-def",
					},
				},
			},
		},
		{
			name:    "path param captures multiple slashes",
			pattern: `^/files/(?P<path>.+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/files/docs/reports/2024/summary.pdf",
					wantParam: map[string]string{
						"path": "docs/reports/2024/summary.pdf",
					},
				},
				{
					requestPath: "/files/readme.txt",
					wantParam: map[string]string{
						"path": "readme.txt",
					},
				},
			},
		},
		{
			name:    "bucket fixed, key captures multiple slashes",
			pattern: `^/storage/(?P<bucket>[^/]+)/(?P<key>.+)$`,
			subtests: []requestResponse{
				{
					requestPath: "/storage/mybucket/2024/01/report.csv",
					wantParam: map[string]string{
						"bucket": "mybucket",
						"key":    "2024/01/report.csv",
					},
				},
				{
					requestPath: "/storage/other-bucket/a/b/c/d",
					wantParam: map[string]string{
						"bucket": "other-bucket",
						"key":    "a/b/c/d",
					},
				},
			},
		},
		{
			name:    "fixed prefix and suffix around multi-slash param",
			pattern: `^/export/(?P<format>[^/]+)/data/(?P<query>.+)/download$`,
			subtests: []requestResponse{
				{
					requestPath: "/export/csv/data/users/active/2024/download",
					wantParam: map[string]string{
						"format": "csv",
						"query":  "users/active/2024",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subtests {
				got := make(map[string]string)
				h := NewRegexpHandler()
				h.Add(regexp.MustCompile(tt.pattern), func(w http.ResponseWriter, r *http.Request) {
					for key := range st.wantParam {
						got[key] = RegexpParam(r, key)
					}
					w.WriteHeader(http.StatusOK)
				})

				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, st.requestPath, nil))

				for key, want := range st.wantParam {
					if got[key] != want {
						t.Errorf("PathParam(%q) = %q, want %q", key, got[key], want)
					}
				}
			}
		})
	}
}
