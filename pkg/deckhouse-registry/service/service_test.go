// Copyright 2026 Flant JSC
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

package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry/client"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

func newService(t *testing.T) *service.BasicService {
	t.Helper()

	return service.NewBasicService("test", client.New("registry.example.com").WithSegment("root"), log.NewLogger())
}

func TestPathAndSub(t *testing.T) {
	svc := newService(t)

	assert.Equal(t, "registry.example.com/root", svc.Path())
	assert.Equal(t, "registry.example.com/root/a/b", svc.Sub("child", "a", "b").Path())
	assert.Equal(t, "registry.example.com/root/mod", svc.Named("child", "mod").Path())

	// Sub does not mutate its receiver.
	assert.Equal(t, "registry.example.com/root", svc.Path())
}

func TestRef(t *testing.T) {
	svc := newService(t)

	const digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	assert.Equal(t, "registry.example.com/root:v1.0.0", svc.Ref("v1.0.0"))
	assert.Equal(t, "registry.example.com/root@"+digest, svc.Ref(digest))
	assert.Equal(t, "registry.example.com/root@"+digest, svc.Ref("@"+digest))
}

// TestNamedDoesNotValidate pins the documented behaviour: a bad segment
// collapses out of the path instead of failing, which is why callers taking
// names from user input must call ValidateName first.
func TestNamedDoesNotValidate(t *testing.T) {
	svc := newService(t)

	assert.Equal(t, "registry.example.com/root", svc.Named("child", "").Path())
	assert.Error(t, service.ValidateName(""))
}

func TestValidateName(t *testing.T) {
	valid := []string{"stronghold", "trivy-db", "se-plus", "user-authn", "a1", "a.b", "a__b"}
	for _, name := range valid {
		assert.NoError(t, service.ValidateName(name), name)
	}

	invalid := []string{"", "Stronghold", "a/b", "-a", "a-", "a:b", "a b"}
	for _, name := range invalid {
		assert.Error(t, service.ValidateName(name), name)
	}

	assert.ErrorIs(t, service.ValidateName(""), service.ErrEmptyName)
	assert.ErrorIs(t, service.ValidateName("Nope"), service.ErrInvalidName)
}
