/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

func GenerateHash() string {
	// Get the current time
	currentTime := time.Now()

	// Convert time to string
	timeStr := currentTime.Format(time.RFC3339)

	// Convert time string to byte array
	timeBytes := []byte(timeStr)

	// Calculate hash from the time string
	hash := sha256.New()
	hash.Write(timeBytes)
	hashInBytes := hash.Sum(nil)

	// Convert hash to hex string
	hashStr := hex.EncodeToString(hashInBytes)

	return hashStr
}
