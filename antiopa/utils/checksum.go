package utils

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

func CalculateChecksum(Values ...string) string {
	hasher := md5.New()
	sort.Strings(Values)
	for _, value := range Values {
		hasher.Write([]byte(value))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
