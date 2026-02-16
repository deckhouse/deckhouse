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

package nelm

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	nelmlog "github.com/werf/nelm/pkg/log"
)

func Test_NewNelmClient(t *testing.T) {
	logger := log.NewNop()
	cl := New(logger, WithLabels(map[string]string{
		"heritage": "deckhouse",
	}))
	assert.NotNil(t, cl)
	singleLogger := nelmlog.Default

	logger2 := log.NewNop()
	_ = New(logger2, WithLabels(map[string]string{
		"heritage": "deckhouse",
	}))
	assert.Equal(t, singleLogger, nelmlog.Default) // logger has not changed
}
