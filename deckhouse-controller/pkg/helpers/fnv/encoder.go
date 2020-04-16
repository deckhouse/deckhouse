package fnv

import (
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"strings"
)

var encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func Encode(input string) error {
	if input == "" {
		return fmt.Errorf("not enough arguments to encode")
	}
	toDecodeString := []byte(input)
	encodedString := strings.TrimRight(encoding.EncodeToString(fnv.New64().Sum(toDecodeString)), "=")
	fmt.Print(encodedString)

	return nil
}
