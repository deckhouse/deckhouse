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

package revocationfeed

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

const (
	// DexAuthenticatorNamespaceLabel is the namespace of the DexAuthenticator object on its
	// OAuth2Client.
	DexAuthenticatorNamespaceLabel = "deckhouse.io/dex-authenticator-namespace"

	// InstanceSystem and InstanceUser are the two shared dex-authenticator instances, by the name of
	// their ServiceAccount: the first serves the DexAuthenticator objects of the d8-* namespaces, the
	// second all the others.
	InstanceSystem = "dex-authenticator-system"
	InstanceUser   = "dex-authenticator"

	// TokenAudience is the audience of the ServiceAccount token an instance presents to the feed: a
	// token of the instance issued for anything else is refused.
	TokenAudience = "revocation-feed.d8-user-authn"

	// tokenReviewTimeout bounds the review of a token: the subscriber waits for the answer.
	tokenReviewTimeout = 5 * time.Second

	systemNamespacePrefix = "d8-"
	bearerPrefix          = "Bearer "
	serviceAccountPrefix  = "system:serviceaccount:" + naming.DexNamespace + ":"
)

// TokenReviewer checks a ServiceAccount token issued for TokenAudience with the API server and
// returns the user name it belongs to; false for a token the API server does not accept.
type TokenReviewer func(ctx context.Context, token string) (username string, ok bool, err error)

// NewTokenReviewer returns a TokenReviewer that asks the API server through c, which must write
// to the API server directly. A token is accepted only when it was issued for TokenAudience.
func NewTokenReviewer(c client.Client) TokenReviewer {
	return func(ctx context.Context, token string) (string, bool, error) {
		ctx, cancel := context.WithTimeout(ctx, tokenReviewTimeout)
		defer cancel()

		review := &authenticationv1.TokenReview{
			Spec: authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{TokenAudience}},
		}
		if err := c.Create(ctx, review); err != nil {
			return "", false, fmt.Errorf("create a token review: %w", err)
		}
		if !review.Status.Authenticated || !slices.Contains(review.Status.Audiences, TokenAudience) {
			return "", false, nil
		}
		return review.Status.User.Username, true, nil
	}
}

// instanceOfNamespace is the instance that serves the DexAuthenticator objects of a namespace.
func instanceOfNamespace(namespace string) string {
	if strings.HasPrefix(namespace, systemNamespacePrefix) {
		return InstanceSystem
	}
	return InstanceUser
}

// bearerToken returns the bearer token of r, if it carries one.
func bearerToken(r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), bearerPrefix)
	return token, ok && token != ""
}

// authenticateInstance checks the ServiceAccount token of r and returns the instance it belongs
// to. A token the API server refuses, or one of another ServiceAccount, is false. An error is a
// failed review.
func authenticateInstance(ctx context.Context, review TokenReviewer, token string) (string, bool, error) {
	if review == nil {
		return "", false, nil
	}
	username, ok, err := review(ctx, token)
	if err != nil {
		return "", false, fmt.Errorf("review the token: %w", err)
	}
	if !ok {
		return "", false, nil
	}
	name, ok := strings.CutPrefix(username, serviceAccountPrefix)
	if !ok || (name != InstanceSystem && name != InstanceUser) {
		return "", false, nil
	}
	return name, true, nil
}

// instanceID names the streams of an instance where a stream is counted by its client.
func instanceID(instance string) string {
	return "instance:" + instance
}
