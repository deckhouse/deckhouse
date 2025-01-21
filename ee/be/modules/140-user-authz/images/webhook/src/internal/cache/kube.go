/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"net/http"
	"os"
	"sync"
	"time"
)

// Update token periodically because BoundServiceAccountToken feature is enabled for Kubernetes >=1.21
// https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#bound-service-account-token-volume

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

	token, err := os.ReadFile(tokenPath)
	if err != nil {
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
