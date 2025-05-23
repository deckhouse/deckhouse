/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func ComputeHash(values ...any) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	hash := sha256.New()

	for _, value := range values {
		buf, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("marshal error: %w", err)
		}

		hash.Write(buf)
	}

	hashBytes := hash.Sum([]byte{})
	ret := hex.EncodeToString(hashBytes)

	return ret, nil
}
