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

import "context"

const TombstoneKey = ".tombstone"

type Cache interface {
	Save(context.Context, string, []byte) error
	SaveStruct(context.Context, string, interface{}) error

	Load(context.Context, string) ([]byte, error)
	LoadStruct(context.Context, string, interface{}) error

	Delete(context.Context, string)
	Clean(ctx context.Context)
	CleanWithExceptions(ctx context.Context, excludeKeys ...string)

	GetPath(string) string
	Iterate(context.Context, func(string, []byte) error) error
	InCache(context.Context, string) (bool, error)

	NeedIntermediateSave() bool

	Dir() string
}
