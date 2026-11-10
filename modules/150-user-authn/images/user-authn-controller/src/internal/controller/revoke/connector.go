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

package revoke

import (
	"context"
	"time"
)

// DeleteConnectorTokens deletes the refresh tokens Dex issued through the connector, whoever the user is, and the
// references to them in the OfflineSessions of the pairs, and returns how many tokens it deleted. The users sign in
// again, and the tokens Dex issues then carry the claims the connector gives them now.
//
// With a non-zero createdBefore it deletes only the tokens issued before that time, and the ones whose issue time is
// unknown (the zero time is before every cut-off). A token issued later was issued under what the connector is now, and its user need not sign in again.
//
// It touches neither the generations of the pairs nor the browser sessions: the sign-in a browser session
// stands for has nothing stored that depends on the connector settings, and the access and ID tokens already
// issued run out by themselves. Dex does not read a Revocation of type connector, so there is no record to write
// either. Every step can be repeated: a token or a reference that is gone is not an error.
func (r *Revoker) DeleteConnectorTokens(ctx context.Context, connectorID string, createdBefore time.Time) (int, error) {
	return r.deleteTokens(ctx, func(token tokenOwner) bool {
		if token.connectorID != connectorID {
			return false
		}
		return createdBefore.IsZero() || token.createdAt.Before(createdBefore)
	}, "connectorID", connectorID)
}
