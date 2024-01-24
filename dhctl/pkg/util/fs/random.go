// Copyright 2022 Flant JSC
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

package fs

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

func RandomTmpFileName() string {
	fileName := RandomNumberSuffix("dhctl-tst-touch")
	return filepath.Join(os.TempDir(), fileName)
}

// The suffix will consist of 128 bits of non-secure randomness composed in 32 hex digits, 16 bytes.
// For example, for "name-here" the call will produce something like "name-here-7d03f3e265dace994308fc812e9ab366"
func RandomNumberSuffix(name string) string {
	// Could be using k8s.io/apimachinery/pkg/util/rand.String() here instead
	return fmt.Sprintf("%s-%x%x", name, rand.Uint64(), rand.Uint64()) // these two rand calls are thread safe
}
