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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// DexAuthenticatorLabel marks the OAuth2Client of a DexAuthenticator; only these clients get the
	// feed.
	DexAuthenticatorLabel = "deckhouse.io/dex-authenticator-for"

	// secretHashField and saltField replace the client secret in the cache: SHA-256 of the secret,
	// to check the secret a client presents, and the salt of its keys. Both hex.
	secretHashField = "secretSHA256"
	saltField       = "feedSalt"

	// saltMessage is what the salt of a client is the HMAC of, keyed with the client secret.
	saltMessage = "deckhouse.io/user-authn/revocation-feed/v1/salt"

	// Key kinds: the value is the subject of a pair, the ID of a browser session (the sid claim), or
	// the ID of a grant (the refresh_id of a refresh token).
	keyPair    = "pair"
	keySession = "sid"
	keyGrant   = "grant"
)

// Salt is the salt of the keys of a client: HMAC-SHA256 of saltMessage keyed with the client secret.
// The client and the controller both know the secret, so nothing is handed out, and the keys stay
// the same across reconnections.
func Salt(secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(saltMessage))
	return mac.Sum(nil)
}

// Key is what the feed sends a client instead of a subject, a session ID, or a grant ID: the first
// 128 bits of HMAC-SHA256(salt, kind || 0x00 || value), base64url without padding. Another client
// cannot match it to its own users, and nobody without the salt can guess the value behind it.
func Key(salt []byte, kind, value string) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(kind))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// ClientCache is how the manager caches OAuth2Client objects for the feed: the clients of
// DexAuthenticators in the Dex namespace, without their secrets (TransformClient).
func ClientCache() (client.Object, cache.ByObject, error) {
	exists, err := labels.NewRequirement(DexAuthenticatorLabel, selection.Exists, nil)
	if err != nil {
		return nil, cache.ByObject{}, fmt.Errorf("select dex-authenticator clients: %w", err)
	}
	return controller.Object(controller.OAuth2ClientGVK), cache.ByObject{
		Namespaces: map[string]cache.Config{naming.DexNamespace: {}},
		Label:      labels.NewSelector().Add(*exists),
		Transform:  TransformClient,
	}, nil
}

// TransformClient reduces an OAuth2Client to what the feed needs: the name, namespace, labels, and
// resourceVersion, the client id, and the SHA-256 and the salt derived from the secret. The secret
// itself is not kept.
func TransformClient(obj any) (any, error) {
	switch typed := obj.(type) {
	case *unstructured.Unstructured:
		if typed == nil {
			return obj, nil
		}
		return reduceClient(typed), nil
	case toolscache.DeletedFinalStateUnknown:
		inner, err := TransformClient(typed.Obj)
		if err != nil {
			return nil, err
		}
		typed.Obj = inner
		return typed, nil
	default:
		return obj, nil
	}
}

func reduceClient(u *unstructured.Unstructured) *unstructured.Unstructured {
	out := &unstructured.Unstructured{Object: map[string]any{}}
	out.SetAPIVersion(u.GetAPIVersion())
	out.SetKind(u.GetKind())
	out.SetName(u.GetName())
	out.SetNamespace(u.GetNamespace())
	out.SetLabels(u.GetLabels())
	out.SetResourceVersion(u.GetResourceVersion())
	out.SetUID(u.GetUID())
	if id, ok := u.Object["id"].(string); ok {
		out.Object["id"] = id
	}
	if secret, ok := u.Object["secret"].(string); ok && secret != "" {
		sum := sha256.Sum256([]byte(secret))
		out.Object[secretHashField] = hex.EncodeToString(sum[:])
		out.Object[saltField] = hex.EncodeToString(Salt([]byte(secret)))
		return out
	}
	// client-go may run a transform again on an object it has already transformed, so an object
	// without the secret keeps the hash and the salt it carries.
	for _, field := range []string{secretHashField, saltField} {
		if value, ok := u.Object[field].(string); ok {
			out.Object[field] = value
		}
	}
	return out
}

// feedClient is an authenticated DexAuthenticator client.
type feedClient struct {
	id   string
	salt []byte
}

// dummyHash stands in for the secret of an unknown client, so that it takes as long to refuse as a
// wrong secret.
var dummyHash = sha256.Sum256([]byte("deckhouse.io/user-authn/revocation-feed/unknown-client"))

// authenticate checks the HTTP Basic credentials of r, client_id:client_secret form-encoded as on the
// token endpoint (RFC 6749, section 2.3.1), against the cached DexAuthenticator clients. A missing,
// unknown, or wrong client is false, with no difference among them. An error is a failed cache read.
func authenticate(ctx context.Context, reader client.Reader, r *http.Request) (feedClient, bool, error) {
	rawID, rawSecret, ok := r.BasicAuth()
	if !ok {
		return feedClient{}, false, nil
	}
	id, err := url.QueryUnescape(rawID)
	if err != nil {
		return feedClient{}, false, nil
	}
	secret, err := url.QueryUnescape(rawSecret)
	if err != nil {
		return feedClient{}, false, nil
	}

	list := controller.List(controller.OAuth2ClientGVK)
	if err := reader.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.OAuth2ClientIDField: id}); err != nil {
		return feedClient{}, false, fmt.Errorf("list oauth2 clients: %w", err)
	}
	stored := dummyHash[:]
	var salt []byte
	found := false
	for i := range list.Items {
		item := &list.Items[i]
		if _, isAuthenticator := item.GetLabels()[DexAuthenticatorLabel]; !isAuthenticator {
			continue
		}
		hash, hashErr := hex.DecodeString(stringField(item, secretHashField))
		itemSalt, saltErr := hex.DecodeString(stringField(item, saltField))
		if hashErr != nil || saltErr != nil || len(hash) != sha256.Size || len(itemSalt) == 0 {
			continue
		}
		stored, salt, found = hash, itemSalt, true
		break
	}
	presented := sha256.Sum256([]byte(secret))
	match := subtle.ConstantTimeCompare(presented[:], stored) == 1
	if !found || !match {
		return feedClient{}, false, nil
	}
	return feedClient{id: id, salt: salt}, true, nil
}

func stringField(obj *unstructured.Unstructured, field string) string {
	s, _ := obj.Object[field].(string)
	return s
}
