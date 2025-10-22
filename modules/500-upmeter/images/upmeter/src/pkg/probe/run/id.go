/*
Copyright 2021 Flant JSC

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

package run

import (
	"math/rand"
	"os"
	"strconv"

	"github.com/spaolacci/murmur3"
)

const (
	alphaNumSymbols = "abcdefghijklmnopqrstuvwxyz01234567890123456789"
	alphaNumCount   = len(alphaNumSymbols)

	seed uint32 = 0xa5
)

func RandomIdentifier(prefix string) string {
	return prefix + "-" + ID() + "-" + randomAlphaNum(5)
}

func StaticIdentifier(prefix string) string {
	return prefix + "-" + ID()
}

var id string

func ID() string {
	if id == "" {
		id = nodeNameHash(os.Getenv("NODE_NAME"))
	}
	return id
}

func nodeNameHash(nodeName string) string {
	h32 := murmur3.New32WithSeed(seed)
	_, _ = h32.Write([]byte(nodeName))
	return strconv.FormatInt(int64(h32.Sum32()), 16)
}

func randomAlphaNum(count int) string {
	res := make([]byte, count)

	for i := 0; i < count; i++ {
		res[i] = alphaNumSymbols[rand.Int31n(int32(alphaNumCount))]
	}

	return string(res)
}
