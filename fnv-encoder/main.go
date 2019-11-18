package main

import (
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
)

var encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func main() {
	if len(os.Args) < 2 {
		fmt.Print("Not enough arguments to encode")
		os.Exit(1)
	}
	toDecodeString := []byte(os.Args[1])
	encodedString := strings.TrimRight(encoding.EncodeToString(fnv.New64().Sum(toDecodeString)), "=")
	fmt.Print(encodedString)
}
