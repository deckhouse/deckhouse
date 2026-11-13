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

package kuberbacauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apimachinery/pkg/util/wait"
	authnclientv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authzclientv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// User is the identity confirmed by a TokenReview.
type User struct {
	Name   string
	UID    string
	Groups []string
	Extra  map[string]authenticationv1.ExtraValue
}

// decisionCache keeps each entry fresh for its own TTL and then, for staleTTL more,
// only as a fallback for the time the API server cannot answer.
type decisionCache struct {
	lru      *utilcache.LRUExpireCache
	clock    utilcache.Clock
	staleTTL time.Duration
}

type cacheEntry struct {
	value      any
	freshUntil time.Time
}

func newDecisionCache(size int, staleTTL time.Duration, clock utilcache.Clock) *decisionCache {
	return &decisionCache{
		lru:      utilcache.NewLRUExpireCacheWithClock(size, clock),
		clock:    clock,
		staleTTL: staleTTL,
	}
}

// get returns the cached value, whether it is still fresh and whether it was found at all.
func (c *decisionCache) get(key string) (any, bool, bool) {
	v, ok := c.lru.Get(key)
	if !ok {
		return nil, false, false
	}
	e := v.(cacheEntry)
	return e.value, c.clock.Now().Before(e.freshUntil), true
}

func (c *decisionCache) add(key string, value any, ttl time.Duration) {
	c.lru.Add(key, cacheEntry{value: value, freshUntil: c.clock.Now().Add(ttl)}, ttl+c.staleTTL)
}

// retry calls fn until it succeeds, fails for good, the backoff runs out or ctx is done,
// and returns the last error.
func retry(ctx context.Context, backoff wait.Backoff, fn func(context.Context) error) error {
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		lastErr = fn(ctx)
		if lastErr != nil && permanent(lastErr) {
			return false, lastErr
		}
		return lastErr == nil, nil
	})
	if lastErr != nil {
		return lastErr
	}
	return err
}

// permanent reports an answer of the API server that a retry cannot change, such as Forbidden
// when the application may not create reviews.
func permanent(err error) bool {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return false
	}
	code := status.Status().Code
	return code >= 400 && code < 500 && code != http.StatusRequestTimeout && code != http.StatusTooManyRequests
}

// tokenResult wraps the user so that an invalid token is cached as a nil user.
type tokenResult struct {
	user *User
}

type authenticator struct {
	client  authnclientv1.TokenReviewsGetter
	cache   *decisionCache
	okTTL   time.Duration
	failTTL time.Duration
	backoff wait.Backoff
	log     *slog.Logger
}

// authenticate returns nil and no error when the token is not valid.
func (a *authenticator) authenticate(ctx context.Context, token string) (*User, error) {
	sum := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(sum[:])

	cached, fresh, found := a.cache.get(key)
	if fresh {
		return cached.(tokenResult).user, nil
	}

	var result *authenticationv1.TokenReview
	err := retry(ctx, a.backoff, func(ctx context.Context) error {
		var err error
		result, err = a.client.TokenReviews().Create(ctx, &authenticationv1.TokenReview{
			Spec: authenticationv1.TokenReviewSpec{Token: token},
		}, metav1.CreateOptions{})
		return err
	})
	if err != nil {
		if found {
			a.log.Warn("TokenReview failed, using a cached result", slog.String("error", err.Error()))
			return cached.(tokenResult).user, nil
		}
		return nil, err
	}

	if !result.Status.Authenticated {
		a.cache.add(key, tokenResult{}, a.failTTL)
		return nil, nil
	}
	u := result.Status.User
	user := &User{Name: u.Username, UID: u.UID, Groups: u.Groups, Extra: u.Extra}
	a.cache.add(key, tokenResult{user: user}, a.okTTL)
	return user, nil
}

type authorizer struct {
	client   authzclientv1.SubjectAccessReviewsGetter
	cache    *decisionCache
	allowTTL time.Duration
	denyTTL  time.Duration
	backoff  wait.Backoff
	log      *slog.Logger
}

// authorize returns whether user may perform verb on attrs and the reason given by the API server.
func (a *authorizer) authorize(ctx context.Context, user *User, verb string, attrs ResourceAttributes) (bool, string, error) {
	spec := authorizationv1.SubjectAccessReviewSpec{
		User:   user.Name,
		UID:    user.UID,
		Groups: user.Groups,
		ResourceAttributes: &authorizationv1.ResourceAttributes{
			Namespace:   attrs.Namespace,
			Verb:        verb,
			Group:       attrs.APIGroup,
			Version:     attrs.APIVersion,
			Resource:    attrs.Resource,
			Subresource: attrs.Subresource,
			Name:        attrs.Name,
		},
	}
	if len(user.Extra) > 0 {
		spec.Extra = make(map[string]authorizationv1.ExtraValue, len(user.Extra))
		for k, v := range user.Extra {
			spec.Extra[k] = authorizationv1.ExtraValue(v)
		}
	}
	key := sarKey(&spec)

	cached, fresh, found := a.cache.get(key)
	if fresh {
		d := cached.(authorizationv1.SubjectAccessReviewStatus)
		return d.Allowed, d.Reason, nil
	}

	var result *authorizationv1.SubjectAccessReview
	err := retry(ctx, a.backoff, func(ctx context.Context) error {
		var err error
		result, err = a.client.SubjectAccessReviews().Create(ctx, &authorizationv1.SubjectAccessReview{Spec: spec}, metav1.CreateOptions{})
		return err
	})
	if err != nil {
		if found {
			a.log.Warn("SubjectAccessReview failed, using a cached decision", slog.String("error", err.Error()))
			d := cached.(authorizationv1.SubjectAccessReviewStatus)
			return d.Allowed, d.Reason, nil
		}
		return false, "", err
	}

	ttl := a.denyTTL
	if result.Status.Allowed {
		ttl = a.allowTTL
	}
	a.cache.add(key, result.Status, ttl)
	return result.Status.Allowed, result.Status.Reason, nil
}

// sarKey builds a cache key from every field of the review that can change the decision.
func sarKey(s *authorizationv1.SubjectAccessReviewSpec) string {
	r := s.ResourceAttributes
	groups := append([]string(nil), s.Groups...)
	sort.Strings(groups)
	extraKeys := make([]string, 0, len(s.Extra))
	for k := range s.Extra {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)

	parts := make([]string, 0, 10+len(extraKeys))
	parts = append(parts, s.User, s.UID, strings.Join(groups, "\x01"),
		r.Namespace, r.Verb, r.Group, r.Version, r.Resource, r.Subresource, r.Name)
	for _, k := range extraKeys {
		parts = append(parts, k+"\x02"+strings.Join(s.Extra[k], "\x01"))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
