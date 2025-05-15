// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
)

type FakeImageDescriptorProvider struct {
	expectedReference name.Reference

	ReturnedDescriptor *v1.Descriptor
	ReturnedConfigFile *v1.ConfigFile
	ReturnedError      error

	T *testing.T
}

func NewFakeImageDescriptorProvider(t *testing.T) *FakeImageDescriptorProvider {
	return &FakeImageDescriptorProvider{T: t}
}

func (f *FakeImageDescriptorProvider) ConfigFile(ref name.Reference, _ ...remote.Option) (*v1.ConfigFile, error) {
	if !assert.Equal(f.T, f.expectedReference, ref) {
		f.T.Fatalf("Expected name.Reference does not match actual: want %+v, got %+v", f.expectedReference, ref)
	}
	return f.ReturnedConfigFile, f.ReturnedError
}

func (f *FakeImageDescriptorProvider) ExpectReference(ref name.Reference) *FakeImageDescriptorProvider {
	f.expectedReference = ref
	return f
}

func (f *FakeImageDescriptorProvider) Return(conf *v1.ConfigFile, err error) *FakeImageDescriptorProvider {
	f.ReturnedConfigFile = conf
	f.ReturnedError = err
	return f
}
