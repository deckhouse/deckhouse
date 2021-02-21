package util

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/spaolacci/murmur3"
)

var AgentUniqueIdentifier string

func RandomIdentifier(prefix string) string {
	return prefix + "-" + AgentUniqueId() + "-" + RndAlphaNum(5)
}

func UniqueIdentifier(prefix string) string {
	return prefix + "-" + AgentUniqueId()
}

var uniqIdSeed uint32 = 0xa5

func AgentUniqueId() string {
	if AgentUniqueIdentifier == "" {
		nodeName := os.Getenv("NODE_NAME")
		h32 := murmur3.New32WithSeed(uniqIdSeed)
		_, _ = h32.Write([]byte(nodeName))
		AgentUniqueIdentifier = strconv.FormatInt(int64(h32.Sum32()), 16)
	}
	return AgentUniqueIdentifier
}

const alphaNumSymbols = "abcdefghijklmnopqrstuvwxyz01234567890123456789"
const alphaNumCount = len(alphaNumSymbols)

func RndAlphaNum(count int) string {
	rand.Seed(time.Now().UnixNano())
	res := make([]byte, count)

	for i := 0; i < count; i++ {
		res[i] = alphaNumSymbols[rand.Int31n(int32(alphaNumCount))]
	}

	return string(res)
}
