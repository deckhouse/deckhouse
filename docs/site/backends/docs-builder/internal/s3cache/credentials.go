// Copyright 2026 Flant JSC
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

package s3cache

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// credentials give the key a request to the bucket is signed with: the
// static keys of a Secret (staticKeys) or short-lived keys from STS
// (stsKeys).
type credentials interface {
	// get returns the key to sign a request with.
	get(ctx context.Context) (key, error)
	// refused forgets a key S3 refused and reports whether the request is
	// worth sending again.
	refused(k key) bool
}

// key is a key pair, with the time it was obtained and expires for one from
// STS.
type key struct {
	accessKey, secretKey, sessionToken string
	obtained, expires                  time.Time
}

// staticKeys are the keys of a Secret: a rotated pair restarts the pod.
type staticKeys struct{ key key }

func (s staticKeys) get(context.Context) (key, error) { return s.key, nil }

// refused reports false: a static key has nothing to replace it with.
func (staticKeys) refused(key) bool { return false }

const (
	// stsBackoff is how long a failure of STS is answered without asking it
	// again, so an unavailable STS costs one wait instead of one per request.
	stsBackoff = 30 * time.Second
	// keyLifetime is assumed for a key whose expiry STS did not give or gave
	// in the past (a clock skew); system-s3 issues keys for 30 minutes.
	keyLifetime = 25 * time.Minute
)

// stsKeys are the short-lived keys STS issues for the ServiceAccount token
// (AssumeRoleWithWebIdentity), as system-s3 does for a SystemBucket. A key
// is reused until a minute before it expires: STS allows few live keys per
// bucket.
type stsKeys struct {
	// endpoint issues the keys of the bucket for the token in tokenPath.
	endpoint, tokenPath string
	client              *http.Client
	now                 func() time.Time
	// ready is told whether STS gave a key.
	ready func(ready bool)

	sts singleflight.Group

	mu      sync.Mutex
	current key
	err     error // the last failure of STS, answered until retry
	retry   time.Time
	// keepUntil: a fresh key was refused, so refusals are not about the key
	// and do not replace it until then.
	keepUntil time.Time
}

func (s *stsKeys) get(ctx context.Context) (key, error) {
	s.mu.Lock()
	k, err := s.current, s.err
	if s.now().After(s.retry) {
		err = nil
	}
	s.mu.Unlock()

	if s.now().Before(k.expires.Add(-time.Minute)) {
		return k, nil
	}
	if err != nil {
		return key{}, err
	}

	// One request asks STS for everyone waiting; each waits no longer than
	// its own context.
	ch := s.sts.DoChan("key", func() (any, error) { return s.refresh() })
	select {
	case <-ctx.Done():
		return key{}, ctx.Err()
	case r := <-ch:
		if r.Err != nil {
			return key{}, r.Err
		}
		return r.Val.(key), nil
	}
}

func (s *stsKeys) refresh() (key, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	k, err := s.assumeRole(ctx)
	if err != nil {
		s.mu.Lock()
		s.err, s.retry = err, s.now().Add(stsBackoff)
		s.mu.Unlock()
		s.ready(false)
		return key{}, err
	}

	s.mu.Lock()
	s.current = k
	s.mu.Unlock()
	s.ready(true)

	return k, nil
}

// refused forgets a key S3 refused and reports whether the request is worth
// sending again. system-s3 may revoke a key before it expires (a restarted
// replica, too many live keys), so a key older than a minute is replaced. A
// fresh key that is refused means the refusal is not about the key (no
// access, or a storage that refuses a missing object): the key is kept until
// it would expire anyway, since asking STS on every refusal only adds live
// keys.
func (s *stsKeys) refused(k key) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == k {
		now := s.now()
		if now.Sub(k.obtained) < time.Minute {
			s.keepUntil = now.Add(keyLifetime)
			return false
		}
		if now.Before(s.keepUntil) {
			return false
		}
		s.current = key{}
	}

	return true
}

// assumeRole exchanges the ServiceAccount token for a key pair
// (AssumeRoleWithWebIdentity). The token file is read every time: kubelet
// rotates it.
func (s *stsKeys) assumeRole(ctx context.Context) (key, error) {
	token, err := os.ReadFile(s.tokenPath)
	if err != nil {
		return key{}, fmt.Errorf("read the ServiceAccount token: %w", err)
	}

	form := url.Values{
		"Action":           {"AssumeRoleWithWebIdentity"},
		"Version":          {"2011-06-15"},
		"WebIdentityToken": {strings.TrimSpace(string(token))},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return key{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return key{}, fmt.Errorf("STS: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return key{}, fmt.Errorf("STS: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return key{}, fmt.Errorf("STS: %s: %s", resp.Status, errorCode(body))
	}

	var out struct {
		Credentials struct {
			AccessKeyID     string    `xml:"AccessKeyId"`
			SecretAccessKey string    `xml:"SecretAccessKey"`
			SessionToken    string    `xml:"SessionToken"`
			Expiration      time.Time `xml:"Expiration"`
		} `xml:"AssumeRoleWithWebIdentityResult>Credentials"`
	}
	if err := xml.Unmarshal(body, &out); err != nil {
		return key{}, fmt.Errorf("STS response: %w", err)
	}
	if out.Credentials.AccessKeyID == "" {
		return key{}, errors.New("STS response has no key")
	}

	now := s.now()
	expires := out.Credentials.Expiration
	if !expires.After(now.Add(2 * time.Minute)) {
		expires = now.Add(keyLifetime)
	}

	return key{
		accessKey:    out.Credentials.AccessKeyID,
		secretKey:    out.Credentials.SecretAccessKey,
		sessionToken: out.Credentials.SessionToken,
		obtained:     now,
		expires:      expires,
	}, nil
}
