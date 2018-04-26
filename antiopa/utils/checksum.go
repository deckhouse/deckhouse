package utils

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

func CalculateChecksum(stringArr ...string) string {
	hasher := md5.New()
	sort.Strings(stringArr)
	for _, value := range stringArr {
		hasher.Write([]byte(value))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
