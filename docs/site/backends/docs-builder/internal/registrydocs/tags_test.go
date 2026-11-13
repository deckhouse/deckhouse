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

package registrydocs

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

func TestTagsWithoutReleaseRepository(t *testing.T) {
	r := newTestRegistry(t)
	_, err := Tags(r.sources(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if !errors.Is(err, ondemand.ErrNoVersions) || !ondemand.Unpublished(err) {
		t.Fatalf("err = %v; want ondemand.ErrNoVersions", err)
	}
}

func TestTagsOfASourceThatDidNotAnswer(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:v1.0.0", release("v1.0.0")...)
	r.failing.Store(&[]string{"/v2/src2/"})

	tags, err := Tags(r.sources(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if !errors.Is(err, ondemand.ErrPartialList) || !slices.Equal(tags, []string{"v1.0.0"}) {
		t.Fatalf("tags %v, err %v; want the tags of src1 and ondemand.ErrPartialList", tags, err)
	}
}

// The tags of the store are the list, complete, although the upstream behind
// it refuses docs-builder: the upstream is read only where the store fails.
func TestTagsOfTheStore(t *testing.T) {
	r := newTestRegistry(t)
	r.push("store/echo/release:v1.0.0", release("v1.0.0")...)
	r.failing.Store(&[]string{"/v2/upstream/"})

	tags, err := Tags(r.storeThenUpstream(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if err != nil || !slices.Equal(tags, []string{"v1.0.0"}) {
		t.Fatalf("tags %v, %v; want v1.0.0 of the store, complete", tags, err)
	}
}
