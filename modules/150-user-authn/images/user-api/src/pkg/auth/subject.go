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

package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// Field numbers of the Dex message IDTokenSubject {string user_id = 1; string conn_id = 2;}.
const (
	subjectUserIDField      protowire.Number = 1
	subjectConnectorIDField protowire.Number = 2
)

// ParseSubject decodes the sub claim of a Dex token into the user ID and the connector ID it
// encodes: the protobuf message IDTokenSubject in base64url without padding. Both fields must be
// present, non-empty and appear once; Dex never writes anything else into the message.
func ParseSubject(sub string) (string, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(sub)
	if err != nil {
		return "", "", fmt.Errorf("decode base64url: %w", err)
	}

	var userID, connectorID string
	var seenUserID, seenConnectorID bool
	for len(data) > 0 {
		number, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			return "", "", fmt.Errorf("read field tag: %w", protowire.ParseError(n))
		}
		data = data[n:]

		if wireType != protowire.BytesType {
			return "", "", fmt.Errorf("field %d has wire type %d, want a string", number, wireType)
		}
		value, n := protowire.ConsumeBytes(data)
		if n < 0 {
			return "", "", fmt.Errorf("read field %d: %w", number, protowire.ParseError(n))
		}
		data = data[n:]

		if !utf8.Valid(value) {
			return "", "", fmt.Errorf("field %d is not valid UTF-8", number)
		}

		switch number {
		case subjectUserIDField:
			if seenUserID {
				return "", "", errors.New("user ID is repeated")
			}
			seenUserID = true
			userID = string(value)
		case subjectConnectorIDField:
			if seenConnectorID {
				return "", "", errors.New("connector ID is repeated")
			}
			seenConnectorID = true
			connectorID = string(value)
		default:
			return "", "", fmt.Errorf("unexpected field %d", number)
		}
	}

	if userID == "" {
		return "", "", errors.New("user ID is empty")
	}
	if connectorID == "" {
		return "", "", errors.New("connector ID is empty")
	}

	return userID, connectorID, nil
}
