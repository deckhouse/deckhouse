package encoding

import (
	"encoding/base32"
	"hash/fnv"
	"strings"
)

var letters = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func ToFnvLikeDex(input string) string {
	toDecodeString := []byte(input)
	return strings.TrimRight(letters.EncodeToString(fnv.New64().Sum(toDecodeString)), "=")
}
