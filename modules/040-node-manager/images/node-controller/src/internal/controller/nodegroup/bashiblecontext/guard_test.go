/*
Copyright 2026 Flant JSC

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

package bashiblecontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func staticContextEntry(name string, cidrs ...string) map[string]interface{} {
	list := make([]interface{}, 0, len(cidrs))
	for _, cidr := range cidrs {
		list = append(list, cidr)
	}
	return map[string]interface{}{
		"name":     name,
		"nodeType": "Static",
		"static":   map[string]interface{}{"internalNetworkCIDRs": list},
	}
}

func TestPublishedStaticBlock(t *testing.T) {
	cidrs := map[string]interface{}{"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"}}

	t.Run("nothing published", func(t *testing.T) {
		assert.Nil(t, publishedStaticBlock(nil))
	})
	t.Run("published entries without CIDRs", func(t *testing.T) {
		prior := map[string]map[string]interface{}{
			"a": staticContextEntry("a"),
			"b": {"name": "b", "nodeType": "CloudEphemeral"},
		}
		assert.Nil(t, publishedStaticBlock(prior))
	})
	t.Run("one entry carries them", func(t *testing.T) {
		prior := map[string]map[string]interface{}{
			"a": staticContextEntry("a"),
			"b": staticContextEntry("b", "172.18.200.0/24"),
		}
		assert.Equal(t, cidrs, publishedStaticBlock(prior))
	})
}
