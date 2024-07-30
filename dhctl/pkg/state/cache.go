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

package state

const TombstoneKey = ".tombstone"

type Cache interface {
	Save(string, []byte) error
	SaveStruct(string, interface{}) error

	Load(string) ([]byte, error)
	LoadStruct(string, interface{}) error

	Delete(string)
	Clean()
	CleanWithExceptions(excludeKeys ...string)

	GetPath(string) string
	Iterate(func(string, []byte) error) error
	InCache(string) (bool, error)

	NeedIntermediateSave() bool

	Dir() string
}
