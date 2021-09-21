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
