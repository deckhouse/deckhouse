/*
Copyright 2021 Flant CJSC

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

const (
	alphaNumSymbols = "abcdefghijklmnopqrstuvwxyz01234567890123456789"
	alphaNumCount   = len(alphaNumSymbols)
)

func RndAlphaNum(count int) string {
	rand.Seed(time.Now().UnixNano())
	res := make([]byte, count)

	for i := 0; i < count; i++ {
		res[i] = alphaNumSymbols[rand.Int31n(int32(alphaNumCount))]
	}

	return string(res)
}
