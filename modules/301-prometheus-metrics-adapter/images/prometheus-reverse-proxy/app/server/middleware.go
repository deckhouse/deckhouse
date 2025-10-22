/*
Copyright 2022 Flant JSC

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

package server

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
)

/////////////////////
//   Middlewares   //
/////////////////////

// 1. Token update proxy

// Update token periodically because BoundServiceAccountToken feature is enabled for Kubernetes >=1.21
// https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#bound-service-account-token-volume

const (
	renewTokenPeriod = 30 * time.Second
	tokenPath        = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type kubeTransport struct {
	mu     sync.RWMutex
	token  string
	expiry time.Time

	base http.RoundTripper
}

func wrapKubeTransport(base http.RoundTripper) http.RoundTripper {
	t := &kubeTransport{base: base}
	t.updateToken()
	return t
}

func (t *kubeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.updateToken()

	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+t.GetToken())

	return t.base.RoundTrip(r2)
}

func (t *kubeTransport) updateToken() {
	t.mu.RLock()
	exp := t.expiry
	t.mu.RUnlock()

	now := time.Now()
	if now.Before(exp) {
		// Do not need to update token yet
		return
	}

	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		errLog.Println("Cannot read service account token, will try later")
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = string(token)
	t.expiry = now.Add(renewTokenPeriod)
}

func (t *kubeTransport) GetToken() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token
}

// 2. Log requests

type logHandler struct {
	base   http.HandlerFunc
	logger *log.Logger
}

func wrapLoggerHandler(base http.HandlerFunc) http.Handler {
	t := &logHandler{base: base, logger: infLog}
	return t
}

func (t *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := uuid.New()
	t.logger.Printf("%s -- Start request: %s [%s] %s %s\n",
		id, r.RemoteAddr, r.UserAgent(), r.Method, r.URL.String(),
	)

	ctx := context.WithValue(r.Context(), "id", id.String())
	req := r.WithContext(ctx)
	*r = *req

	m := httpsnoop.CaptureMetrics(t.base, w, r)

	t.logger.Printf("%s -- Response: %d, %s, %d bytes\n", id, m.Code, m.Duration.String(), m.Written)
}
