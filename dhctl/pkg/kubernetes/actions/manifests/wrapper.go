// Copyright 2023 Flant JSC
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

package manifests

type ValidatorFunc func(manifest interface{}) error

type ManifestWrapper struct {
	validators []ValidatorFunc
	data       interface{}
}

func NewManifestWrapper(data interface{}, validator ...ValidatorFunc) Manifest {
	mw := &ManifestWrapper{}
	mw.SetManifest(data)
	mw.AddValidator(validator...)
	return mw
}

func (mw *ManifestWrapper) AddValidator(validator ...ValidatorFunc) {
	mw.validators = append(mw.validators, validator...)
}

func (mw *ManifestWrapper) IsValid() error {
	var err error
	for _, v := range mw.validators {
		err = v(mw.data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mw *ManifestWrapper) GetManifest() interface{} {
	return mw.data
}

func (mw *ManifestWrapper) SetManifest(data interface{}) {
	mw.data = data
}
