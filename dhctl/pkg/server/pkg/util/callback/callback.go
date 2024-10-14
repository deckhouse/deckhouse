// Copyright 2024 Flant JSC
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

package callback

import "errors"

func Call(fncs ...func() error) error {
	return NewCallback(fncs...).Call()
}

type Callback struct {
	Functions []func() error
}

func NewCallback(fncs ...func() error) *Callback {
	cb := &Callback{}
	for _, f := range fncs {
		cb.Add(f)
	}
	return cb
}

func (cb *Callback) Add(f func() error) {
	if f == nil {
		return
	}
	cb.Functions = append(cb.Functions, f)
}

func (cb *Callback) Call() (err error) {
	for _, f := range cb.Functions {
		err = errors.Join(err, f())
	}
	return
}

func (cb *Callback) AsFunc() func() error {
	return cb.Call
}
