package httpclient

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Update token periodically because BoundServiceAccountToken feature is enabled for Kubernetes >=1.21
// https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#bound-service-account-token-volume
const (
	renewTokenPeriod = 30 * time.Second
	tokenPath        = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type Token struct {
	mu     sync.RWMutex
	token  string
	expiry time.Time
}

func NewToken() (*Token, error) {
	t := &Token{}
	if err := t.updateToken(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Token) updateToken() error {
	t.mu.RLock()
	exp := t.expiry
	t.mu.RUnlock()

	now := time.Now()
	if now.Before(exp) {
		return nil
	}

	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("cannot read service account token: %s", err.Error())
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = string(token)
	t.expiry = now.Add(renewTokenPeriod)
	return nil
}

func (t *Token) GetToken() (string, error) {
	if err := t.updateToken(); err != nil {
		return "", err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token, nil
}
