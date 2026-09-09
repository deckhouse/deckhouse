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

package startup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration_WaitAfterDone(t *testing.T) {
	m := NewMigration()
	m.MarkDone()
	m.MarkDone() // second call must not panic
	require.NoError(t, m.Wait(context.Background()))
}

func TestMigration_WaitCancelled(t *testing.T) {
	m := NewMigration()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := m.Wait(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestMigration_NilReceiver(t *testing.T) {
	var m *Migration
	m.MarkDone()
	require.NoError(t, m.Wait(context.Background()))
}
