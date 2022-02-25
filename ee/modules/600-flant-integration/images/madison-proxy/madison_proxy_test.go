/*
Copyright 2021 Flant JSC

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

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxy(t *testing.T) {
	madisonKey := "testkey"
	madisonBackend := "192.168.1.1:8080"
	madisonScheme := "http"

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, rs *http.Request) {
		w.Header().Set("X-Echo-Host", rs.Host)
		switch rs.URL.String() {
		case "/api/events/prometheus/" + madisonKey:
			w.WriteHeader(http.StatusOK)
			return

		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	madisonBackend = strings.TrimPrefix(u.String(), "http://")

	t.Run("check v1 route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", nil)
		rw := httptest.NewRecorder()

		router := newMadisonProxy(madisonScheme, madisonBackend, madisonKey)
		router.ServeHTTP(rw, req)
		assert.Equal(t, http.StatusOK, rw.Result().StatusCode)
		assert.Equal(t, "madison.flant.com", rw.Result().Header.Get("X-Echo-Host"))
	})

	t.Run("check v2 route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/alerts", nil)
		rw := httptest.NewRecorder()

		router := newMadisonProxy(madisonScheme, madisonBackend, madisonKey)
		router.ServeHTTP(rw, req)
		assert.Equal(t, http.StatusOK, rw.Result().StatusCode)
		assert.Equal(t, "madison.flant.com", rw.Result().Header.Get("X-Echo-Host"))
	})
}
