// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stringsutil

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
)

func RandomStrElement(list []string) (string, int) {
	indx := rand.Intn(len(list)) // this call is thread safe

	return list[indx], indx
}

func Index(list []string, elem string) int {
	indx := -1
	for i, v := range list {
		if v == elem {
			indx = i
			break
		}
	}

	return indx
}

func ExcludeElementFromSlice(list []string, elem string) []string {
	indx := Index(list, elem)

	if indx >= 0 {
		var res []string

		res = append(res, list[:indx]...)
		res = append(res, list[indx+1:]...)

		return res
	}

	return list
}

func Sha256Encode(input string) string {
	hasher := sha256.New()

	hasher.Write([]byte(input))

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func Sha256EncodeBytes(input []byte) string {
	hasher := sha256.New()

	hasher.Write(input)

	return fmt.Sprintf("%x", hasher.Sum(nil))
}
